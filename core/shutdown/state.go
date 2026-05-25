// Package shutdown tracks daemon-wide graceful shutdown state. The daemon
// command marks shutdown started when SIGTERM/SIGINT is received; the
// "ipfs diag healthy" subcommand checks this state for Dockerfile
// HEALTHCHECK and other monitoring.
package shutdown

import (
	"sync/atomic"
	"time"
)

// startedAt holds the unix-nano timestamp when shutdown began.
// Zero means shutdown has not started.
var startedAt atomic.Int64

// MarkStarted records that graceful shutdown has begun. Safe to call
// multiple times concurrently; only the first call wins. Returns true on
// the first call, false on subsequent calls.
func MarkStarted() bool {
	return startedAt.CompareAndSwap(0, time.Now().UnixNano())
}

// StartedAt returns when shutdown began, or the zero time if not started.
func StartedAt() time.Time {
	n := startedAt.Load()
	if n == 0 {
		return time.Time{}
	}
	return time.Unix(0, n)
}

// InProgress reports whether shutdown has been initiated.
func InProgress() bool {
	return startedAt.Load() != 0
}
