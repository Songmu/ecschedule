package ecschedule

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchevents"
	cweTypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchevents/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/goccy/go-yaml"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// Rule the rule
type Rule struct {
	Name               string `yaml:"name" json:"name"`
	Description        string `yaml:"description,omitempty" json:"description,omitempty"`
	ScheduleExpression string `yaml:"scheduleExpression" json:"scheduleExpression"`
	Disabled           bool   `yaml:"disabled,omitempty" json:"disabled,omitempty"` // ENABLE | DISABLE
	*Target            `yaml:",inline" json:",inline"`
	// Targets []*Target `yaml:"targets,omitempty"`

	*BaseConfig `yaml:",inline,omitempty"`
}

// Target cluster
type Target struct {
	TargetID                 string                          `yaml:"targetId,omitempty" json:"targetId,omitempty"`
	TaskDefinition           string                          `yaml:"taskDefinition" json:"taskDefinition"`
	TaskCount                int32                           `yaml:"taskCount,omitempty" json:"taskCount,omitempty"`
	ContainerOverrides       []*ContainerOverride            `yaml:"containerOverrides,omitempty" json:"containerOverrides,omitempty"`
	Role                     string                          `yaml:"role,omitempty" json:"role,omitempty"`
	Group                    string                          `yaml:"group,omitempty" json:"group,omitempty"`
	CapacityProviderStrategy []*CapacityProviderStrategyItem `yaml:"capacityProviderStrategy,omitempty" json:"capacityProviderStrategy,omitempty"`
	LaunchType               string                          `yaml:"launch_type,omitempty" json:"launch_type,omitempty"`
	PlatformVersion          string                          `yaml:"platform_version,omitempty" json:"platform_version,omitempty"`
	NetworkConfiguration     *NetworkConfiguration           `yaml:"network_configuration,omitempty" json:"network_configuration,omitempty"`
	DeadLetterConfig         *DeadLetterConfig               `yaml:"dead_letter_config,omitempty" json:"dead_letter_config,omitempty"`
	PropagateTags            *string                         `yaml:"propagateTags,omitempty" json:"propagateTags,omitempty"`
}

// CapacityProviderStrategyItem represents ECS capacity provider strategy item
type CapacityProviderStrategyItem struct {
	CapacityProvider string `yaml:"capacityProvider" json:"capacityProvider"`
	Base             int32  `yaml:"base" json:"base"`
	Weight           int32  `yaml:"weight" json:"weight"`
}

// ContainerOverride overrides container
type ContainerOverride struct {
	Name              string            `yaml:"name" json:"name"`
	Command           []string          `yaml:"command,flow" json:"command"` // ,flow
	Environment       map[string]string `yaml:"environment,omitempty" json:"environment,omitempty"`
	Cpu               *int32            `yaml:"cpu,omitempty" json:"cpu,omitempty"`
	Memory            *int32            `yaml:"memory,omitempty" json:"memory,omitempty"`
	MemoryReservation *int32            `yaml:"memoryReservation,omitempty" json:"memoryReservation,omitempty"`
}

// A DeadLetterConfig object that contains information about a dead-letter queue
// configuration.
type DeadLetterConfig struct {
	Sqs string `yaml:"sqs" json:"sqs"`
}

func (dlc *DeadLetterConfig) sqsArn(r *Rule) string {
	ta := r.Target

	if strings.HasPrefix(ta.DeadLetterConfig.Sqs, "arn:") {
		return ta.DeadLetterConfig.Sqs
	}

	return fmt.Sprintf("arn:aws:sqs:%s:%s:%s", r.Region, r.AccountID, r.DeadLetterConfig.Sqs)
}

// NetworkConfiguration represents ECS network configuration
type NetworkConfiguration struct {
	AwsVpcConfiguration *AwsVpcConfiguration `yaml:"aws_vpc_configuration" json:"aws_vpc_configuration"`
}

// AwsVpcConfiguration represents AWS VPC configuration
type AwsVpcConfiguration struct {
	Subnets        []string `yaml:"subnets" json:"subnets"`
	SecurityGroups []string `yaml:"security_groups,omitempty" json:"security_groups,omitempty"`
	AssignPublicIP string   `yaml:"assign_public_ip,omitempty" json:"assign_public_ip,omitempty"`
}

