package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Songmu/ecschedule"
)

func main() {
	log.SetFlags(0)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// After the first signal cancels ctx, unregister so a second
	// SIGINT/SIGTERM gets default (fatal) handling — the escape hatch
	// when an in-flight AWS call refuses to finish.
	go func() {
		<-ctx.Done()
		stop()
	}()
	err := ecschedule.Run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	if err != nil && err != flag.ErrHelp {
		log.Printf("💢 %s\n", err)
		exitCode := 1
		if ecoder, ok := err.(interface{ ExitCode() int }); ok {
			exitCode = ecoder.ExitCode()
		}
		os.Exit(exitCode)
	}
}
