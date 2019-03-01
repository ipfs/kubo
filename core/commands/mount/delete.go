package fusemount

import (
	"fmt"
	mfs "gx/ipfs/Qmb74fRYPgpjYzoBV7PAVNmP3DQaRrh8dHdKE4PwnF3cRx/go-mfs"
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
	/*
		lNode, err := fs.parseLocalPath(path)
		if err != nil {
			log.Errorf("Unlink - path err %s", err)
			return -fuse.EINVAL
		}
	*/

	fsNode, err := fs.LookupPath(path)
	if err != nil {
		log.Errorf("Unlink - lookup error: %s", err)
		return -fuse.ENOENT
	}
	fsNode.Lock()
	defer fsNode.Unlock()

	switch fsNode.(type) {
	default:
		log.Errorf("Unlink - request in read only section: %q", path)
		return -fuse.EROFS
	case *ipfsRoot, *ipfsNode:
		//TODO: block rm?
		log.Errorf("Unlink - request in read only section: %q", path)
		return -fuse.EROFS
	case *mfsNode:
		if err := mfsRemove(fs.filesRoot, path[frs:]); err != nil {
			log.Errorf("Unlink - mfs error: %s", err)
			return -fuse.EIO
		}

		//invalidate cache object
		fs.cc.ReleasePath(path)
	case *ipnsKey:
		_, keyName := gopath.Split(path)
		_, err := fs.core.Key().Remove(fs.ctx, keyName)
		if err != nil {
			log.Errorf("could not remove IPNS key %q: %s", keyName, err)
			return -fuse.EIO
		}

	case *ipnsNode:
		keyRoot, subPath, err := fs.ipnsMFSSplit(path)
		if err != nil {
			log.Errorf("Unlink - IPNS key error: %s", err)
		}
		if err := mfsRemove(keyRoot, subPath); err != nil {
			log.Errorf("Unlink - mfs error: %s", err)
			return -fuse.EIO
		}
	}

	return fuseSuccess
}

//TODO: lock parent
func (fs *FUSEIPFS) Rmdir(path string) int {
	fs.Lock()
	defer fs.Unlock()
	log.Debugf("Rmdir - Request %q", path)

	lNode, err := parseLocalPath(path)
	if err != nil {
		log.Errorf("Rmdir - path err %s", err)
		return -fuse.ENOENT
	}

	parentPath := gopath.Dir(path)
	parent, err := fs.LookupPath(parentPath)
	if err != nil {
		log.Errorf("Mkdir - could not fetch/lock parent for %q", path)
	}
	parent.Lock()
	defer parent.Unlock()
	defer fs.cc.ReleasePath(parentPath) //TODO: don't do this on failure

	defer fs.cc.ReleasePath(path) //TODO: don't do on failure

	switch lNode.(type) {
	case *ipnsKey:
		_, keyName := gopath.Split(path)
		_, err := fs.core.Key().Remove(fs.ctx, keyName)
		if err != nil {
			log.Errorf("could not remove IPNS key %q: %s", keyName, err)
			return -fuse.EIO
		}

	case *ipnsNode:
	case *ipfsNode:
		return -fuse.EROFS
	case *mfsNode:
		if err := mfsRemove(fs.filesRoot, path[frs:]); err != nil {
			log.Errorf("Unlink - DBG EIO %q %s", path, err)
			return -fuse.EIO
		}
	}
	return fuseSuccess
}

func mfsRemove(mRoot *mfs.Root, path string) error {
	dir, name := gopath.Split(path)
	parent, err := mfs.Lookup(mRoot, dir)
	if err != nil {
		return fmt.Errorf("parent lookup: %s", err)
	}
	pDir, ok := parent.(*mfs.Directory)
	if !ok {
		return fmt.Errorf("no such file or directory: %s", path)
	}

	if err = pDir.Unlink(name); err != nil {
		return err
	}

	//TODO: is it the callers responsibility to flush?
	//return pDir.Flush()
	return nil
}
