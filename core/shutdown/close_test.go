package shutdown

import (
	"context"
	"errors"
	"testing"
	"time"
)

const (
	// testFinishDeadline is the ctx deadline for the happy-path test: long
	// enough that the close callback returns first.
	testFinishDeadline = time.Second
	// testTimeoutDeadline is the ctx deadline for the timeout test: short
	// enough to fire before the simulated close callback returns.
	testTimeoutDeadline = 50 * time.Millisecond
	// testTimeoutSlack is the tolerance for the ctx race in the timeout
	// test: real wall time may exceed testTimeoutDeadline by up to this
	// margin without being a regression.
	testTimeoutSlack = 200 * time.Millisecond
	// testBlockingOperation is the duration the simulated close callback
	// sleeps to ensure it never returns naturally during the test.
	testBlockingOperation = time.Hour
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
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeoutDeadline)
	defer cancel()
	start := time.Now()
	err := CloseWithCtx(ctx, "slow", func() error {
		time.Sleep(testBlockingOperation)
		return nil
	})
	if elapsed := time.Since(start); elapsed > testTimeoutDeadline+testTimeoutSlack {
		t.Fatalf("did not honor deadline, took %s", elapsed)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want DeadlineExceeded, got %v", err)
	}
}
