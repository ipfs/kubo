package testutils

import (
	"os"
	"os/exec"
	"runtime"
	"testing"
)

func RequiresDocker(t *testing.T) {
	if os.Getenv("TEST_DOCKER") != "1" {
		t.SkipNow()
	}
}

func RequiresFUSE(t *testing.T) {
	// Skip if FUSE tests are explicitly disabled
	if os.Getenv("TEST_FUSE") == "0" {
		t.Skip("FUSE tests disabled via TEST_FUSE=0")
	}

	// If TEST_FUSE=1 is set, always run (for backwards compatibility)
	if os.Getenv("TEST_FUSE") == "1" {
		return
	}

	// Auto-detect FUSE availability based on platform and tools
	if !isFUSEAvailable(t) {
		t.Skip("FUSE not available (no fusermount/umount found or unsupported platform)")
	}
}

// isFUSEAvailable checks if FUSE is available on the current system
func isFUSEAvailable(t *testing.T) bool {
	t.Helper()

	// Check platform support
	switch runtime.GOOS {
	case "linux", "darwin", "freebsd", "openbsd", "netbsd":
		// These platforms potentially support FUSE
	case "windows":
		// Windows has limited FUSE support via WinFsp, but skip for now
		return false
	default:
		// Unknown platform, assume no FUSE support
		return false
	}

	// Check for required unmount tools
	var unmountCmd string
	if runtime.GOOS == "linux" {
		unmountCmd = "fusermount"
	} else {
		unmountCmd = "umount"
	}

	_, err := exec.LookPath(unmountCmd)
	return err == nil
}

func RequiresExpensive(t *testing.T) {
	if os.Getenv("TEST_EXPENSIVE") == "1" || testing.Short() {
		t.SkipNow()
	}
}

func RequiresPlugins(t *testing.T) {
	if os.Getenv("TEST_PLUGIN") != "1" {
		t.SkipNow()
	}
}

func RequiresLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.SkipNow()
	}
}
