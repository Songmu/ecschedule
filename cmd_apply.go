package ecsched

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/goccy/go-yaml"
)

var cmdApply = &runnerImpl{
	name:        "apply",
	description: "apply the rule",
	run: func(ctx context.Context, argv []string, outStream, errStream io.Writer) error {
		fs := flag.NewFlagSet("ecsched apply", flag.ContinueOnError)
		fs.SetOutput(errStream)
		var (
			conf   = fs.String("conf", "", "configuration")
			rule   = fs.String("rule", "", "rule")
			dryRun = fs.Bool("dry-run", false, "dry run")
			all    = fs.Bool("all", false, "apply all rules")
		)
		if err := fs.Parse(argv); err != nil {
			return err
		}
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
			c, err = LoadConfig(f, a.AccountID)
			if err != nil {
				return err
			}
		}

		var ruleNames []string
		if !*all {
			ruleNames = append(ruleNames, *rule)
		} else {
			for _, r := range c.Rules {
				ruleNames = append(ruleNames, r.Name)
			}
		}
		for _, rule := range ruleNames {
			ru := c.GetRuleByName(rule)
			if ru == nil {
				return fmt.Errorf("no rules found for %s", rule)
			}
			var dryRunSuffix string
			if *dryRun {
				dryRunSuffix = " (dry-run)"
			}
			log.Printf("applying the rule %q%s", rule, dryRunSuffix)
			if err := ru.Apply(ctx, a.Session, *dryRun); err != nil {
				return err
			}
			for _, v := range ru.ContainerOverrides {
				// mask environment variables
				v.Environment = nil
			}
			bs, _ := yaml.Marshal(ru)
			log.Printf("✅ following rule applied%s\n%s", dryRunSuffix, string(bs))
		}
		return nil
	},
}
