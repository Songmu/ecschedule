package ecschedule

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sync/atomic"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchevents"
	"github.com/goccy/go-yaml"
)

type diffResult struct {
	ruleName         string
	diffOutput       string
	validationErrors []string
}

var cmdDiff = &runnerImpl{
	name:        "diff",
	description: "diff of the rule with remote",
	run: func(ctx context.Context, argv []string, outStream, errStream io.Writer) (err error) {
		fs := flag.NewFlagSet("ecschedule diff", flag.ContinueOnError)
		fs.SetOutput(errStream)
		var (
			conf     = fs.String("conf", "", "configuration")
			rule     = fs.String("rule", "", "rule")
			all      = fs.Bool("all", false, "diff all rules")
			unified  = fs.Bool("u", false, "output in unified diff format (colored, similar to git diff)")
			noColor  = fs.Bool("no-color", false, "disable colored output (Unified diff format only)")
			prune    = fs.Bool("prune", false, "detect orphaned rules for deletion")
			validate = fs.Bool("validate", false, "perform validation (env, tfstate, ssm, task definition)")
			parallel = fs.Int("parallel", 1, "number of parallel workers (default: 1, recommended: 1-10 due to AWS API rate limits. Note: output order is not guaranteed when parallel > 1)")
		)
		if err := fs.Parse(argv); err != nil {
			return err
		}

		setupColor(*noColor)

		if !*all && *rule == "" {
			return errors.New("-rule or -all option required")
		}
		if *prune && !*all {
			return errors.New("-prune can only be used with -all flag")
		}
		if *parallel < 1 {
			return errors.New("-parallel must be at least 1")
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

		// Create AWS client once before starting workers
		svc := cloudwatchevents.NewFromConfig(a.AwsConf, func(o *cloudwatchevents.Options) {
			o.Region = c.Region
		})

		var hasValidationError atomic.Bool

		processDiffJob := func(ctx context.Context, ruleName string) (diffResult, error) {
			result := diffResult{ruleName: ruleName}

			ru := c.GetRuleByName(ruleName)
			if ru == nil {
				return result, fmt.Errorf("no rules found for %s", ruleName)
			}

			if *validate {
				if err := ru.validateEnv(); err != nil {
					result.validationErrors = append(result.validationErrors, fmt.Sprintf("  env: %s", err))
				}
				if err := ru.validateTFstate(); err != nil {
					result.validationErrors = append(result.validationErrors, fmt.Sprintf("  tfstate: %s", err))
				}
				if err := ru.validateSSM(); err != nil {
					result.validationErrors = append(result.validationErrors, fmt.Sprintf("  ssm: %s", err))
				}
				if err := ru.validateTaskDefinition(ctx, a.AwsConf); err != nil {
					result.validationErrors = append(result.validationErrors, fmt.Sprintf("  task definition: %s", err))
				}

				if len(result.validationErrors) > 0 {
					hasValidationError.Store(true)
				}
			}

			from, to, err := ru.diff(ctx, svc)
			if err != nil {
				return result, err
			}

			result.diffOutput = formatDiff(ruleName, from, to, format)
			return result, nil
		}

		results, errChan := executeJobsInParallel[diffResult](ctx, ruleNames, *parallel, processDiffJob)

		for result := range results {
			if len(result.validationErrors) > 0 {
				log.Printf("❌ %q: validation failed", result.ruleName)
				for _, verr := range result.validationErrors {
					log.Println(verr)
				}
			}

			if result.diffOutput != "" {
				if *unified {
					fmt.Fprintln(errStream, result.diffOutput)
				} else {
					log.Printf("💡 diff of the rule %q\n%s", result.ruleName, result.diffOutput)
				}
			}
		}

		err = <-errChan
		if err != nil {
			return err
		}

		// Display orphaned rules if -prune is specified
		if *prune {
			orphanedRules, err := extractOrphanedRules(ctx, a.AwsConf, c.BaseConfig, ruleNames)
			if err != nil {
				return err
			}

			for _, rule := range orphanedRules {
				remoteRuleYaml, err := yaml.Marshal(rule)
				if err != nil {
					return err
				}

				diffOutput := formatDiff(rule.Name, string(remoteRuleYaml), "", format)

				if *unified {
					fmt.Fprintln(errStream, diffOutput)
				} else {
					log.Printf("🪓 orphaned rule will be deleted\n%s", diffOutput)
				}
			}
		}

		if hasValidationError.Load() {
			return errors.New("validation failed for one or more rules")
		}

		return nil
	},
}
