package ecsched

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/service/cloudwatchevents"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/ghodss/yaml"
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
		conf    = fs.String("conf", "", "configuration") // TODO --config is beter?
		write   = fs.Bool("w", false, "overwrite configuration file")
		region  = fs.String("region", "", "region")
		cluster = fs.String("cluster", "", "cluster")
		role    = fs.String("role", "", "role")
	)
	if err := fs.Parse(argv); err != nil {
		return err
	}
	c := getConfig(ctx)
	if *conf != "" {
		f, err := os.Open(*conf)
		if err != nil {
			return err
		}
		defer f.Close()
		c, err = LoadConfig(f)
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
	if *role == "" {
		*role = c.Role
	}
	if *region == "" || *cluster == "" || *role == "" {
		return fmt.Errorf("all of region, cluster and role are should be specified")
	}
	c.Region = *region
	c.Cluster = *cluster
	c.Role = *role
	sess, err := NewAWSSession(*region)
	if err != nil {
		return err
	}
	accountID, err := GetAWSAccountID(sess)
	if err != nil {
		return err
	}
	c.AccountID = accountID
	svc := cloudwatchevents.New(sess)
	ruleList, err := svc.ListRulesWithContext(ctx, &cloudwatchevents.ListRulesInput{})
	if err != nil {
		return err
	}
	var rules []*Rule
	ruleArnPrefix := fmt.Sprintf("arn:aws:events:%s:%s:rule/", *region, accountID)
	clusterArn := fmt.Sprintf("arn:aws:ecs:%s:%s:cluster/%s", *region, accountID, *cluster)
	taskDefArnPrefix := fmt.Sprintf("arn:aws:%s:%s:task-definition/", *region, accountID)
RuleList:
	for _, r := range ruleList.Rules {
		if !strings.HasPrefix(*r.Arn, ruleArnPrefix) {
			continue
		}
		ta, err := svc.ListTargetsByRuleWithContext(ctx, &cloudwatchevents.ListTargetsByRuleInput{
			Rule: r.Name,
		})
		if err != nil {
			return err
		}
		var targets []*Target
		for _, t := range ta.Targets {
			if *t.Arn != clusterArn {
				continue RuleList
			}
			targetID := *t.Id
			if targetID == *r.Name {
				targetID = ""
			}
			ecsParams := t.EcsParameters
			if ecsParams == nil {
				// ignore rule which have some non ecs targets
				continue RuleList
			}
			target := &Target{TargetID: targetID}

			taskCount := *ecsParams.TaskCount
			if taskCount != 1 {
				target.TaskCount = taskCount
			}
			taskDef := *ecsParams.TaskDefinitionArn
			if strings.HasPrefix(taskDef, taskDefArnPrefix) {
				taskDef = strings.TrimPrefix(taskDef, taskDefArnPrefix)
			}
			target.TaskDefinition = taskDef

			taskOv := &ecs.TaskOverride{}
			if err := json.Unmarshal([]byte(*t.Input), taskOv); err != nil {
				return err
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
					Name:        *r.Name,
					Command:     cmd,
					Environment: env,
				})
			}
			target.ContainerOverrides = contOverrides
			targets = append(targets, target)
		}
		ru := &Rule{
			Name:               *r.Name,
			Description:        *r.Description,
			ScheduleExpression: *r.ScheduleExpression,
			Disable:            *r.State == "DISABLED",
		}
		switch len(targets) {
		case 0:
			continue RuleList
		case 1:
			ru.Target = targets[0]
		default:
			ru.Target = targets[0]
			ru.Targets = targets[1:]
		}
		rules = append(rules, ru)
	}
	c.Rules = rules
	bs, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	fmt.Fprint(outStream, string(bs))
	_ = write
	return nil
}
