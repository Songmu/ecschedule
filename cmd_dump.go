package ecsched

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
)

type cmdDump struct{}

func (cd *cmdDump) name() string {
	return "dump"
}

func (cd *cmdDump) description() string {
	return "dump tasks"
}

func (cd *cmdDump) run(ctx context.Context, argv []string, outStream, errStream io.Writer) error {
	fs := flag.NewFlagSet("ecsched dump", flag.ContinueOnError)
	fs.SetOutput(errStream)
	var (
		conf    = fs.String("conf", "", "configuration")
		write   = fs.Bool("w", false, "overwrite configuration file")
		region  = fs.String("region", "", "region")
		cluster = fs.String("cluster", "", "cluster")
		role    = fs.String("role", "", "role")
	)
	if err := fs.Parse(argv); err != nil {
		return err
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
	if c == nil {
		c = &Config{BaseConfig: &BaseConfig{}}
	}
	if *region == "" {
		*region = c.Region
	}
	if *cluster == "" {
		*cluster = c.Cluster
	}
	if *role == "" {
		*role = c.Role
	}
	if *region == "" || *cluster == "" || *role == "" {
		return fmt.Errorf("all of region, cluster and role are should be specified")
	}
	_ = write
	return nil
}
