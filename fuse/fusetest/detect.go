//go:build (linux || darwin || freebsd) && !nofuse

package fusetest

import (
	"os"
	"os/exec"
	"runtime"
	"testing"
)

// fuseFlagFromEnv returns the value of TEST_FUSE if set, or empty string.
// Also checks the legacy TEST_NO_FUSE for backwards compatibility.
func fuseFlagFromEnv() string {
	if v := os.Getenv("TEST_FUSE"); v != "" {
		return v
	}
	// Legacy: TEST_NO_FUSE=1 is equivalent to TEST_FUSE=0
	if os.Getenv("TEST_NO_FUSE") == "1" {
		return "0"
	}
	return ""
}

// fuseAvailable checks whether FUSE is likely to work on this system
// and skips with a helpful message if not.
//
// hanwen/go-fuse supports Linux, macOS, and FreeBSD. NetBSD and OpenBSD
// are not supported: NetBSD uses PUFFS (a different protocol) and
// OpenBSD's FUSE support is not compatible with go-fuse's mount mechanism.
func fuseAvailable(t *testing.T) bool {
	t.Helper()

	switch runtime.GOOS {
	case "linux", "darwin", "freebsd":
	default:
		t.Skip("FUSE not supported on", runtime.GOOS)
		return false
	}

	if runtime.GOOS == "linux" {
		// go-fuse tries fusermount3 first, then fusermount.
		if _, err := exec.LookPath("fusermount"); err == nil {
			return true
		}
		if _, err := exec.LookPath("fusermount3"); err == nil {
			return true
		}
		t.Skip("neither fusermount nor fusermount3 found in PATH")
		return false
	}

	if _, err := exec.LookPath("umount"); err != nil {
		t.Skip("umount not found in PATH")
	}
	return true
}
