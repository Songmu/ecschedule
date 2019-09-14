package ecsched

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

type cmdApply struct{}

func (cd *cmdApply) name() string {
	return "apply"
}

func (cd *cmdApply) description() string {
	return "apply the rule"
}

func (cd *cmdApply) run(ctx context.Context, argv []string, outStream, errStream io.Writer) error {
	return nil
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
	c := getConfig(ctx)
	if *conf != "" {
		f, err := os.Open(*conf)
		if err != nil {
			return err
		}
		defer f.Close()
		c, err = LoadConfig(f)
		if err != nil {
			return err
		}
	}
	ru := c.GetRuleByName(*rule)
	if ru == nil {
		return fmt.Errorf("no rules found for %s", *rule)
	}
	return ru.Apply(ctx)
}
