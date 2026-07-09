package waitall

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestWaitAll_SuccessPreservesOrder(t *testing.T) {
	tasks := []Task[int]{
		{Fn: func(ctx context.Context) (int, error) { return 1, nil }},
		{Fn: func(ctx context.Context) (int, error) { return 2, nil }},
		{Fn: func(ctx context.Context) (int, error) { return 3, nil }},
	}

	results := WaitAll(context.Background(), tasks...)

	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
	for i, want := range []int{1, 2, 3} {
		if results[i].Err != nil {
			t.Errorf("result[%d]: unexpected error: %v", i, results[i].Err)
		}
		if results[i].Value != want {
			t.Errorf("result[%d].Value = %d, want %d", i, results[i].Value, want)
		}
	}
}

func TestWaitAll_TaskError(t *testing.T) {
	wantErr := errors.New("boom")
	results := WaitAll(context.Background(), Task[int]{
		Fn: func(ctx context.Context) (int, error) { return 0, wantErr },
	})

	if !errors.Is(results[0].Err, wantErr) {
		t.Fatalf("Err = %v, want %v", results[0].Err, wantErr)
	}
}

func TestWaitAll_RunsConcurrently(t *testing.T) {
	const n = 5
	const delay = 100 * time.Millisecond

	tasks := make([]Task[int], n)
	for i := range tasks {
		tasks[i] = Task[int]{Fn: func(ctx context.Context) (int, error) {
			time.Sleep(delay)
			return 0, nil
		}}
	}

	start := time.Now()
	WaitAll(context.Background(), tasks...)
	elapsed := time.Since(start)

	if elapsed >= n*delay {
		t.Fatalf("elapsed %v looks sequential, want well under %v", elapsed, n*delay)
	}
}

func TestWaitAll_PerTaskTimeout(t *testing.T) {
	results := WaitAll(context.Background(), Task[int]{
		Timeout: 20 * time.Millisecond,
		Fn: func(ctx context.Context) (int, error) {
			<-ctx.Done()
			return 0, ctx.Err()
		},
	})

	err := results[0].Err
	if !errors.Is(err, ErrAborted) {
		t.Errorf("Err = %v, want errors.Is match for ErrAborted", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Err = %v, want errors.Is match for context.DeadlineExceeded", err)
	}
}

func TestWaitAll_ParentCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	results := WaitAll(ctx, Task[int]{
		Fn: func(ctx context.Context) (int, error) {
			<-ctx.Done()
			return 0, ctx.Err()
		},
	})

	err := results[0].Err
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Err = %v, want errors.Is match for context.Canceled", err)
	}
}

func TestWaitAll_PanicIsRecovered(t *testing.T) {
	results := WaitAll(context.Background(), Task[int]{
		Fn: func(ctx context.Context) (int, error) { panic("kaboom") },
	})

	err := results[0].Err
	if err == nil || !strings.Contains(err.Error(), "kaboom") {
		t.Fatalf("Err = %v, want an error mentioning the panic value", err)
	}
}

func TestWaitAll_NilFn(t *testing.T) {
	results := WaitAll(context.Background(), Task[int]{})

	if results[0].Err == nil {
		t.Fatal("expected an error for nil Fn, got nil")
	}
}
