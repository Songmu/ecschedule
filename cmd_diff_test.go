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

func TestExecuteDiffJobsInParallel(t *testing.T) {
	t.Run("parallelism=1 executes jobs sequentially", func(t *testing.T) {
		var executionOrder []string
		var mu sync.Mutex

		mockJob := func(ctx context.Context, ruleName string) (diffResult, error) {
			mu.Lock()
			executionOrder = append(executionOrder, ruleName)
			mu.Unlock()

			return diffResult{
				ruleName:   ruleName,
				diffOutput: fmt.Sprintf("diff for %s", ruleName),
			}, nil
		}

		results, errChan := executeDiffJobsInParallel(
			context.Background(),
			[]string{"rule1", "rule2", "rule3"},
			1,
			mockJob,
		)

		var collected []diffResult
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

	t.Run("parallelism>1 executes jobs concurrently", func(t *testing.T) {
		var executedCount atomic.Int32
		jobCount := 20

		mockJob := func(ctx context.Context, ruleName string) (diffResult, error) {
			executedCount.Add(1)
			time.Sleep(10 * time.Millisecond)

			return diffResult{
				ruleName:   ruleName,
				diffOutput: fmt.Sprintf("diff for %s", ruleName),
			}, nil
		}

		ruleNames := make([]string, jobCount)
		for i := 0; i < jobCount; i++ {
			ruleNames[i] = fmt.Sprintf("rule-%d", i)
		}

		results, errChan := executeDiffJobsInParallel(
			context.Background(),
			ruleNames,
			5,
			mockJob,
		)

		var collected []diffResult
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

		mockJob := func(ctx context.Context, ruleName string) (diffResult, error) {
			if ruleName == errorRuleName {
				return diffResult{}, fmt.Errorf("simulated error for %s", ruleName)
			}

			return diffResult{
				ruleName:   ruleName,
				diffOutput: fmt.Sprintf("diff for %s", ruleName),
			}, nil
		}

		results, errChan := executeDiffJobsInParallel(
			context.Background(),
			[]string{"rule1", errorRuleName, "rule3"},
			2,
			mockJob,
		)

		var collected []diffResult
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
	})

	t.Run("handles panic in jobs", func(t *testing.T) {
		panicRuleName := "rule-panic"

		mockJob := func(ctx context.Context, ruleName string) (diffResult, error) {
			if ruleName == panicRuleName {
				panic("simulated panic")
			}

			return diffResult{
				ruleName:   ruleName,
				diffOutput: fmt.Sprintf("diff for %s", ruleName),
			}, nil
		}

		results, errChan := executeDiffJobsInParallel(
			context.Background(),
			[]string{"rule1", panicRuleName, "rule3"},
			2,
			mockJob,
		)

		var collected []diffResult
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

		mockJob := func(ctx context.Context, ruleName string) (diffResult, error) {
			executedCount.Add(1)

			if executedCount.Load() == 2 {
				cancel()
			}

			select {
			case <-ctx.Done():
				return diffResult{}, ctx.Err()
			case <-time.After(100 * time.Millisecond):
				return diffResult{
					ruleName:   ruleName,
					diffOutput: fmt.Sprintf("diff for %s", ruleName),
				}, nil
			}
		}

		results, errChan := executeDiffJobsInParallel(
			ctx,
			[]string{"rule1", "rule2", "rule3", "rule4", "rule5"},
			2,
			mockJob,
		)

		var collected []diffResult
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

	t.Run("validation errors are tracked", func(t *testing.T) {
		mockJob := func(ctx context.Context, ruleName string) (diffResult, error) {
			result := diffResult{
				ruleName:   ruleName,
				diffOutput: fmt.Sprintf("diff for %s", ruleName),
			}

			if strings.HasSuffix(ruleName, "-invalid") {
				result.validationErrors = []string{"validation error 1", "validation error 2"}
			}

			return result, nil
		}

		results, errChan := executeDiffJobsInParallel(
			context.Background(),
			[]string{"rule1", "rule2-invalid", "rule3"},
			2,
			mockJob,
		)

		validationErrCount := 0
		for result := range results {
			if len(result.validationErrors) > 0 {
				validationErrCount++
			}
		}

		err := <-errChan
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if validationErrCount != 1 {
			t.Errorf("expected 1 result with validation errors, got %d", validationErrCount)
		}
	})
}
