package ecsched

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/service/cloudwatchevents"
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
	sess, err := NewAWSSession(*region)
	if err != nil {
		return err
	}
	accountID, err := GetAWSAccountID(sess)
	if err != nil {
		return err
	}
	svc := cloudwatchevents.New(sess)
	ruleList, err := svc.ListRulesWithContext(ctx, &cloudwatchevents.ListRulesInput{})
	if err != nil {
		return err
	}
	var rules []*Rule
	ruleArnPrefix := fmt.Sprintf("arn:aws:events:%s:%s:rule/", *region, accountID)
	clusterArn := fmt.Sprintf("arn:aws:%s:%s:cluster/%s", *region, accountID, *cluster)
	taskDefArnPrefix := fmt.Sprintf("arn:aws:%s:%s:task-definition/", *region, accountID)
RuleList:
	for _, r := range ruleList.Rules {
		if strings.HasPrefix(*r.Arn, ruleArnPrefix) {
			ta, err := svc.ListTargetsByRuleWithContext(ctx, &cloudwatchevents.ListTargetsByRuleInput{
				Rule: r.Name,
			})
			if err != nil {
				log.Println(err)
				continue
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

				// TODO: Unmarshal ta.Input
				targets = append(targets, target)
			}
			if len(targets) > 1 {

			}
		}
		_ = rules
	}

	_ = accountID
	_ = write
	return nil
}
