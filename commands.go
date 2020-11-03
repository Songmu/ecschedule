package ecsched

import (
	"context"
	"fmt"
	"io"
	"log"
)

type commander struct {
	cmdNames             []string
	dispatch             map[string]runner
	maxSubcommandNameLen int
}

func (co *commander) register(rnrs ...runner) {
	for _, r := range rnrs {
		n := r.Name()
		if co.dispatch == nil {
			co.dispatch = map[string]runner{}
		}
		if _, ok := co.dispatch[n]; ok {
			log.Fatalf("subcommand %q already registered", n)
		}
		co.dispatch[n] = r
		co.cmdNames = append(co.cmdNames, n)
		if co.maxSubcommandNameLen < len(n) {
			co.maxSubcommandNameLen = len(n)
		}
	}
}

var cmder = &commander{}

func init() {
	cmder.register(
		cmdApply,
		cmdDump,
		cmdRun,
		cmdDiff,
	)
}

func formatCommands(out io.Writer) {
	format := fmt.Sprintf("  %%-%ds  %%s\n", cmder.maxSubcommandNameLen)
	for _, n := range cmder.cmdNames {
		r := cmder.dispatch[n]
		fmt.Fprintf(out, format, r.Name(), r.Description())
	}
}

type runner interface {
	Name() string
	Description() string
	Run(context.Context, []string, io.Writer, io.Writer) error
}

type runnerImpl struct {
	name, description string
	run               func(context.Context, []string, io.Writer, io.Writer) error
}

func (ri *runnerImpl) Name() string {
	return ri.name
}

func (ri *runnerImpl) Description() string {
	return ri.description
}

func (ri *runnerImpl) Run(ctx context.Context, argv []string, outStream io.Writer, errStream io.Writer) error {
	return ri.run(ctx, argv, outStream, errStream)
}
