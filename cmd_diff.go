package ecsched

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchevents"
	"github.com/goccy/go-yaml"
	"github.com/sergi/go-diff/diffmatchpatch"
)

type cmdDiff struct{}

func (cd *cmdDiff) name() string {
	return "diff"
}

func (cd *cmdDiff) description() string {
	return "diff of the rule with remote"
}

func (cd *cmdDiff) run(ctx context.Context, argv []string, outStream, errStream io.Writer) (err error) {
	fs := flag.NewFlagSet("ecsched apply", flag.ContinueOnError)
	fs.SetOutput(errStream)
	var (
		conf = fs.String("conf", "", "configuration")
		rule = fs.String("rule", "", "rule")
		// all  = fs.Bool("all", false, "apply all rules")
	)
	if err := fs.Parse(argv); err != nil {
		return err
	}
	if *rule == "" {
		return errors.New("-rule option required")
	}
	a := getApp(ctx)
	c := a.Config
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
	ru := c.GetRuleByName(*rule)
	if ru == nil {
		return fmt.Errorf("no rules found for %s", *rule)
	}
	origBc := ru.BaseConfig
	ru.BaseConfig = nil
	defer func() { ru.BaseConfig = origBc }()
	bs, err := yaml.Marshal(ru)
	if err != nil {
		return err
	}
	localRuleYaml := string(bs)

	role := c.Role
	if role == "" {
		role = defaultRole
	}
	accountID := a.AccountID
	sess := a.Session
	svc := cloudwatchevents.New(sess, &aws.Config{Region: &c.Region})
	ruleList, err := svc.ListRulesWithContext(ctx, &cloudwatchevents.ListRulesInput{
		NamePrefix: rule,
	})

	var (
		roleArnPrefix = fmt.Sprintf("arn:aws:iam::%s:role/", accountID)
		rg            = &ruleGetter{
			svc:              svc,
			ruleArnPrefix:    fmt.Sprintf("arn:aws:events:%s:%s:rule/", c.Region, accountID),
			clusterArn:       fmt.Sprintf("arn:aws:ecs:%s:%s:cluster/%s", c.Region, accountID, c.Cluster),
			taskDefArnPrefix: fmt.Sprintf("arn:aws:ecs:%s:%s:task-definition/", c.Region, accountID),
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
			return err
		}
		if ru != nil {
			bs, err := yaml.Marshal(ru)
			if err != nil {
				return err
			}
			remoteRuleYaml = string(bs)
			break
		}
	}
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(remoteRuleYaml, localRuleYaml, false)
	_, err = fmt.Fprint(outStream, dmp.DiffPrettyText(diffs))
	return err
}
