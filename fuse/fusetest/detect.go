//go:build !nofuse

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
// On Linux, bazil.org/fuse requires "fusermount" (not "fusermount3") in
// PATH. Systems with only fuse3 installed need a symlink:
//
//	sudo ln -s /usr/bin/fusermount3 /usr/local/bin/fusermount
func fuseAvailable(t *testing.T) bool {
	t.Helper()

	switch runtime.GOOS {
	case "linux", "darwin", "freebsd", "netbsd", "openbsd":
	default:
		t.Skip("FUSE not supported on", runtime.GOOS)
		return false
	}

	if runtime.GOOS == "linux" {
		if _, err := exec.LookPath("fusermount"); err == nil {
			return true
		}
		if _, err := exec.LookPath("fusermount3"); err == nil {
			t.Skip("fusermount3 found but bazil.org/fuse needs \"fusermount\"; create a symlink: sudo ln -s /usr/bin/fusermount3 /usr/local/bin/fusermount")
		}
		t.Skip("fusermount not found in PATH")
		return false
	}

	if _, err := exec.LookPath("umount"); err != nil {
		t.Skip("umount not found in PATH")
	}
	return true
}
