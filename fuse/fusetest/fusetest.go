//go:build (linux || darwin || freebsd) && !nofuse

// Package fusetest provides test helpers shared across FUSE test packages.
package fusetest

import (
	"os"
	"testing"

	"github.com/hanwen/go-fuse/v2/fs"
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

// TestMount mounts root at a temp directory with the given options and
// registers an unmount cleanup. Returns the mount directory path.
// Callers set mount-specific options (timeouts, MaxReadAhead, etc.)
// before calling; this helper adds NullPermissions, UID, and GID.
func TestMount(t *testing.T, root fs.InodeEmbedder, opts *fs.Options) string {
	t.Helper()
	SkipUnlessFUSE(t)
	mntDir := t.TempDir()
	if opts == nil {
		opts = &fs.Options{}
	}
	opts.NullPermissions = true
	opts.UID = uint32(os.Getuid())
	opts.GID = uint32(os.Getgid())
	if opts.MountOptions.FsName == "" {
		opts.MountOptions.FsName = "kubo-test"
	}
	server, err := fs.Mount(mntDir, root, opts)
	MountError(t, err)
	t.Cleanup(func() { _ = server.Unmount() })
	return mntDir
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
