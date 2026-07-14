package ecschedule

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
)

const cmdName = "ecschedule"

// extVarFlag accumulates repeated -ext-str/-ext-code key=value (or bare key, read from env) pairs
type extVarFlag struct {
	pairs map[string]string
}

func newExtVarFlag() *extVarFlag {
	return &extVarFlag{pairs: map[string]string{}}
}

func (e *extVarFlag) String() string {
	parts := make([]string, 0, len(e.pairs))
	for k, v := range e.pairs {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, ",")
}

func (e *extVarFlag) Set(s string) error {
	if i := strings.IndexByte(s, '='); i >= 0 {
		key := s[:i]
		if key == "" {
			return fmt.Errorf("empty key in %q", s)
		}
		e.pairs[key] = s[i+1:]
		return nil
	}
	if s == "" {
		return fmt.Errorf("empty key")
	}
	v, ok := os.LookupEnv(s)
	if !ok {
		return fmt.Errorf("environment variable %q is not defined", s)
	}
	e.pairs[s] = v
	return nil
}

// Run the ecschedule
func Run(ctx context.Context, argv []string, outStream, errStream io.Writer) error {
	log.SetOutput(errStream)
	log.SetPrefix(fmt.Sprintf("[%s] ", cmdName))
	nameAndVer := fmt.Sprintf("%s (v%s rev:%s)", cmdName, version, revision)
	fs := flag.NewFlagSet(
		fmt.Sprintf("%s (v%s rev:%s)", cmdName, version, revision), flag.ContinueOnError)
	fs.SetOutput(errStream)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage of %s:\n", nameAndVer)
		fs.PrintDefaults()
		fmt.Fprintf(fs.Output(), "\nCommands:\n")
		formatCommands(fs.Output())
	}
	var (
		conf    = fs.String("conf", "", "configuration")
		ver     = fs.Bool("version", false, "display version")
		extStr  = newExtVarFlag()
		extCode = newExtVarFlag()
	)
	fs.Var(extStr, "ext-str", "jsonnet std.extVar string binding (key=value, or just key to read from env)")
	fs.Var(extCode, "ext-code", "jsonnet std.extVar code binding (key=value, or just key to read from env)")
	if err := fs.Parse(argv); err != nil {
		return err
	}
	if *ver {
		return printVersion(outStream)
	}
	awsConf, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}
	accountID, err := GetAWSAccountID(awsConf)
	if err != nil {
		return err
	}
	a := &app{
		AccountID: accountID,
		AwsConf:   awsConf,
		ExtStr:    extStr.pairs,
		ExtCode:   extCode.pairs,
	}
	ctx = setApp(ctx, a)
	if *conf != "" {
		f, err := os.Open(*conf)
		if err != nil {
			return err
		}
		defer f.Close()
		c, err := LoadConfig(ctx, f, a.AccountID, *conf, a.loadConfigOptions()...)
		if err != nil {
			return err
		}
		a.Config = c
	}
	ctx = setApp(ctx, a)
	argv = fs.Args()
	if len(argv) < 1 {
		return fmt.Errorf("no subcommand specified")
	}
	rnr, ok := cmder.dispatch[argv[0]]
	if !ok {
		return fmt.Errorf("unknown subcommand: %s", argv[0])
	}
	return rnr.Run(ctx, argv[1:], outStream, errStream)
}

func printVersion(out io.Writer) error {
	_, err := fmt.Fprintf(out, "%s v%s (rev:%s)\n", cmdName, version, revision)
	return err
}
