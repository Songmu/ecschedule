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

type applyDryRunResult struct {
	ruleName string
	ruleYaml string
}

var cmdApply = &runnerImpl{
	name:        "apply",
	description: "apply the rule",
	run: func(ctx context.Context, argv []string, outStream, errStream io.Writer) error {
		fs := flag.NewFlagSet("ecschedule apply", flag.ContinueOnError)
		fs.SetOutput(errStream)
		var (
			conf     = fs.String("conf", "", "configuration")
			rule     = fs.String("rule", "", "rule")
			dryRun   = fs.Bool("dry-run", false, "dry run")
			all      = fs.Bool("all", false, "apply all rules")
			prune    = fs.Bool("prune", false, "prune orphaned rules after apply")
			unified  = fs.Bool("u", false, "output diff in unified format (colored, similar to git diff)")
			noColor  = fs.Bool("no-color", false, "disable colored output (Unified diff format only)")
			parallel = fs.Int("parallel", 1, "number of parallel workers for dry-run (default: 1, only effective with -dry-run, recommended: 1-10 due to AWS API rate limits. Note: output order is not guaranteed when parallel > 1)")
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

		if *parallel < 1 {
			return errors.New("-parallel must be at least 1")
		}
		if *parallel > 1 && !*dryRun {
			return errors.New("-parallel can only be used with -dry-run (apply parallelization is not yet supported)")
		}

		var dryRunSuffix string
		if *dryRun {
			dryRunSuffix = " (dry-run)"
		}

		format := selectDiffFormat(*unified)

		if *dryRun {
			processApplyDryRunJob := func(ctx context.Context, ruleName string) (applyDryRunResult, error) {
				ru := c.GetRuleByName(ruleName)
				if ru == nil {
					return applyDryRunResult{}, fmt.Errorf("no rules found for %s", ruleName)
				}
				log.Printf("applying the rule %q%s", ruleName, dryRunSuffix)
				if err := ru.applyInternal(ctx, a.AwsConf, true, format); err != nil {
					return applyDryRunResult{}, err
				}
				for _, v := range ru.ContainerOverrides {
					v.Environment = nil
				}
				bs, _ := yaml.Marshal(ru)
				return applyDryRunResult{ruleName: ruleName, ruleYaml: string(bs)}, nil
			}

			results, errChan := executeJobsInParallel[applyDryRunResult](ctx, ruleNames, *parallel, processApplyDryRunJob)
			for result := range results {
				log.Printf("✅ following rule applied%s\n%s", dryRunSuffix, result.ruleYaml)
			}
			if err := <-errChan; err != nil {
				return err
			}
		} else {
			for _, rule := range ruleNames {
				ru := c.GetRuleByName(rule)
				if ru == nil {
					return fmt.Errorf("no rules found for %s", rule)
				}
				log.Printf("applying the rule %q", rule)
				if err := ru.applyInternal(ctx, a.AwsConf, false, format); err != nil {
					return err
				}
				for _, v := range ru.ContainerOverrides {
					v.Environment = nil
				}
				bs, _ := yaml.Marshal(ru)
				log.Printf("✅ following rule applied\n%s", string(bs))
			}
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
