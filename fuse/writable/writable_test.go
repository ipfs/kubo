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