func (nc *NetworkConfiguration) ecsParameters() *cweTypes.NetworkConfiguration {
	awsVpcConf := &cweTypes.AwsVpcConfiguration{
		Subnets: nc.AwsVpcConfiguration.Subnets,
	}
	if sgs := nc.AwsVpcConfiguration.SecurityGroups; len(sgs) > 0 {
		awsVpcConf.SecurityGroups = sgs
	}
	if as := nc.AwsVpcConfiguration.AssignPublicIP; as != "" {
		awsVpcConf.AssignPublicIp = cweTypes.AssignPublicIp(as)
	}
	return &cweTypes.NetworkConfiguration{
		AwsvpcConfiguration: awsVpcConf,
	}
}

func (nc *NetworkConfiguration) inputParameters() *ecsTypes.NetworkConfiguration {
	awsVpcConfiguration := &ecsTypes.AwsVpcConfiguration{
		Subnets: nc.AwsVpcConfiguration.Subnets,
	}
	if as := nc.AwsVpcConfiguration.AssignPublicIP; as != "" {
		awsVpcConfiguration.AssignPublicIp = ecsTypes.AssignPublicIp(as)
	}
	if sgs := nc.AwsVpcConfiguration.SecurityGroups; len(sgs) > 0 {
		awsVpcConfiguration.SecurityGroups = sgs
	}
	return &ecsTypes.NetworkConfiguration{
		AwsvpcConfiguration: awsVpcConfiguration,
	}
}

func (ta *Target) targetID(r *Rule) string {
	if r.TargetID == "" {
		return r.Name
	}
	return ta.TargetID
}

func (ta *Target) taskCount() int32 {
	if ta.TaskCount < 1 {
		return 1
	}
	return ta.TaskCount
}

// NewRuleFromRemote creates a new rule from remote (AWS EventBridge Rule)
func NewRuleFromRemote(ctx context.Context, awsConf aws.Config, bc *BaseConfig, ruleName string) (*Rule, error) {
	cw := cloudwatchevents.NewFromConfig(awsConf, func(o *cloudwatchevents.Options) {
		o.Region = bc.Region
	})
	remoteRule, err := cw.DescribeRule(ctx, &cloudwatchevents.DescribeRuleInput{
		Name: aws.String(ruleName),
	})
	if err != nil {
		return nil, err
	}

	var (
		rg = &ruleGetter{
			svc:              cw,
			ruleArnPrefix:    fmt.Sprintf("arn:aws:events:%s:%s:rule/", bc.Region, bc.AccountID),
			clusterArn:       fmt.Sprintf("arn:aws:ecs:%s:%s:cluster/%s", bc.Region, bc.AccountID, bc.Cluster),
			taskDefArnPrefix: fmt.Sprintf("arn:aws:ecs:%s:%s:task-definition/", bc.Region, bc.AccountID),
		}
	)

	cwRule := &cweTypes.Rule{
		Arn:                remoteRule.Arn,
		Description:        remoteRule.Description,
		EventBusName:       remoteRule.EventBusName,
		EventPattern:       remoteRule.EventPattern,
		ManagedBy:          remoteRule.ManagedBy,
		Name:               remoteRule.Name,
		RoleArn:            remoteRule.RoleArn,
		ScheduleExpression: remoteRule.ScheduleExpression,
		State:              remoteRule.State,
	}
	ru, err := rg.getRule(ctx, cwRule)
	if err != nil {
		return nil, err
	}

	newRule := &Rule{
		Name:               ru.Name,
		Description:        ru.Description,
		ScheduleExpression: ru.ScheduleExpression,
		Disabled:           ru.Disabled,
		Target:             ru.Target,
		BaseConfig:         bc,
	}
	return newRule, nil
}

func (r *Rule) roleARN() string {
	if strings.HasPrefix(r.Role, "arn:") {
		return r.Role
	}
	role := r.Role
	if role == "" {
		role = defaultRole
	}
	return fmt.Sprintf("arn:aws:iam::%s:role/%s", r.AccountID, role)
}

func (r *Rule) ruleARN() string {
	return fmt.Sprintf("arn:aws:events:%s:%s:rule/%s", r.Region, r.AccountID, r.Name)
}

func (ta *Target) targetARN(r *Rule) string {
	if strings.HasPrefix(r.Cluster, "arn:") {
		return r.Cluster
	}
	return fmt.Sprintf("arn:aws:ecs:%s:%s:cluster/%s", r.Region, r.AccountID, r.Cluster)
}

func (ta *Target) taskDefinitionArn(r *Rule) string {
	if strings.HasPrefix(r.TaskDefinition, "arn:") {
		return r.TaskDefinition
	}
	return fmt.Sprintf("arn:aws:ecs:%s:%s:task-definition/%s", r.Region, r.AccountID, r.TaskDefinition)
}

