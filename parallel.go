package ecschedule

import (
	"context"
	"fmt"
	"log"
	"runtime/debug"
	"sync/atomic"

	"golang.org/x/sync/errgroup"
)

// executeJobsInParallel is the original fail-fast runner: all jobs share
// one errgroup.WithContext, so the first error cancels the context and the
// rest stop early. diff and apply -dry-run still use it.
//
// New callers should prefer executeJobsInParallelContinueOnError, the more
// general of the two: fail-fast can be layered on top of it by cancelling
// a derived context when the first error arrives, while the reverse is not
// possible. Moving the remaining callers over would change what they
// report — every rule's error instead of only the first — so that is left
// out of this change. The intent is to consolidate on that runner and drop
// this function once the behavior change is acceptable.
func executeJobsInParallel[T any](
	ctx context.Context,
	ruleNames []string,
	parallel int,
	jobFunc func(ctx context.Context, ruleName string) (T, error),
) (<-chan T, <-chan error) {
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(parallel)

	results := make(chan T, len(ruleNames))
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

// jobOutcome is the result of one named job run by
// executeJobsInParallelContinueOnError.
type jobOutcome[T any] struct {
	Index   int // position in names; summaries sort by this
	Name    string
	Result  T
	Err     error
	Skipped bool // admission saw a canceled ctx; jobFunc never ran
}

// executeJobsInParallelContinueOnError runs jobFunc for every name with
// at most `parallel` workers and returns a channel of outcomes in
// completion order.
//
// Contract:
//   - The channel is buffered to len(names); sends never block.
//   - Submission, g.Wait(), and close all run in one background goroutine;
//     the function returns the channel immediately, so callers can consume
//     outcomes while jobs are still being admitted.
//   - One job's failure never cancels sibling jobs (plain errgroup.Group,
//     errors are recorded in the outcome, never returned to the group).
//   - Every name yields exactly one outcome on every path — success,
//     error, admission skip, panic — via a single deferred send. The
//     submission loop never breaks early, so Index coverage is total.
//   - Each job checks ctx at admission; if already canceled it emits
//     Skipped without running jobFunc. Jobs are admitted in names order
//     (g.Go blocks while saturated, blocking the background goroutine).
//   - A panic in jobFunc is recovered and recorded as that job's error,
//     with the stack embedded.
func executeJobsInParallelContinueOnError[T any](
	ctx context.Context,
	names []string,
	parallel int,
	jobFunc func(ctx context.Context, name string) (T, error),
) <-chan jobOutcome[T] {
	outcomes := make(chan jobOutcome[T], len(names))
	var g errgroup.Group
	g.SetLimit(parallel)

	go func() {
		for i, name := range names {
			g.Go(func() error {
				out := jobOutcome[T]{Index: i, Name: name}
				defer func() {
					if rec := recover(); rec != nil {
						out.Err = fmt.Errorf("panic in worker for rule %q: %v\n%s",
							name, rec, debug.Stack())
					}
					outcomes <- out
				}()
				if ctx.Err() != nil {
					out.Skipped = true
					return nil
				}
				out.Result, out.Err = jobFunc(ctx, name)
				return nil
			})
		}
		_ = g.Wait()
		close(outcomes)
	}()

	return outcomes
}
