package daemon

import (
	"io"
	"path"

	lock "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/camlistore/lock"
)

// LockFile is the filename of the daemon lock, relative to config dir
const LockFile = "daemon.lock"

func Lock(confdir string) (io.Closer, error) {
	return lock.Lock(path.Join(confdir, LockFile))
}

func Locked(confdir string) bool {
	if lk, err := Lock(confdir); err != nil {
		return true

	} else {
		lk.Close()
		return false
	}
}
