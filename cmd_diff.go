package ecschedule

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"runtime/debug"
	"sync/atomic"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchevents"
	"github.com/goccy/go-yaml"
	"golang.org/x/sync/errgroup"
)

type diffResult struct {
	ruleName         string
	diffOutput       string
	validationErrors []string
}

func executeDiffJobsInParallel(
	ctx context.Context,
	ruleNames []string,
	parallelism int,
	jobFunc func(ctx context.Context, ruleName string) (diffResult, error),
) (<-chan diffResult, <-chan error) {
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(parallelism)

	results := make(chan diffResult, len(ruleNames))
	errChan := make(chan error, 1)
	var panicCount atomic.Int32

	for _, ruleName := range ruleNames {
		ruleName := ruleName
		g.Go(func() error {
			defer func() {
				if rec := recover(); rec != nil {
					panicCount.Add(1)
					log.Printf("[ERROR] panic in worker for rule %q: %v\n%s",
						ruleName, rec, debug.Stack())
				}
			}()

			result, err := jobFunc(ctx, ruleName)
			if err != nil {
				return err
			}

			select {
			case results <- result:
			case <-ctx.Done():
				return ctx.Err()
			}

			return nil
		})
	}

	go func() {
		err := g.Wait()
		close(results)

		if err != nil {
			errChan <- err
		} else if count := panicCount.Load(); count > 0 {
			errChan <- fmt.Errorf("%d rule(s) failed due to panic (see logs above for details)", count)
		} else {
			errChan <- nil
		}
		close(errChan)
	}()

	return results, errChan
}

var cmdDiff = &runnerImpl{
	name:        "diff",
	description: "diff of the rule with remote",
	run: func(ctx context.Context, argv []string, outStream, errStream io.Writer) (err error) {
		fs := flag.NewFlagSet("ecschedule diff", flag.ContinueOnError)
		fs.SetOutput(errStream)
		var (
			conf        = fs.String("conf", "", "configuration")
			rule        = fs.String("rule", "", "rule")
			all         = fs.Bool("all", false, "diff all rules")
			unified     = fs.Bool("u", false, "output in unified diff format (colored, similar to git diff)")
			noColor     = fs.Bool("no-color", false, "disable colored output (Unified diff format only)")
			prune       = fs.Bool("prune", false, "detect orphaned rules for deletion")
			validate    = fs.Bool("validate", false, "perform validation (env, tfstate, ssm, task definition)")
			parallelism = fs.Int("parallelism", 1, "number of parallel workers (default: 1, recommended: 1-10 due to AWS API rate limits. Note: output order is not guaranteed when parallelism > 1)")
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
		if *parallelism < 1 {
			return errors.New("-parallelism must be at least 1")
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

		results, errChan := executeDiffJobsInParallel(ctx, ruleNames, *parallelism, processDiffJob)

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
