package daemon

import (
	"io"
	"path"

	lock "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/camlistore/lock"
	"github.com/jbenet/go-ipfs/util"
	"github.com/jbenet/go-ipfs/util/errors"
)

// LockFile is the filename of the daemon lock, relative to config dir
const LockFile = "daemon.lock"

func Lock(confdir string) (io.Closer, error) {
	c, err := lock.Lock(path.Join(confdir, LockFile))
	return c, errors.Wrap(err)
}

func Locked(confdir string) bool {
	if !util.FileExists(path.Join(confdir, LockFile)) {
		return false
	}
	if lk, err := Lock(confdir); err != nil {
		return true

	} else {
		lk.Close()
		return false
	}
}