func (r *Rule) state() string {
	if r.Disabled {
		return "DISABLED"
	}
	return "ENABLED"
}

func (r *Rule) ecsParameters() *cweTypes.EcsParameters {
	p := cweTypes.EcsParameters{
		TaskDefinitionArn: aws.String(r.taskDefinitionArn(r)),
		TaskCount:         aws.Int32(r.taskCount()),
	}
	ta := r.Target
	if ta.Group != "" {
		p.Group = aws.String(ta.Group)
	}
	if ta.LaunchType != "" {
		p.LaunchType = cweTypes.LaunchType(ta.LaunchType)
	}

	var capacityProviderStrategy []cweTypes.CapacityProviderStrategyItem
	for _, cps := range ta.CapacityProviderStrategy {
		capacityProviderStrategy = append(capacityProviderStrategy, cweTypes.CapacityProviderStrategyItem{
			CapacityProvider: aws.String(cps.CapacityProvider),
			Base:             cps.Base,
			Weight:           cps.Weight,
		})
	}
	p.CapacityProviderStrategy = capacityProviderStrategy

	if ta.PlatformVersion != "" {
		p.PlatformVersion = aws.String(ta.PlatformVersion)
	}
	if ta.PropagateTags != nil {
		p.PropagateTags = cweTypes.PropagateTags(*ta.PropagateTags)
	}
	if nc := ta.NetworkConfiguration; nc != nil {
		p.NetworkConfiguration = nc.ecsParameters()
	}
	return &p
}

func (r *Rule) deadLetterConfigParameters() *cweTypes.DeadLetterConfig {
	ta := r.Target

	if dlc := ta.DeadLetterConfig; dlc != nil {
		arn := dlc.sqsArn(r)
		return &cweTypes.DeadLetterConfig{
			Arn: aws.String(arn),
		}
	}

	return nil
}

func (r *Rule) mergeBaseConfig(bc *BaseConfig, role string) {
	if r.Role == "" {
		// XXX care multiple target
		r.Role = role
	}
	if r.BaseConfig == nil {
		r.BaseConfig = bc
		return
	}
	if r.Region == "" {
		r.Region = bc.Region
	}
	if r.Cluster == "" {
		r.Cluster = bc.Cluster
	}
	if r.AccountID == "" {
		r.AccountID = bc.AccountID
	}
}

// PutRuleInput puts rule input
func (r *Rule) PutRuleInput() *cloudwatchevents.PutRuleInput {
	return &cloudwatchevents.PutRuleInput{
		Description:        aws.String(r.Description),
		Name:               aws.String(r.Name),
		RoleArn:            aws.String(r.roleARN()),
		ScheduleExpression: aws.String(r.ScheduleExpression),
		State:              cweTypes.RuleState(r.state()),
	}
}

// PutTargetsInput puts targets input
func (r *Rule) PutTargetsInput() *cloudwatchevents.PutTargetsInput {
	return &cloudwatchevents.PutTargetsInput{
		Rule:    aws.String(r.Name),
		Targets: []cweTypes.Target{*r.target()},
	}
}

// TagResourceInput tags resource input
func (r *Rule) TagResourceInput() *cloudwatchevents.TagResourceInput {
	return &cloudwatchevents.TagResourceInput{
		ResourceARN: aws.String(r.ruleARN()),
		Tags: []cweTypes.Tag{
			{
				Key:   aws.String("ecschedule:tracking-id"),
				Value: aws.String(r.Cluster),
			},
		},
	}
}

type containerOverridesJSON struct {
	ContainerOverrides []*containerOverrideJSON `json:"containerOverrides"`
}

type containerOverrideJSON struct {
	Name              string    `json:"name"`
	Command           []string  `json:"command,omitempty"`
	Environment       []*kvPair `json:"environment,omitempty"`
	Cpu               *int32    `json:"cpu,omitempty"`
	Memory            *int32    `json:"memory,omitempty"`
	MemoryReservation *int32    `json:"memoryReservation,omitempty"`
}

