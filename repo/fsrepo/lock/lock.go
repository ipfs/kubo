package lock

import (
	"io"
	"os"
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

func Locked(confdir string) (bool, error) {
	if !util.FileExists(path.Join(confdir, LockFile)) {
		return false, nil
	}
	if lk, err := Lock(confdir); err != nil {
		if os.IsPermission(err) {
			return false, err
		}
		return true, nil
	} else {
		lk.Close()
		return false, nil
	}
}
