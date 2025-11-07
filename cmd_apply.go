package ecschedule

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
		fs := flag.NewFlagSet("ecschedule apply", flag.ContinueOnError)
		fs.SetOutput(errStream)
		var (
			conf    = fs.String("conf", "", "configuration")
			rule    = fs.String("rule", "", "rule")
			dryRun  = fs.Bool("dry-run", false, "dry run")
			all     = fs.Bool("all", false, "apply all rules")
			prune   = fs.Bool("prune", false, "prune orphaned rules after apply")
			unified = fs.Bool("u", false, "output diff in unified format (colored, similar to git diff)")
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

		var dryRunSuffix string
		if *dryRun {
			dryRunSuffix = " (dry-run)"
		}

		format := selectDiffFormat(*unified)

		for _, rule := range ruleNames {
			ru := c.GetRuleByName(rule)
			if ru == nil {
				return fmt.Errorf("no rules found for %s", rule)
			}
			log.Printf("applying the rule %q%s", rule, dryRunSuffix)
			if err := ru.applyInternal(ctx, a.AwsConf, *dryRun, format); err != nil {
				return err
			}
			for _, v := range ru.ContainerOverrides {
				// mask environment variables
				v.Environment = nil
			}
			bs, _ := yaml.Marshal(ru)
			log.Printf("✅ following rule applied%s\n%s", dryRunSuffix, string(bs))
		}

		if *prune {
			orphanedRules, err := extractOrphanedRules(ctx, a.AwsConf, c.BaseConfig, ruleNames)
			if err != nil {
				return err
			}

			if len(orphanedRules) > 0 {
				log.Printf("orphaned rules will be deleted %s", dryRunSuffix)
				for _, rule := range orphanedRules {
					if err := rule.deleteInternal(ctx, a.AwsConf, *dryRun, format); err != nil {
						return err
					}
				}
			}
		}

		return nil
	},
}
