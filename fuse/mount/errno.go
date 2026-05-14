// FUSE error mapping helpers. go-fuse only builds on linux, darwin, and freebsd.
//go:build (linux || darwin || freebsd) && !nofuse

package mount

import (
	"context"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
)

// ReadErrno maps an error from a context-aware read or write to a FUSE
// errno. It exists so context cancellation surfaces as EINTR rather than
// the unspecified code that fs.ToErrno produces for context.Canceled.
//
// The kernel sends FUSE_INTERRUPT when a userspace process is killed
// mid-syscall (Ctrl-C, SIGKILL on a stuck `cat`). go-fuse cancels the
// per-request context in response. Returning EINTR tells the kernel to
// abort the syscall with the right errno; without this, fs.ToErrno
// turns context.Canceled into something the caller can't act on.
func ReadErrno(err error) syscall.Errno {
	if err == context.Canceled || err == context.DeadlineExceeded {
		return syscall.EINTR
	}
	return fs.ToErrno(err)
}
