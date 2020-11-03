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
	"github.com/sergi/go-diff/diffmatchpatch"
)

var cmdDiff = &runnerImpl{
	name:        "diff",
	description: "diff of the rule with remote",
	run: func(ctx context.Context, argv []string, outStream, errStream io.Writer) (err error) {
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

		sess := a.Session
		svc := cloudwatchevents.New(sess, &aws.Config{Region: &c.Region})
		from, to, err := ru.diff(ctx, svc)
		if err != nil {
			return err
		}
		dmp := diffmatchpatch.New()
		diffs := dmp.DiffMain(from, to, false)
		_, err = fmt.Fprint(outStream, dmp.DiffPrettyText(diffs))
		return err
	},
}
