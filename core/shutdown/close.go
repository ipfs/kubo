package shutdown

import (
	"context"
	"fmt"
	"time"

	logging "github.com/ipfs/go-log/v2"
)

var closeLog = logging.Logger("shutdown")

// CloseWithCtx runs close in a goroutine and returns when it finishes or
// when ctx is done, whichever comes first. If ctx fires before close
// returns, the goroutine is leaked intentionally; the process is about to
// exit, so the leak is bounded by process lifetime. Logs at ERROR which
// subsystem failed to close in time so operators see it in journal/docker
// logs.
func CloseWithCtx(ctx context.Context, name string, close func() error) error {
	done := make(chan error, 1)
	start := time.Now()
	go func() { done <- close() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		closeLog.Errorf("subsystem %q failed to close within shutdown deadline (after %s): %s",
			name, time.Since(start), ctx.Err())
		return fmt.Errorf("%s close: %w", name, ctx.Err())
	}
}
