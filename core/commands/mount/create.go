package fusemount

import (
	"fmt"
	dag "gx/ipfs/QmPJNbVw8o3ohC43ppSXyNXwYKsWShG4zygnirHptfbHri/go-merkledag"
	coreoptions "gx/ipfs/QmXLwxifxwfc2bAwq6rdjbYqAsGzWsDE9RM5TWMGtykyj6/interface-go-ipfs-core/options"
	mfs "gx/ipfs/Qmb74fRYPgpjYzoBV7PAVNmP3DQaRrh8dHdKE4PwnF3cRx/go-mfs"
	unixfs "gx/ipfs/QmcYUTQ7tBZeH1CLsZM2S3xhMEZdvUgXvbjhpMsLDpk3oJ/go-unixfs"
	"os"
	gopath "path"

	"github.com/billziss-gh/cgofuse/fuse"
)

//TODO: lock parent?
func (fs *FUSEIPFS) Link(origin, target string) int {
	fs.Lock()
	defer fs.Unlock()
	log.Errorf("Link - Request %q -> %q", origin, target)

	switch parsePathType(target) {
	case tIPNS:
		log.Errorf("Link - IPNS support not implemented yet %q", target)
		return -fuse.EROFS
	case tMFS:
		fErr, gErr := mfsSymlink(fs.filesRoot, target, target[frs:])
		if gErr != nil {
			log.Errorf("Link - mfs error: %s", gErr)
		}
		return fErr
	default:
		log.Errorf("Link - Unexpected request %q", target)
		return -fuse.ENOENT
	}
}

//TODO: Upon successful completion, link() shall mark for update the st_ctime field of the file. Also, the st_ctime and st_mtime fields of the directory that contains the new entry shall be marked for update.
//TODO: lock parent
func (fs *FUSEIPFS) Symlink(target, linkActual string) int {
	fs.Lock()
	defer fs.Unlock()
	log.Debugf("Symlink - Request %q -> %q", target, linkActual)

	switch parsePathType(linkActual) {
	case tIPNS:
		log.Errorf("Symlink - IPNS support not implemented yet %q", linkActual)
		return -fuse.EROFS
	case tMFS:
		fErr, gErr := mfsSymlink(fs.filesRoot, target, linkActual[frs:])
		if gErr != nil {
			log.Errorf("Symlink - error: %s", gErr)
		}
		return fErr
	default:
		log.Errorf("Symlink - Unexpected request %q", linkActual)
		return -fuse.ENOENT
	}
}

func mfsSymlink(filesRoot *mfs.Root, target, linkActual string) (int, error) {
	linkDir, linkName := gopath.Split(linkActual)
	log.Errorf("mfsSymlink 1 - %q -> %q", target, linkActual)

	mfsNode, err := mfs.Lookup(filesRoot, linkDir)
	if err != nil {
		return -fuse.ENOENT, err
	}
	log.Errorf("mfsSymlink 2 - %q -> %q", target, linkActual)
	mfsDir, ok := mfsNode.(*mfs.Directory)
	if !ok {
		return -fuse.ENOTDIR, fmt.Errorf("%s was not a directory", linkDir)
	}
	log.Errorf("mfsSymlink 3 - %q -> %q", target, linkActual)

	dagData, err := unixfs.SymlinkData(target)
	if err != nil {
		log.Errorf("mfsSymlink I/O sunk %q:%s", linkActual, err)
		return -fuse.EIO, err
	}
	log.Errorf("mfsSymlink 4 - %q -> %q", target, linkActual)

	dagNode := dag.NodeWithData(dagData)
	dagNode.SetCidBuilder(mfsDir.GetCidBuilder())
	log.Errorf("mfsSymlink 5 - %q -> %q", target, linkActual)

	if err := mfsDir.AddChild(linkName, dagNode); err != nil {
		log.Errorf("mfsSymlink I/O sunk %q:%s", linkActual, err)
		return -fuse.EIO, err
	}
	log.Errorf("mfsSymlink 6 - %q -> %q", target, linkActual)
	return 0, nil
}

