// Package waitall runs a set of tasks concurrently and collects their
// results, with per-task timeouts and cooperative cancellation via
// context.Context.
package waitall

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"time"
)

// ErrAborted indicates a task did not complete before its own timeout or
// before the ctx passed to WaitAll was done. Check errors.Is(err,
// context.DeadlineExceeded) or errors.Is(err, context.Canceled) to tell
// the two cases apart.
var ErrAborted = errors.New("waitall: task aborted")

type Result[T any] struct {
	Value T
	Err   error
}

// Task describes a unit of work to run concurrently via WaitAll.
//
// Fn is handed a context that is cancelled once Timeout elapses (if set)
// or once the parent context passed to WaitAll is done. Fn should
// respect ctx.Done() to stop promptly: Go cannot forcibly stop a running
// goroutine, so an Fn that ignores cancellation keeps running in the
// background even after WaitAll has reported its result as aborted. This
// includes any eventual panic: it is still recovered (so it can't crash
// the program), but since nothing is listening for it anymore, it is
// silently discarded rather than surfaced as Result.Err.
//
// If Fn panics before Timeout or ctx expire, the panic is recovered and
// returned as Result.Err (with a stack trace) instead of crashing the
// program.
type Task[T any] struct {
	Fn      func(ctx context.Context) (T, error)
	Timeout time.Duration // <= 0 disables the per-task timeout
}

// WaitAll runs tasks concurrently, waits for each to finish or time
// out, and returns results in the same order as tasks. Cancelling ctx
// aborts waiting on every task at once (e.g. for shutdown or signal
// handling); pass context.Background() if no such cancellation is needed.
func WaitAll[T any](ctx context.Context, tasks ...Task[T]) []Result[T] {
	results := make([]Result[T], len(tasks))
	var wg sync.WaitGroup

	for i, task := range tasks {
		wg.Add(1)
		go func(i int, task Task[T]) {
			defer wg.Done()
			results[i] = runTask(ctx, task)
		}(i, task)
	}

	wg.Wait()
	return results
}

func runTask[T any](parent context.Context, task Task[T]) Result[T] {
	if task.Fn == nil {
		return Result[T]{Err: errors.New("waitall: nil task function")}
	}

	if err := parent.Err(); err != nil {
		return Result[T]{Err: fmt.Errorf("%w: %w", ErrAborted, err)}
	}

	ctx := parent
	cancel := func() {}
	if task.Timeout > 0 {
		ctx, cancel = context.WithTimeout(parent, task.Timeout)
	}
	defer cancel()

	done := make(chan Result[T], 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- Result[T]{Err: fmt.Errorf("task panicked: %v\n%s", r, debug.Stack())}
			}
		}()
		v, err := task.Fn(ctx)
		done <- Result[T]{Value: v, Err: err}
	}()

	select {
	case res := <-done:
		return res
	case <-ctx.Done():
		return Result[T]{Err: fmt.Errorf("%w: %w", ErrAborted, ctx.Err())}
	}
}
