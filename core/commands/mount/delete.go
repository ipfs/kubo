package fusemount

import (
	"errors"
	"fmt"
	mfs "gx/ipfs/Qmb74fRYPgpjYzoBV7PAVNmP3DQaRrh8dHdKE4PwnF3cRx/go-mfs"
	unixfs "gx/ipfs/QmcYUTQ7tBZeH1CLsZM2S3xhMEZdvUgXvbjhpMsLDpk3oJ/go-unixfs"
	gopath "path"

	"github.com/billziss-gh/cgofuse/fuse"
)

/* TODO
func (*FileSystemBase) Removexattr(path string, name string) int
*/

//TODO: review; make sure we invalidate node and handles
//TODO: lock parent?
func (fs *FUSEIPFS) Unlink(path string) int {
	fs.Lock()
	defer fs.Unlock()
	log.Debugf("Unlink - Request %q", path)

	fErr, gErr := fs.Remove(path, unixfs.TFile)
	if gErr != nil {
		log.Errorf("Unlink - %q: %s", path, gErr)
	}

	return fErr
}

func (fs *FUSEIPFS) Rmdir(path string) int {
	fs.Lock()
	defer fs.Unlock()
	log.Debugf("Rmdir - Request %q", path)

	fErr, gErr := fs.Remove(path, unixfs.TDirectory)
	if gErr != nil {
		log.Errorf("Rmdir - %q: %s", path, gErr)
	}

	return fErr
}

func (fs *FUSEIPFS) Remove(path string, nodeType FsType) (int, error) {
	//TODO: wrap parent locking and cache release somehow; unlink, mk, et al. need this too
	parentPath := gopath.Dir(path)
	parent, err := fs.LookupPath(parentPath)
	if err != nil {
		return -fuse.ENOENT, errors.New("could not fetch/lock parent path")
	}
	parent.Lock()
	defer parent.Unlock()

	fsNode, err := fs.shallowLookupPath(path)
	if err != nil {
		return -fuse.ENOENT, err
	}

	nodeStat, err := fsNode.Stat()

	//Rmdir
	if nodeType == unixfs.TDirectory {
		if err != nil {
			return -fuse.EACCES, err
		}
		if !typeCheck(nodeStat.Mode, nodeType) {
			return -fuse.ENOTDIR, errIOType
		}

		//TODO: check if empty at FS level?
	} else {
		if err != nil {
			return -fuse.EIO, err
		}
	}

	callContext, cancel := deriveCallContext(fs.ctx)
	defer cancel()

	fErr, gErr := fsNode.Remove(callContext)
	if gErr == nil {
		fs.cc.ReleasePath(parentPath)
		fs.cc.ReleasePath(path)
	}
	return fErr, gErr
}

func mfsRemove(mRoot *mfs.Root, path string) (int, error) {
	dir, name := gopath.Split(path)
	parent, err := mfs.Lookup(mRoot, dir)
	if err != nil {
		return -fuse.ENOENT, fmt.Errorf("parent lookup: %s", err)
	}
	pDir, ok := parent.(*mfs.Directory)
	if !ok {
		return -fuse.ENOTDIR, errIOType
	}
	if err = pDir.Unlink(name); err != nil {
		return -fuse.EIO, err
	}

	return fuseSuccess, nil
}
