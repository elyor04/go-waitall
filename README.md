# go-waitall

Run a set of tasks concurrently, collect their results, with per-task
timeouts and cooperative cancellation via `context.Context`.

## Install

```sh
go get github.com/elyor04/go-waitall
```

## Usage

```go
import (
	"context"
	"fmt"
	"time"

	"github.com/elyor04/go-waitall"
)

func main() {
	results := waitall.WaitAll(context.Background(),
		waitall.Task[int]{
			Fn: func(ctx context.Context) (int, error) {
				return 42, nil
			},
		},
		waitall.Task[int]{
			Timeout: time.Second,
			Fn: func(ctx context.Context) (int, error) {
				time.Sleep(2 * time.Second)
				return 0, nil
			},
		},
	)

	for _, r := range results {
		fmt.Println(r.Value, r.Err)
	}
}
```

Results are returned in the same order as the input tasks. If a task
doesn't finish before its `Timeout` (or before the `ctx` passed to
`WaitAll` is done), its `Result.Err` wraps `waitall.ErrAborted` alongside
the underlying `context.DeadlineExceeded` / `context.Canceled`, so you
can check either:

```go
errors.Is(err, waitall.ErrAborted)
errors.Is(err, context.DeadlineExceeded)
```

Note: cancellation is cooperative. Go cannot forcibly stop a running
goroutine — a `Fn` that ignores `ctx.Done()` keeps running in the
background even after `WaitAll` reports its result as aborted. This
includes an eventual panic in that abandoned call: it's recovered (so it
can't crash the program) but silently discarded, since nothing is
listening for it anymore.

If `ctx` is already done when `WaitAll` is called, tasks are not started
at all — each task's `Fn` is skipped entirely and its `Result.Err` wraps
`ErrAborted` immediately.

## License

MIT
