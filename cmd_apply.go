package ecsched

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/ghodss/yaml"
)

type cmdApply struct{}

func (cd *cmdApply) name() string {
	return "apply"
}

func (cd *cmdApply) description() string {
	return "apply the rule"
}

func (cd *cmdApply) run(ctx context.Context, argv []string, outStream, errStream io.Writer) (err error) {
	fs := flag.NewFlagSet("ecsched apply", flag.ContinueOnError)
	fs.SetOutput(errStream)
	var (
		conf   = fs.String("conf", "", "configuration")
		rule   = fs.String("rule", "", "rule")
		dryRun = fs.Bool("dry-run", false, "dry run")
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
	var dryRunSuffix string
	if *dryRun {
		dryRunSuffix = " (dry-run)"
	}
	log.Printf("applying rule %q%s", *rule, dryRunSuffix)
	defer func() {
		if err == nil {
			logResult(ru, dryRunSuffix)
		}
	}()
	if *dryRun {
		return nil
	}
	return ru.Apply(ctx, a.Session)
}

func logResult(ru *Rule, dryRun string) {
	for _, v := range ru.ContainerOverrides {
		// mask environment variables
		v.Environment = nil
	}
	bs, _ := yaml.Marshal(ru)
	log.Printf("following rule applied%s\n%s", dryRun, string(bs))
}