type kvPair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (r *Rule) target() *cweTypes.Target {
	if r.Target == nil {
		return nil
	}
	coj := &containerOverridesJSON{}
	for _, co := range r.ContainerOverrides {
		var kvPairs []*kvPair
		for k, v := range co.Environment {
			kvPairs = append(kvPairs, &kvPair{
				Name:  k,
				Value: v,
			})
		}
		coj.ContainerOverrides = append(coj.ContainerOverrides, &containerOverrideJSON{
			Name:              co.Name,
			Command:           co.Command,
			Environment:       kvPairs,
			Cpu:               co.Cpu,
			Memory:            co.Memory,
			MemoryReservation: co.MemoryReservation,
		})
	}
	bs, _ := json.Marshal(coj)
	return &cweTypes.Target{
		Id:               aws.String(r.targetID(r)),
		Arn:              aws.String(r.targetARN(r)),
		RoleArn:          aws.String(r.roleARN()),
		EcsParameters:    r.ecsParameters(),
		DeadLetterConfig: r.deadLetterConfigParameters(),
		Input:            aws.String(string(bs)),
	}
}

var envReg = regexp.MustCompile(`ecschedule::<([^>]+)>`)
var tfstateReg = regexp.MustCompile(`ecschedule::tfstate::<([^>]+)>`)

func (r *Rule) validateEnv() error {
	bs, err := yaml.Marshal(r)
	if err != nil {
		return err
	}
	m := envReg.FindAllSubmatch(bs, -1)
	if len(m) > 0 {
		if len(m) == 1 {
			return fmt.Errorf("environment variable %s is not defined", string(m[0][1]))
		}
		var envs []string
		for _, v := range m {
			envs = append(envs, string(v[1]))
		}
		return fmt.Errorf("environment variables %s are not defined", strings.Join(envs, " and "))
	}
	return nil
}

func (r *Rule) validateTFstate() error {
	bs, err := yaml.Marshal(r)
	if err != nil {
		return err
	}
	m := tfstateReg.FindAllSubmatch(bs, -1)
	if len(m) > 0 {
		if len(m) == 1 {
			return fmt.Errorf("tfstate reference %s is not defined", string(m[0][1]))
		}
		var envs []string
		for _, v := range m {
			envs = append(envs, string(v[1]))
		}
		return fmt.Errorf("tfstate reference %s are not defined", strings.Join(envs, " and "))
	}
	return nil
}

func (r *Rule) validateTaskDefinition(ctx context.Context, awsConf aws.Config) error {
	svc := ecs.NewFromConfig(awsConf, func(o *ecs.Options) {
		o.Region = r.Region
	})
	input := &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(r.Target.TaskDefinition),
	}
	if _, err := svc.DescribeTaskDefinition(ctx, input); err != nil {
		return fmt.Errorf("task definition %s is not defined: %s", r.Target.TaskDefinition, err.Error())
	}
	return nil
}

// Apply the rule
func (r *Rule) Apply(ctx context.Context, awsConf aws.Config, dryRun bool) error {
	if err := r.validateEnv(); err != nil {
		return err
	}
	if err := r.validateTFstate(); err != nil {
		return err
	}
	if err := r.validateTaskDefinition(ctx, awsConf); err != nil {
		return err
	}
	svc := cloudwatchevents.NewFromConfig(awsConf, func(o *cloudwatchevents.Options) {
		o.Region = r.Region
	})

	from, to, err := r.diff(ctx, svc)
	if err != nil {
		return err
	}
	if from == to {
		log.Println("💡 skip applying. no differences")
		return nil
	}

	var dryRunSuffix string
	if dryRun {
		dryRunSuffix = " (dry-run)"
	}
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(from, to, false)
	log.Printf("💡 applying following changes%s\n%s", dryRunSuffix, dmp.DiffPrettyText(diffs))
	if dryRun {
		return nil
	}
	if _, err := svc.PutRule(ctx, r.PutRuleInput()); err != nil {
		return err
	}
	if _, err = svc.PutTargets(ctx, r.PutTargetsInput()); err != nil {
		return err
	}
	_, err = svc.TagResource(ctx, r.TagResourceInput())

	return err
}

