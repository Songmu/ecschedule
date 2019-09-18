package ecsched

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
)

type cmdRun struct{}

func (cd *cmdRun) name() string {
	return "run"
}

func (cd *cmdRun) description() string {
	return "run the rule"
}

func (cd *cmdRun) run(ctx context.Context, argv []string, outStream, errStream io.Writer) (err error) {
	fs := flag.NewFlagSet("ecsched run", flag.ContinueOnError)
	fs.SetOutput(errStream)
	var (
		conf   = fs.String("conf", "", "configuration")
		rule   = fs.String("rule", "", "rule")
		dryRun = fs.Bool("dry-run", false, "dry run")
		// noWait = fs.Bool("no-wait", false, "exit immediately after starting the rule")
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
	log.Printf("running the rule %q%s", *rule, dryRunSuffix)
	defer func() {
		if err == nil {
			log.Printf("✅ ran the rule %q%s", ru.Name, dryRunSuffix)
		}
	}()
	if *dryRun {
		return nil
	}
	return ru.Run(ctx, a.Session, true)
}
