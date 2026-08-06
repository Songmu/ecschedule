package ecschedule

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func collectAll[T any](ch <-chan jobOutcome[T]) []jobOutcome[T] {
	var outs []jobOutcome[T]
	for out := range ch {
		outs = append(outs, out)
	}
	return outs
}

func TestExecuteJobsInParallelContinueOnErrorAllSuccess(t *testing.T) {
	names := []string{"a", "b", "c", "d"}
	outs := collectAll(executeJobsInParallelContinueOnError(context.Background(), names, 2,
		func(ctx context.Context, name string) (string, error) {
			return "ok-" + name, nil
		}))
	if len(outs) != len(names) {
		t.Fatalf("outcomes = %d, want %d", len(outs), len(names))
	}
	seen := map[int]jobOutcome[string]{}
	for _, o := range outs {
		seen[o.Index] = o
	}
	for i, name := range names {
		o, ok := seen[i]
		if !ok {
			t.Fatalf("missing outcome for index %d", i)
		}
		if o.Name != name || o.Err != nil || o.Skipped || o.Result != "ok-"+name {
			t.Errorf("outcome %d = %+v", i, o)
		}
	}
}

func TestExecuteJobsInParallelContinueOnErrorPartialFailureContinues(t *testing.T) {
	names := []string{"a", "bad", "c"}
	outs := collectAll(executeJobsInParallelContinueOnError(context.Background(), names, 2,
		func(ctx context.Context, name string) (string, error) {
			if name == "bad" {
				return "", errors.New("boom")
			}
			return "ok", nil
		}))
	if len(outs) != 3 {
		t.Fatalf("all jobs must be attempted; outcomes = %d", len(outs))
	}
	var failed, succeeded int
	for _, o := range outs {
		if o.Skipped {
			t.Errorf("no job should be skipped without cancellation: %+v", o)
		}
		if o.Err != nil {
			failed++
		} else {
			succeeded++
		}
	}
	if failed != 1 || succeeded != 2 {
		t.Errorf("failed=%d succeeded=%d, want 1/2", failed, succeeded)
	}
}

func TestExecuteJobsInParallelContinueOnErrorPreCanceledCtxSkipsAll(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ran := false
	outs := collectAll(executeJobsInParallelContinueOnError(ctx, []string{"a", "b"}, 2,
		func(ctx context.Context, name string) (string, error) {
			ran = true
			return "", nil
		}))
	if ran {
		t.Error("jobFunc must not run when ctx is already canceled")
	}
	if len(outs) != 2 {
		t.Fatalf("outcomes = %d, want 2 (exactly one per name even when skipped)", len(outs))
	}
	for _, o := range outs {
		if !o.Skipped {
			t.Errorf("outcome should be Skipped: %+v", o)
		}
	}
}

func TestExecuteJobsInParallelContinueOnErrorPanicRecordedWithStack(t *testing.T) {
	outs := collectAll(executeJobsInParallelContinueOnError(context.Background(), []string{"a", "boom"}, 2,
		func(ctx context.Context, name string) (string, error) {
			if name == "boom" {
				panic("kaboom")
			}
			return "ok", nil
		}))
	if len(outs) != 2 {
		t.Fatalf("panic must still yield exactly one outcome per name; got %d", len(outs))
	}
	var panicked *jobOutcome[string]
	for i := range outs {
		if outs[i].Name == "boom" {
			panicked = &outs[i]
		}
	}
	if panicked == nil || panicked.Err == nil {
		t.Fatal("panicking job must record an error")
	}
	msg := panicked.Err.Error()
	if !strings.Contains(msg, "kaboom") {
		t.Errorf("error should contain the panic value: %s", msg)
	}
	if !strings.Contains(msg, "goroutine") {
		t.Errorf("error should embed the stack trace: %s", msg)
	}
}

func TestExecuteJobsInParallelContinueOnErrorSequentialOrder(t *testing.T) {
	var order []string
	outs := collectAll(executeJobsInParallelContinueOnError(context.Background(), []string{"a", "b", "c"}, 1,
		func(ctx context.Context, name string) (string, error) {
			order = append(order, name) // safe: parallel=1 serializes jobs
			return "", nil
		}))
	if strings.Join(order, ",") != "a,b,c" {
		t.Errorf("parallel=1 must run jobs in names order: %v", order)
	}
	if len(outs) != 3 {
		t.Errorf("outcomes = %d", len(outs))
	}
}

func TestExecuteJobsInParallelContinueOnErrorReturnsImmediately(t *testing.T) {
	// Verify that executeJobsInParallelContinueOnError returns immediately
	// without blocking even when parallel=1 and the first job blocks.
	blocker := make(chan struct{})
	defer close(blocker)

	// Call executeJobsInParallelContinueOnError; this should return
	// immediately, not block even though the first job will block waiting on
	// blocker channel.
	ch := executeJobsInParallelContinueOnError(context.Background(), []string{"block", "b", "c"}, 1,
		func(ctx context.Context, name string) (string, error) {
			if name == "block" {
				<-blocker // Block until test signals us
			}
			return "ok", nil
		})

	// Now: ch is open, but "block" hasn't delivered an outcome yet because
	// it's still waiting on blocker. Try to receive with a timeout.
	// If executeJobsInParallelContinueOnError had blocked in the caller,
	// we'd be stuck here.
	select {
	case out := <-ch:
		t.Errorf("should not have an outcome yet; got %+v", out)
	case <-time.After(10 * time.Millisecond):
		// Expected: channel open but no outcome ready yet
	}

	// Now unblock the first job and drain all outcomes
	blocker <- struct{}{}

	var outcomes []jobOutcome[string]
	for out := range ch {
		outcomes = append(outcomes, out)
	}
	if len(outcomes) != 3 {
		t.Errorf("all jobs should complete; got %d outcomes", len(outcomes))
	}
}
