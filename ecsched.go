package ecsched

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
)

const cmdName = "ecsched"

// Run the ecsched
func Run(argv []string, outStream, errStream io.Writer) error {
	log.SetOutput(errStream)
	fs := flag.NewFlagSet(
		fmt.Sprintf("%s (v%s rev:%s)", cmdName, version, revision), flag.ContinueOnError)
	fs.SetOutput(errStream)
	var (
		conf = fs.String("conf", "", "configuration")
		ver  = fs.Bool("version", false, "display version")
	)
	if err := fs.Parse(argv); err != nil {
		return err
	}
	if *ver {
		return printVersion(outStream)
	}
	sess, err := NewAWSSession()
	if err != nil {
		return err
	}
	accountID, err := GetAWSAccountID(sess)
	if err != nil {
		return err
	}
	a := &app{
		AccountID: accountID,
		Session:   sess,
	}
	if *conf != "" {
		f, err := os.Open(*conf)
		if err != nil {
			return err
		}
		defer f.Close()
		c, err := LoadConfig(f, a.AccountID)
		if err != nil {
			return err
		}
		a.Config = c
	}
	ctx := setApp(context.Background(), a)
	argv = fs.Args()
	if len(argv) < 1 {
		return fmt.Errorf("no subcommand specified")
	}
	rnr, ok := dispatch[argv[0]]
	if !ok {
		return fmt.Errorf("unknown subcommand: %s", argv[0])
	}
	return rnr.run(ctx, argv[1:], outStream, errStream)
}

func printVersion(out io.Writer) error {
	_, err := fmt.Fprintf(out, "%s v%s (rev:%s)\n", cmdName, version, revision)
	return err
}

var (
	subCommands = []runner{
		&cmdDump{},
		&cmdApply{},
	}
	dispatch          = make(map[string]runner, len(subCommands))
	maxSubcommandName int
)

func init() {
	for _, r := range subCommands {
		n := r.name()
		l := len(n)
		if l > maxSubcommandName {
			maxSubcommandName = l
		}
		dispatch[n] = r
	}
}

func formatCommands(out io.Writer) {
	format := fmt.Sprintf("    %%-%ds  %%\n", maxSubcommandName)
	for _, r := range subCommands {
		fmt.Fprint(out, format, r.name(), r.description())
	}
}

type runner interface {
	name() string
	description() string
	run(context.Context, []string, io.Writer, io.Writer) error
}
