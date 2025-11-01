package ecschedule

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchevents"
)

var cmdDiff = &runnerImpl{
	name:        "diff",
	description: "diff of the rule with remote",
	run: func(ctx context.Context, argv []string, outStream, errStream io.Writer) (err error) {
		fs := flag.NewFlagSet("ecschedule diff", flag.ContinueOnError)
		fs.SetOutput(errStream)
		var (
			conf    = fs.String("conf", "", "configuration")
			rule    = fs.String("rule", "", "rule")
			all     = fs.Bool("all", false, "diff all rules")
			unified = fs.Bool("u", false, "output in unified diff format (colored, similar to git diff)")
			noColor = fs.Bool("no-color", false, "disable colored output (Unified diff format only)")
		)
		if err := fs.Parse(argv); err != nil {
			return err
		}

		setupColor(*noColor)

		if !*all && *rule == "" {
			return errors.New("-rule or -all option required")
		}
		a := getApp(ctx)
		c := a.Config
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
			return errors.New("-conf option required")
		}

		var ruleNames []string
		if !*all {
			ruleNames = append(ruleNames, *rule)
		} else {
			for _, r := range c.Rules {
				ruleNames = append(ruleNames, r.Name)
			}
		}

		format := selectDiffFormat(*unified)

		for _, rule := range ruleNames {
			ru := c.GetRuleByName(rule)
			if ru == nil {
				return fmt.Errorf("no rules found for %s", rule)
			}
			svc := cloudwatchevents.NewFromConfig(a.AwsConf, func(o *cloudwatchevents.Options) {
				o.Region = c.Region
			})
			from, to, err := ru.diff(ctx, svc)
			if err != nil {
				return err
			}

			diffOutput := formatDiff(rule, from, to, format)

			if *unified {
				if diffOutput == "" {
					continue
				}
				fmt.Fprintln(errStream, diffOutput)
			} else {
				log.Printf("💡 diff of the rule %q\n%s", rule, diffOutput)
			}
		}
		return nil
	},
}
