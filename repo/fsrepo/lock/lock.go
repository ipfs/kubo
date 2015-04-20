package lock

import (
	"io"
	"path"

	lock "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/camlistore/lock"
	"github.com/ipfs/go-ipfs/util"
)

// LockFile is the filename of the repo lock, relative to config dir
// TODO rename repo lock and hide name
const LockFile = "repo.lock"

func Lock(confdir string) (io.Closer, error) {
	c, err := lock.Lock(path.Join(confdir, LockFile))
	return c, err
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