//TODO: lock parent
func (fs *FUSEIPFS) Create(path string, flags int, mode uint32) (int, uint64) {
	log.Debugf("Create - Request[m:%o]{f:%o} %q", mode, flags, path)
	return fs.Open(path, flags)
}

func (fs *FUSEIPFS) Mknod(path string, mode uint32, dev uint64) int {
	fs.Lock()
	defer fs.Unlock()
	log.Debugf("Mknod - Request [%X]{%X}%q", mode, dev, path)

	// TODO: abstract this:	node.PLock(){self.parent.lock();self.lock()} // goes up to soft-root max; i.e ipns-key, mfs-root

	parentPath := gopath.Dir(path)
	parent, err := fs.LookupPath(parentPath)
	if err != nil {
		log.Errorf("Mknod - could not fetch/lock parent for %q", path)
	}
	parent.Lock()
	defer parent.Unlock()
	//
	fErr, gErr := fs.mknod(path)
	if gErr != nil {
		log.Errorf("Mknod - %s", gErr)
	}

	fs.cc.ReleasePath(parentPath)
	return fErr
}

//TODO: inline this
func (fs FUSEIPFS) mknod(path string) (int, error) {
	parsedNode, err := fs.LookupPath(path)
	if err == nil {
		return -fuse.EEXIST, os.ErrExist
	}
	if err != os.ErrNotExist {
		return -fuse.EIO, err
	}

	switch parsedNode.(type) {
	case *ipnsKey:
		_, keyName := gopath.Split(path)
		coreKey, err := fs.core.Key().Generate(fs.ctx, keyName)
		if err != nil {
			return -fuse.EIO, fmt.Errorf("could not generate IPNS key %q: %s", keyName, err)
		}
		newRootNode, err := emptyNode(fs.ctx, fs.core.Dag(), unixfs.TFile, nil)
		if err != nil {
			return -fuse.EIO, fmt.Errorf("could not generate unixdir %q: %s", keyName, err)
		}

		err = fs.ipnsDelayedPublish(coreKey, newRootNode)
		if err != nil {
			return -fuse.EIO, fmt.Errorf("could not publish to key %q: %s", keyName, err)
		}
		return fuseSuccess, nil

	case *ipnsNode:
		return fs.ipnsMknod(path)

	case *mfsNode:
		return mfsMknod(fs.filesRoot, path[frs:])
	}

	return -fuse.EROFS, fmt.Errorf("unexpected request {%T}%q", parsedNode, path)
}

func mfsMknod(filesRoot *mfs.Root, path string) (int, error) {
	if _, err := mfs.Lookup(filesRoot, path); err == nil {
		return -fuse.EEXIST, fmt.Errorf("%q already exists", path)
	}

	dirName, fName := gopath.Split(path)
	mfsNode, err := mfs.Lookup(filesRoot, dirName)
	if err != nil {
		return -fuse.ENOENT, err
	}
	mfsDir, ok := mfsNode.(*mfs.Directory)
	if !ok {
		return -fuse.ENOTDIR, fmt.Errorf("%s is not a directory", dirName)
	}

	dagNode := dag.NodeWithData(unixfs.FilePBData(nil, 0))
	dagNode.SetCidBuilder(mfsDir.GetCidBuilder())

	err = mfsDir.AddChild(fName, dagNode)
	if err != nil {
		log.Errorf("mfsMknod I/O sunk %q:%s", path, err)
		return -fuse.EIO, err
	}

	return fuseSuccess, nil
}

