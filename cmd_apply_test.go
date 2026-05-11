package ecschedule

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestExecuteApplyDryRunJobsInParallel(t *testing.T) {
	t.Run("parallel=1 executes jobs sequentially", func(t *testing.T) {
		var executionOrder []string
		var mu sync.Mutex

		mockJob := func(ctx context.Context, ruleName string) (applyDryRunResult, error) {
			mu.Lock()
			executionOrder = append(executionOrder, ruleName)
			mu.Unlock()

			return applyDryRunResult{
				ruleName: ruleName,
				ruleYaml: fmt.Sprintf("yaml for %s", ruleName),
			}, nil
		}

		results, errChan := executeJobsInParallel[applyDryRunResult](
			context.Background(),
			[]string{"rule1", "rule2", "rule3"},
			1,
			mockJob,
		)

		var collected []applyDryRunResult
		for result := range results {
			collected = append(collected, result)
		}

		err := <-errChan
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(collected) != 3 {
			t.Errorf("expected 3 results, got %d", len(collected))
		}

		if len(executionOrder) != 3 {
			t.Errorf("expected 3 executions, got %d", len(executionOrder))
		}
	})

	t.Run("parallel>1 executes jobs concurrently", func(t *testing.T) {
		var executedCount atomic.Int32
		jobCount := 20

		mockJob := func(ctx context.Context, ruleName string) (applyDryRunResult, error) {
			executedCount.Add(1)
			time.Sleep(10 * time.Millisecond)

			return applyDryRunResult{
				ruleName: ruleName,
				ruleYaml: fmt.Sprintf("yaml for %s", ruleName),
			}, nil
		}

		ruleNames := make([]string, jobCount)
		for i := 0; i < jobCount; i++ {
			ruleNames[i] = fmt.Sprintf("rule-%d", i)
		}

		results, errChan := executeJobsInParallel[applyDryRunResult](
			context.Background(),
			ruleNames,
			5,
			mockJob,
		)

		var collected []applyDryRunResult
		for result := range results {
			collected = append(collected, result)
		}

		err := <-errChan
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(collected) != jobCount {
			t.Errorf("expected %d results, got %d", jobCount, len(collected))
		}

		if executedCount.Load() != int32(jobCount) {
			t.Errorf("expected %d executions, got %d", jobCount, executedCount.Load())
		}
	})

	t.Run("handles job errors", func(t *testing.T) {
		errorRuleName := "rule-error"

		mockJob := func(ctx context.Context, ruleName string) (applyDryRunResult, error) {
			if ruleName == errorRuleName {
				return applyDryRunResult{}, fmt.Errorf("simulated error for %s", ruleName)
			}

			return applyDryRunResult{
				ruleName: ruleName,
				ruleYaml: fmt.Sprintf("yaml for %s", ruleName),
			}, nil
		}

		results, errChan := executeJobsInParallel[applyDryRunResult](
			context.Background(),
			[]string{"rule1", errorRuleName, "rule3"},
			2,
			mockJob,
		)

		var collected []applyDryRunResult
		for result := range results {
			collected = append(collected, result)
		}

		err := <-errChan
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if !strings.Contains(err.Error(), "simulated error") {
			t.Errorf("expected error to contain 'simulated error', got: %v", err)
		}

		_ = collected
	})

	t.Run("handles panic in jobs", func(t *testing.T) {
		panicRuleName := "rule-panic"

		mockJob := func(ctx context.Context, ruleName string) (applyDryRunResult, error) {
			if ruleName == panicRuleName {
				panic("simulated panic")
			}

			return applyDryRunResult{
				ruleName: ruleName,
				ruleYaml: fmt.Sprintf("yaml for %s", ruleName),
			}, nil
		}

		results, errChan := executeJobsInParallel[applyDryRunResult](
			context.Background(),
			[]string{"rule1", panicRuleName, "rule3"},
			2,
			mockJob,
		)

		var collected []applyDryRunResult
		for result := range results {
			collected = append(collected, result)
		}

		err := <-errChan
		if err == nil {
			t.Fatal("expected error due to panic, got nil")
		}

		if !strings.Contains(err.Error(), "failed due to panic") {
			t.Errorf("expected panic error, got: %v", err)
		}

		if len(collected) != 2 {
			t.Errorf("expected 2 results (non-panic jobs), got %d", len(collected))
		}
	})

	t.Run("context cancellation stops workers", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		var executedCount atomic.Int32

		mockJob := func(ctx context.Context, ruleName string) (applyDryRunResult, error) {
			executedCount.Add(1)

			if executedCount.Load() == 2 {
				cancel()
			}

			select {
			case <-ctx.Done():
				return applyDryRunResult{}, ctx.Err()
			case <-time.After(100 * time.Millisecond):
				return applyDryRunResult{
					ruleName: ruleName,
					ruleYaml: fmt.Sprintf("yaml for %s", ruleName),
				}, nil
			}
		}

		results, errChan := executeJobsInParallel[applyDryRunResult](
			ctx,
			[]string{"rule1", "rule2", "rule3", "rule4", "rule5"},
			2,
			mockJob,
		)

		var collected []applyDryRunResult
		for result := range results {
			collected = append(collected, result)
		}

		err := <-errChan
		if err == nil {
			t.Fatal("expected error due to context cancellation, got nil")
		}

		if len(collected) >= 5 {
			t.Errorf("expected fewer than 5 results due to cancellation, got %d", len(collected))
		}
	})

	t.Run("all rules are processed", func(t *testing.T) {
		jobCount := 10
		ruleNames := make([]string, jobCount)
		for i := 0; i < jobCount; i++ {
			ruleNames[i] = fmt.Sprintf("rule-%d", i)
		}

		mockJob := func(ctx context.Context, ruleName string) (applyDryRunResult, error) {
			return applyDryRunResult{
				ruleName: ruleName,
				ruleYaml: fmt.Sprintf("yaml for %s", ruleName),
			}, nil
		}

		results, errChan := executeJobsInParallel[applyDryRunResult](
			context.Background(),
			ruleNames,
			3,
			mockJob,
		)

		var collected []applyDryRunResult
		for result := range results {
			collected = append(collected, result)
		}

		err := <-errChan
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(collected) != jobCount {
			t.Errorf("expected %d results, got %d", jobCount, len(collected))
		}
	})
}
