package ecschedule

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
)

const cmdName = "ecschedule"

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
		conf = fs.String("conf", "", "configuration")
		ver  = fs.Bool("version", false, "display version")
	)
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
	}
	ctx = setApp(ctx, a)
	if *conf != "" {
		f, err := os.Open(*conf)
		if err != nil {
			return err
		}
		defer f.Close()
		c, err := LoadConfig(ctx, f, a.AccountID, *conf)
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