// Run the rule
func (r *Rule) Run(ctx context.Context, awsConf aws.Config, noWait bool) error {
	if err := r.validateEnv(); err != nil {
		return err
	}
	if err := r.validateTFstate(); err != nil {
		return err
	}
	svc := ecs.NewFromConfig(awsConf, func(o *ecs.Options) {
		o.Region = r.Region
	})
	var containerOverrides []ecsTypes.ContainerOverride
	for _, co := range r.ContainerOverrides {
		var (
			kvPairs []ecsTypes.KeyValuePair
			command []string
		)
		for k, v := range co.Environment {
			kvPairs = append(kvPairs, ecsTypes.KeyValuePair{
				Name:  aws.String(k),
				Value: aws.String(v),
			})
		}
		for _, v := range co.Command {
			command = append(command, v)
		}
		containerOverrides = append(containerOverrides, ecsTypes.ContainerOverride{
			Name:              aws.String(co.Name),
			Environment:       kvPairs,
			Command:           command,
			Cpu:               co.Cpu,
			Memory:            co.Memory,
			MemoryReservation: co.MemoryReservation,
		})
	}

	var networkConfiguration *ecsTypes.NetworkConfiguration
	if r.NetworkConfiguration != nil {
		networkConfiguration = r.NetworkConfiguration.inputParameters()
	}
	var propagateTags ecsTypes.PropagateTags
	if r.PropagateTags != nil {
		propagateTags = ecsTypes.PropagateTags(*r.PropagateTags)
	}

	out, err := svc.RunTask(ctx,
		&ecs.RunTaskInput{
			Cluster:        aws.String(r.Cluster),
			TaskDefinition: aws.String(r.taskDefinitionArn(r)),
			Overrides: &ecsTypes.TaskOverride{
				ContainerOverrides: containerOverrides,
			},
			Count:                aws.Int32(r.taskCount()),
			LaunchType:           ecsTypes.LaunchType(r.Target.LaunchType),
			NetworkConfiguration: networkConfiguration,
			PropagateTags:        propagateTags,
		})
	if err != nil {
		return err
	}
	if len(out.Failures) > 0 {
		f := out.Failures[0]
		return fmt.Errorf("failed to Task. Arn: %q: %s", *f.Arn, *f.Reason)
	}
	// TODO: Wait for task termination if `noWait` flag is false
	//       (Is it necessary?)
	return nil
}

// Delete the rule
func (r *Rule) Delete(ctx context.Context, awsConf aws.Config, dryRun bool) error {
	svc := cloudwatchevents.NewFromConfig(awsConf, func(o *cloudwatchevents.Options) {
		o.Region = r.Region
	})
	remoteRuleYaml, err := yaml.Marshal(r)
	if err != nil {
		return err
	}

	var dryRunSuffix string
	if dryRun {
		dryRunSuffix = " (dry-run)"
	}

	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(string(remoteRuleYaml), "", false)
	log.Printf("🪓 deleting following rule%s\n%s", dryRunSuffix, dmp.DiffPrettyText(diffs))

	if dryRun {
		return nil
	}

	// Before deleting the rule, need to delete all targets.
	if _, err := svc.RemoveTargets(ctx, &cloudwatchevents.RemoveTargetsInput{
		Ids:  []string{r.Name},
		Rule: aws.String(r.Name),
	}); err != nil {
		return err
	}
	_, err = svc.DeleteRule(ctx, &cloudwatchevents.DeleteRuleInput{
		Name: aws.String(r.Name),
	})
	return err
}

func (r *Rule) diff(ctx context.Context, cw *cloudwatchevents.Client) (from, to string, err error) {
	rule := aws.String(r.Name)

	c := r.BaseConfig
	r.BaseConfig = nil
	defer func() { r.BaseConfig = c }()
	bs, err := yaml.Marshal(r)
	if err != nil {
		return "", "", err
	}
	localRuleYaml := string(bs)

	role := r.Role
	if role == "" {
		role = defaultRole
	}
	ruleList, err := cw.ListRules(ctx, &cloudwatchevents.ListRulesInput{
		NamePrefix: rule,
	})
	if err != nil {
		return "", "", err
	}

	var (
		roleArnPrefix = fmt.Sprintf("arn:aws:iam::%s:role/", c.AccountID)
		rg            = &ruleGetter{
			svc:              cw,
			ruleArnPrefix:    fmt.Sprintf("arn:aws:events:%s:%s:rule/", c.Region, c.AccountID),
			clusterArn:       fmt.Sprintf("arn:aws:ecs:%s:%s:cluster/%s", c.Region, c.AccountID, c.Cluster),
			taskDefArnPrefix: fmt.Sprintf("arn:aws:ecs:%s:%s:task-definition/", c.Region, c.AccountID),
			roleArnPrefix:    roleArnPrefix,
			roleArn:          fmt.Sprintf("%s%s", roleArnPrefix, role),
		}
		remoteRuleYaml string
	)
	for _, r := range ruleList.Rules {
		if *r.Name != *rule {
			continue
		}
		ru, err := rg.getRule(ctx, &r)
		if err != nil {
			return "", "", err
		}
		if ru != nil {
			bs, err := yaml.Marshal(ru)
			if err != nil {
				return "", "", err
			}
			remoteRuleYaml = string(bs)
			break
		}
	}
	return remoteRuleYaml, localRuleYaml, nil
}
