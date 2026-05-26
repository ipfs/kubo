package shutdown

import (
	"sync/atomic"
	"testing"
	"time"
)

// resetForTest clears the package-level state. Tests in this file mutate
// global state, so they cannot run in parallel.
func resetForTest(t *testing.T) {
	t.Helper()
	startedAt.Store(0)
}

func TestInProgressInitiallyFalse(t *testing.T) {
	resetForTest(t)
	if InProgress() {
		t.Fatal("InProgress() should be false before MarkStarted")
	}
	if !StartedAt().IsZero() {
		t.Fatal("StartedAt() should be zero time before MarkStarted")
	}
}

func TestMarkStartedFirstCallWins(t *testing.T) {
	resetForTest(t)
	if !MarkStarted() {
		t.Fatal("first MarkStarted() should return true")
	}
	if MarkStarted() {
		t.Fatal("second MarkStarted() should return false")
	}
	if !InProgress() {
		t.Fatal("InProgress() should be true after MarkStarted")
	}
	if StartedAt().IsZero() {
		t.Fatal("StartedAt() should be non-zero after MarkStarted")
	}
}

func TestMarkStartedPreservesFirstTimestamp(t *testing.T) {
	resetForTest(t)
	MarkStarted()
	first := StartedAt()
	// Sleep is intentional: it forces time.Now() to advance between the
	// two MarkStarted calls so a regression that replaces the CAS with a
	// plain Store would change StartedAt() and fail the assertion below.
	// Without the gap, both calls could land in the same nanosecond on
	// coarse-resolution clocks and mask the bug.
	time.Sleep(2 * time.Millisecond)
	MarkStarted() // second call must not overwrite
	if !StartedAt().Equal(first) {
		t.Fatalf("StartedAt() changed after second MarkStarted: %v != %v", StartedAt(), first)
	}
}

func TestMarkStartedConcurrent(t *testing.T) {
	resetForTest(t)
	const goroutines = 64
	var winners atomic.Int32
	done := make(chan struct{})
	for range goroutines {
		go func() {
			if MarkStarted() {
				winners.Add(1)
			}
			done <- struct{}{}
		}()
	}
	for range goroutines {
		<-done
	}
	if got := winners.Load(); got != 1 {
		t.Fatalf("expected exactly 1 winner across %d goroutines, got %d", goroutines, got)
	}
}