func (fs *FUSEIPFS) Mkdir(path string, mode uint32) int {
	fs.Lock()
	defer fs.Unlock()
	log.Debugf("Mkdir - Request {%X}%q", mode, path)

	parentPath := gopath.Dir(path)
	parent, err := fs.LookupPath(parentPath)
	if err != nil {
		log.Errorf("Mkdir - could not fetch/lock parent for %q", path)
	}
	parent.Lock()
	defer parent.Unlock()
	defer fs.cc.ReleasePath(parentPath) //TODO: don't do this on failure

	switch parsePathType(path) {
	case tMFS:
		//TODO: review mkdir opts + Mkdir POSIX specs (are intermediate paths allowed by default?)
		if err := mfs.Mkdir(fs.filesRoot, path[frs:], mfs.MkdirOpts{Flush: mfsSync}); err != nil {
			if err == mfs.ErrDirExists || err == os.ErrExist {
				return -fuse.EEXIST
			}
			log.Errorf("Mkdir - unexpected error - %s", err)
			return -fuse.EACCES
		}
		return fuseSuccess
	case tIPNSKey: //TODO: refresh fs.nameRoots
		_, keyName := gopath.Split(path)
		coreKey, err := fs.core.Key().Generate(fs.ctx, keyName)
		if err != nil {
			log.Errorf("Mkdir - could not generate IPNS key %q: %s", keyName, err)
			return -fuse.EACCES
		}
		newRootNode, err := emptyNode(fs.ctx, fs.core.Dag(), unixfs.TDirectory, nil)
		if err != nil {
			log.Errorf("Mkdir - could not generate unixdir %q: %s", keyName, err)
			return -fuse.EACCES
		}

		err = fs.ipnsDelayedPublish(coreKey, newRootNode)
		if err != nil {
			log.Errorf("Mkdir - could not publish to key %q: %s", keyName, err)
			return -fuse.EACCES
		}

		pbNode, ok := newRootNode.(*dag.ProtoNode)
		if !ok { //this should never happen
			log.Errorf("IPNS key %q has incompatible type %T", keyName, newRootNode)
			fs.nameRoots[keyName] = nil
			return -fuse.EACCES
		}

		oAPI, err := fs.core.WithOptions(coreoptions.Api.Offline(true))
		if err != nil {
			log.Errorf("offline API could not be created: %s", err)
			fs.nameRoots[keyName] = nil
			return -fuse.EACCES
		}
		keyRoot, err := mfs.NewRoot(fs.ctx, fs.core.Dag(), pbNode, ipnsPublisher(keyName, oAPI.Name()))
		if err != nil {
			log.Errorf("IPNS key %q could not be mapped to MFS root: %s", keyName, err)
			fs.nameRoots[keyName] = nil
			return -fuse.EACCES
		}
		fs.nameRoots[keyName] = keyRoot

		return fuseSuccess

	case tIPNS:
		fErr, gErr := fs.ipnsMkdir(path)
		if gErr != nil {
			log.Errorf("Mkdir - error: %s", gErr)
		}
		return fErr

	case tMountRoot, tIPFSRoot, tFilesRoot:
		log.Errorf("Mkdir - requested a root entry - %q", path)
		return -fuse.EEXIST
	}

	log.Errorf("Mkdir - unexpected request %q", path)
	return -fuse.ENOENT
}

func (fs *FUSEIPFS) ipnsMkdir(path string) (int, error) {
	keyRoot, subPath, err := fs.ipnsMFSSplit(path)
	if err != nil {
		return -fuse.EACCES, err
	}

	//NOTE: must flush/publish otherwise our resolver is never going to pick up the change
	if err := mfs.Mkdir(keyRoot, subPath, mfs.MkdirOpts{Flush: false}); err != nil {
		if err == mfs.ErrDirExists || err == os.ErrExist {
			return -fuse.EEXIST, err
		}
		return -fuse.EACCES, err
	}

	if err := mfs.FlushPath(keyRoot, subPath); err != nil {
		return -fuse.EACCES, err
	}

	return fuseSuccess, nil
}

func (fs *FUSEIPFS) ipnsMknod(path string) (int, error) {
	keyRoot, subPath, err := fs.ipnsMFSSplit(path)
	if err != nil {
		return -fuse.EIO, err
	}
	blankNode, err := emptyNode(fs.ctx, fs.core.Dag(), unixfs.TFile, nil)
	if err != nil {
		return -fuse.EIO, err
	}
	if err := mfs.PutNode(keyRoot, subPath, blankNode); err != nil {
		return -fuse.EIO, err
	}

	if err := mfs.FlushPath(keyRoot, subPath); err != nil {
		return -fuse.EACCES, err
	}

	return fuseSuccess, nil
}
