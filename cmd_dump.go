package ecsched

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchevents"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/goccy/go-yaml"
)

type cmdDump struct{}

func (cd *cmdDump) name() string {
	return "dump"
}

func (cd *cmdDump) description() string {
	return "dump tasks"
}

func (cd *cmdDump) run(ctx context.Context, argv []string, outStream, errStream io.Writer) error {
	fs := flag.NewFlagSet("ecsched dump", flag.ContinueOnError)
	fs.SetOutput(errStream)
	var (
		conf    = fs.String("conf", "", "configuration")
		region  = fs.String("region", "", "region")
		cluster = fs.String("cluster", "", "cluster")
		role    = fs.String("role", "", "role")
	)
	if err := fs.Parse(argv); err != nil {
		return err
	}
	a := getApp(ctx)
	c := a.Config
	accountID := a.AccountID
	if *conf != "" {
		f, err := os.Open(*conf)
		if err != nil {
			return err
		}
		defer f.Close()
		c, err = LoadConfig(f, a.AccountID)
		if err != nil {
			return err
		}
	}
	if c == nil {
		c = &Config{BaseConfig: &BaseConfig{}}
	}
	if *region == "" {
		*region = c.Region
	}
	if *cluster == "" {
		*cluster = c.Cluster
	}
	if c.Role == "" {
		c.Role = *role
	}
	if *role == "" {
		*role = c.Role
		if *role == "" {
			*role = defaultRole
		}
	}
	if *region == "" || *cluster == "" {
		return fmt.Errorf("region and cluster are must be specified")
	}
	c.Region = *region
	c.Cluster = *cluster

	var (
		sess        = a.Session
		svc         = cloudwatchevents.New(sess, &aws.Config{Region: region})
		remoteRules []*cloudwatchevents.Rule
		nextToken   *string
	)
	for {
		r, err := svc.ListRulesWithContext(ctx, &cloudwatchevents.ListRulesInput{
			NextToken: nextToken,
		})
		if err != nil {
			return err
		}
		remoteRules = append(remoteRules, r.Rules...)
		if r.NextToken == nil {
			break
		}
		nextToken = r.NextToken
	}

	var (
		rules         []*Rule
		roleArnPrefix = fmt.Sprintf("arn:aws:iam::%s:role/", accountID)
		rg            = &ruleGetter{
			svc:              svc,
			ruleArnPrefix:    fmt.Sprintf("arn:aws:events:%s:%s:rule/", *region, accountID),
			clusterArn:       fmt.Sprintf("arn:aws:ecs:%s:%s:cluster/%s", *region, accountID, *cluster),
			taskDefArnPrefix: fmt.Sprintf("arn:aws:ecs:%s:%s:task-definition/", *region, accountID),
			roleArnPrefix:    roleArnPrefix,
			roleArn:          fmt.Sprintf("%s%s", roleArnPrefix, *role),
		}
	)
	for _, r := range remoteRules {
		ru, err := rg.getRule(ctx, r)
		if err != nil {
			return err
		}
		if ru != nil {
			rules = append(rules, ru)
		}
	}
	c.Rules = rules
	bs, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	fmt.Fprint(outStream, string(bs))
	return nil
}

type ruleGetter struct {
	svc                                                                 *cloudwatchevents.CloudWatchEvents
	clusterArn, ruleArnPrefix, taskDefArnPrefix, roleArnPrefix, roleArn string
}

func (rg *ruleGetter) getRule(ctx context.Context, r *cloudwatchevents.Rule) (*Rule, error) {
	if !strings.HasPrefix(*r.Arn, rg.ruleArnPrefix) {
		return nil, nil
	}

	ta, err := rg.svc.ListTargetsByRuleWithContext(ctx, &cloudwatchevents.ListTargetsByRuleInput{
		Rule: r.Name,
	})
	if err != nil {
		return nil, err
	}
	var targets []*Target
	for _, t := range ta.Targets {
		if *t.Arn != rg.clusterArn {
			return nil, nil
		}
		targetID := *t.Id
		if targetID == *r.Name {
			targetID = ""
		}
		ecsParams := t.EcsParameters
		if ecsParams == nil {
			// ignore rule which have some non ecs targets
			return nil, nil
		}
		target := &Target{TargetID: targetID}

		if role := *t.RoleArn; role != rg.roleArn {
			target.Role = strings.TrimPrefix(role, rg.roleArnPrefix)
		}

		taskCount := *ecsParams.TaskCount
		if taskCount != 1 {
			target.TaskCount = taskCount
		}
		target.TaskDefinition = strings.TrimPrefix(*ecsParams.TaskDefinitionArn, rg.taskDefArnPrefix)

		taskOv := &ecs.TaskOverride{}
		if err := json.Unmarshal([]byte(*t.Input), taskOv); err != nil {
			return nil, err
		}
		var contOverrides []*ContainerOverride
		for _, co := range taskOv.ContainerOverrides {
			var cmd []string
			for _, c := range co.Command {
				cmd = append(cmd, *c)
			}
			env := map[string]string{}
			for _, kv := range co.Environment {
				env[*kv.Name] = *kv.Value
			}
			contOverrides = append(contOverrides, &ContainerOverride{
				Name:        *co.Name,
				Command:     cmd,
				Environment: env,
			})
		}
		target.ContainerOverrides = contOverrides
		targets = append(targets, target)
	}
	var desc string
	if r.Description != nil {
		desc = *r.Description
	}
	ru := &Rule{
		Name:               *r.Name,
		Description:        desc,
		ScheduleExpression: *r.ScheduleExpression,
		Disabled:           *r.State == "DISABLED",
	}
	switch len(targets) {
	case 0:
		return nil, nil
	case 1:
		ru.Target = targets[0]
	default:
		// not supported multiple target yet
		return nil, nil
		// ru.Target = targets[0]
		// ru.Targets = targets[1:]
	}
	return ru, nil
}
