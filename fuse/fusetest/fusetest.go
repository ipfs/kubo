//go:build !nofuse

// Package fusetest provides test helpers shared across FUSE test packages.
package fusetest

import (
	"testing"
)

// SkipUnlessFUSE skips the test when FUSE is not available.
//
// Decision order:
//  1. TEST_FUSE=0 (or legacy TEST_NO_FUSE=1) → skip
//  2. TEST_FUSE=1 → run (CI should set this after installing fuse3)
//  3. Neither set → auto-detect based on platform and fusermount in PATH;
//     skip with a helpful message if not found
func SkipUnlessFUSE(t *testing.T) {
	t.Helper()

	if v := fuseFlagFromEnv(); v != "" {
		if v == "0" {
			t.Skip("FUSE tests disabled (TEST_FUSE=0)")
		}
		return // TEST_FUSE=1, run unconditionally
	}

	fuseAvailable(t) // skips with a helpful message if not available
}

// MountError handles a FUSE mount error. When TEST_FUSE=1 (CI), a mount
// failure is fatal because the environment is expected to have working FUSE.
// When auto-detecting (no TEST_FUSE set), mount failures cause a skip.
func MountError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	if fuseFlagFromEnv() == "1" {
		t.Fatal("FUSE mount failed (TEST_FUSE=1, expected FUSE to work):", err)
	}
	t.Skip("FUSE mount failed:", err)
}
