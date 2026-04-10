//go:build (linux || darwin || freebsd) && !nofuse

package writable

import (
	"testing"

	"github.com/hanwen/go-fuse/v2/fuse"
)

// TestSymlinkSetattrChmodNoError verifies that Setattr on a symlink
// with only a mode change is silently accepted. POSIX symlinks have no
// meaningful permission bits (access control uses the target's mode),
// so handlers must not return an error when the kernel forwards a
// chmod-on-symlink request (e.g. via BSD lchmod or fchmodat with
// AT_SYMLINK_NOFOLLOW). Tools like rsync depend on this contract.
//
// This is a unit test rather than an integration test because Linux
// usually rejects fchmodat(AT_SYMLINK_NOFOLLOW) at the VFS layer with
// EOPNOTSUPP and never forwards it to the FUSE filesystem, so a
// userspace test would not actually exercise this code path.
func TestSymlinkSetattrChmodNoError(t *testing.T) {
	// MFSFile is nil: Setattr must still succeed without dereferencing
	// it. StoreMode is true to confirm that even when persistence is
	// enabled, mode changes on symlinks are silently dropped.
	s := &Symlink{
		Target: "/some/target",
		Cfg:    &Config{StoreMode: true},
	}

	in := &fuse.SetAttrIn{}
	in.Valid = fuse.FATTR_MODE
	in.Mode = 0o600

	out := &fuse.AttrOut{}
	if errno := s.Setattr(t.Context(), nil, in, out); errno != 0 {
		t.Fatalf("Symlink.Setattr returned errno %v, want 0", errno)
	}

	// fillAttr must report the POSIX symlink mode (0o777), not the
	// caller-supplied value, because the request is not stored.
	if got := out.Attr.Mode & 0o777; got != 0o777 {
		t.Fatalf("Symlink mode = 0o%o, want 0o777", got)
	}
}

// TestStatfsReportsSpace verifies that Dir.Statfs proxies the
// disk-space statistics of the repo's backing filesystem, and that an
// empty RepoPath produces zeroed (but successful) results.
func TestStatfsReportsSpace(t *testing.T) {
	t.Run("matches repo filesystem", func(t *testing.T) {
		dir := t.TempDir()
		d := &Dir{Cfg: &Config{RepoPath: dir}}
		out := &fuse.StatfsOut{}
		if errno := d.Statfs(t.Context(), out); errno != 0 {
			t.Fatalf("Statfs returned errno %v, want 0", errno)
		}

		// Verify we got real filesystem data (non-zero) and that
		// free blocks don't exceed total blocks. Exact comparison
		// against a second syscall.Statfs call is racy because CI
		// writes can change block counts between the two calls.
		if out.Blocks == 0 {
			t.Fatal("Blocks = 0, expected non-zero for a real filesystem")
		}
		if out.Bfree > out.Blocks {
			t.Fatalf("Bfree (%d) > Blocks (%d)", out.Bfree, out.Blocks)
		}
	})

	t.Run("empty repo path", func(t *testing.T) {
		d := &Dir{Cfg: &Config{}}
		out := &fuse.StatfsOut{}
		if errno := d.Statfs(t.Context(), out); errno != 0 {
			t.Fatalf("Statfs returned errno %v, want 0", errno)
		}
		if out.Blocks != 0 {
			t.Fatalf("expected zeroed Blocks when RepoPath is empty, got %d", out.Blocks)
		}
	})
}
