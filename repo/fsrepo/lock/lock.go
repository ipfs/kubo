package lock

import (
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"syscall"

	lock "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/camlistore/lock"
	"github.com/ipfs/go-ipfs/util"
)

// LockFile is the filename of the repo lock, relative to config dir
// TODO rename repo lock and hide name
const LockFile = "repo.lock"

func errPerm(path string) error {
	return fmt.Errorf("failed to take lock at %s: permission denied", path)
}

func Lock(confdir string) (io.Closer, error) {
	c, err := lock.Lock(path.Join(confdir, LockFile))
	return c, err
}

func Locked(confdir string) (bool, error) {
	if !util.FileExists(path.Join(confdir, LockFile)) {
		return false, nil
	}
	if lk, err := Lock(confdir); err != nil {
		// EAGAIN == someone else has the lock
		if err == syscall.EAGAIN {
			return true, nil
		}
		if strings.Contains(err.Error(), "can't Lock file") {
			return true, nil
		}

		// lock fails on permissions error
		if os.IsPermission(err) {
			return false, errPerm(confdir)
		}
		if isLockCreatePermFail(err) {
			return false, errPerm(confdir)
		}

		// otherwise, we cant guarantee anything, error out
		return false, err
	} else {
		lk.Close()
		return false, nil
	}
}

func isLockCreatePermFail(err error) bool {
	s := err.Error()
	return strings.Contains(s, "Lock Create of") && strings.Contains(s, "permission denied")
}
