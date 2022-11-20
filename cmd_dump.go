package ecschedule

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchevents"
	"github.com/goccy/go-yaml"
)

var cmdDump = &runnerImpl{
	name:        "dump",
	description: "dump tasks",
	run: func(ctx context.Context, argv []string, outStream, errStream io.Writer) error {
		fs := flag.NewFlagSet("ecschedule dump", flag.ContinueOnError)
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
			c, err = LoadConfig(ctx, f, a.AccountID, *conf)
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
				sqsArnPrefix:     fmt.Sprintf("arn:aws:sqs:%s:%s:", *region, accountID),
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
	},
}
