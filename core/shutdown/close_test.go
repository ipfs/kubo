package shutdown

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"
)

const (
	// testFinishDeadline is the ctx deadline for the happy-path tests:
	// long enough that the close callback returns first.
	testFinishDeadline = time.Second
	// testTimeoutDeadline is the ctx deadline for the timeout test. Any
	// positive value works because the test runs under synctest's fake
	// clock; the choice only affects the exact-elapsed assertion below.
	testTimeoutDeadline = 50 * time.Millisecond
)

func TestCloseWithCtx_finishesBeforeDeadline(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), testFinishDeadline)
	defer cancel()
	if err := CloseWithCtx(ctx, "fast", func() error { return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestCloseWithCtx_propagatesCloseError(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), testFinishDeadline)
	defer cancel()
	want := errors.New("close failed")
	err := CloseWithCtx(ctx, "bad", func() error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("want %v, got %v", want, err)
	}
}

func TestCloseWithCtx_timesOut(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeoutDeadline)
		defer cancel()
		// release lets the simulated close exit after we've asserted on
		// CloseWithCtx. Without it, synctest panics with "blocked
		// goroutines remain" because production-side CloseWithCtx
		// intentionally leaks the goroutine when the deadline fires.
		release := make(chan struct{})
		start := time.Now()
		err := CloseWithCtx(ctx, "slow", func() error {
			<-release
			return nil
		})
		if elapsed := time.Since(start); elapsed != testTimeoutDeadline {
			t.Fatalf("want elapsed == %s, got %s", testTimeoutDeadline, elapsed)
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("want DeadlineExceeded, got %v", err)
		}
		close(release)
	})
}
