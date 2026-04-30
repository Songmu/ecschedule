package ecschedule

import (
	"context"
	"fmt"
	"log"
	"runtime/debug"
	"sync/atomic"

	"golang.org/x/sync/errgroup"
)

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
