package ecsched

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchevents"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/goccy/go-yaml"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// Rule the rule
type Rule struct {
	Name               string `yaml:"name"`
	Description        string `yaml:"description,omitempty"`
	ScheduleExpression string `yaml:"scheduleExpression"`
	Disabled           bool   `yaml:"disabled,omitempty"` // ENABLE | DISABLE
	*Target            `yaml:",inline"`
	// Targets []*Target `yaml:"targets,omitempty"`

	*BaseConfig `yaml:",inline,omitempty"`
}

// Target cluster
type Target struct {
	TargetID           string               `yaml:"targetId,omitempty"`
	TaskDefinition     string               `yaml:"taskDefinition"`
	TaskCount          int64                `yaml:"taskCount,omitempty"`
	ContainerOverrides []*ContainerOverride `yaml:"containerOverrides,omitempty"`
	Role               string               `yaml:"role,omitempty"`
}

// ContainerOverride overrids container
type ContainerOverride struct {
	Name        string            `yaml:"name"`
	Command     []string          `yaml:"command,flow"` // ,flow
	Environment map[string]string `yaml:"environment,omitempty"`
}

func (ta *Target) targetID(r *Rule) string {
	if r.TargetID == "" {
		return r.Name
	}
	return ta.TargetID
}

func (ta *Target) taskCount() int64 {
	if ta.TaskCount < 1 {
		return 1
	}
	return ta.TaskCount
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
		State:              aws.String(r.state()),
	}
}

// PutTargetsInput puts targets input
func (r *Rule) PutTargetsInput() *cloudwatchevents.PutTargetsInput {
	return &cloudwatchevents.PutTargetsInput{
		Rule:    aws.String(r.Name),
		Targets: []*cloudwatchevents.Target{r.target()},
	}
}

type containerOverridesJSON struct {
	ContainerOverrides []*containerOverrideJSON `json:"containerOverrides"`
}

type containerOverrideJSON struct {
	Name        string    `json:"name"`
	Command     []string  `json:"command,omitempty"`
	Environment []*kvPair `json:"environment,omitempty"`
}

type kvPair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (r *Rule) target() *cloudwatchevents.Target {
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
			Name:        co.Name,
			Command:     co.Command,
			Environment: kvPairs,
		})
	}
	bs, _ := json.Marshal(coj)
	return &cloudwatchevents.Target{
		Id:      aws.String(r.targetID(r)),
		Arn:     aws.String(r.targetARN(r)),
		RoleArn: aws.String(r.roleARN()),
		EcsParameters: &cloudwatchevents.EcsParameters{
			TaskDefinitionArn: aws.String(r.taskDefinitionArn(r)),
			TaskCount:         aws.Int64(r.taskCount()),
		},
		Input: aws.String(string(bs)),
	}
}

var envReg = regexp.MustCompile(`ecsched::<([^>]+)>`)

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

// Apply the rule
func (r *Rule) Apply(ctx context.Context, sess *session.Session, dryRun bool) error {
	if err := r.validateEnv(); err != nil {
		return err
	}
	svc := cloudwatchevents.New(sess, &aws.Config{Region: aws.String(r.Region)})

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
	if _, err := svc.PutRule(r.PutRuleInput()); err != nil {
		return err
	}
	_, err = svc.PutTargets(r.PutTargetsInput())
	return err
}

// Run the rule
func (r *Rule) Run(ctx context.Context, sess *session.Session, noWait bool) error {
	if err := r.validateEnv(); err != nil {
		return err
	}
	svc := ecs.New(sess, &aws.Config{Region: aws.String(r.Region)})
	var contaierOverrides []*ecs.ContainerOverride
	for _, co := range r.ContainerOverrides {
		var (
			kvPairs []*ecs.KeyValuePair
			command []*string
		)
		for k, v := range co.Environment {
			kvPairs = append(kvPairs, &ecs.KeyValuePair{
				Name:  aws.String(k),
				Value: aws.String(v),
			})
		}
		for _, v := range co.Command {
			command = append(command, aws.String(v))
		}
		contaierOverrides = append(contaierOverrides, &ecs.ContainerOverride{
			Name:        aws.String(co.Name),
			Environment: kvPairs,
			Command:     command,
		})
	}

	out, err := svc.RunTaskWithContext(ctx,
		&ecs.RunTaskInput{
			Cluster:        aws.String(r.Cluster),
			TaskDefinition: aws.String(r.taskDefinitionArn(r)),
			Overrides: &ecs.TaskOverride{
				ContainerOverrides: contaierOverrides,
			},
			Count: aws.Int64(r.taskCount()),
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

func (r *Rule) diff(ctx context.Context, cw *cloudwatchevents.CloudWatchEvents) (from, to string, err error) {
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
	ruleList, err := cw.ListRulesWithContext(ctx, &cloudwatchevents.ListRulesInput{
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
		ru, err := rg.getRule(ctx, r)
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
