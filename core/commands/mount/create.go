package fusemount

import (
	"fmt"
	dag "gx/ipfs/QmPJNbVw8o3ohC43ppSXyNXwYKsWShG4zygnirHptfbHri/go-merkledag"
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
	case tFAPI:
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
	case tFAPI:
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
	fErr, gErr := fs.Mk(path, mode, unixfs.TFile)
	if gErr != nil {
		log.Errorf("Mknod - %q: %s", path, gErr)
	}
	return fErr
}

//TODO: wrap this; overlaps with mknod
func (fs *FUSEIPFS) Mkdir(path string, mode uint32) int {
	fs.Lock()
	defer fs.Unlock()
	log.Debugf("Mkdir - Request {%X}%q", mode, path)
	fErr, gErr := fs.Mk(path, mode, unixfs.TDirectory)
	if gErr != nil {
		log.Errorf("Mkdir - %q: %s", path, gErr)
	}
	return fErr
}

func (fs *FUSEIPFS) Mk(path string, mode uint32, nodeType FsType) (ret int, err error) {
	defer func() {
		if nodeType == unixfs.TDirectory && ret == -fuse.EIO {
			ret = -fuse.EACCES // same interface, different return values
		}
	}()

	//NOTE: mode is not expected to be portable, except for FIFO pipes
	// which are not implemented, but could be through named p2p sockets in theory
	// handling of allowing the OS to create directories with mknod could also be handled here

	//TODO: attain parent lock first

	fsNode, err := fs.shallowLookupPath(path)
	switch err {
	case nil:
		ret, err = -fuse.EEXIST, os.ErrExist
		return
	case os.ErrNotExist:
		break
	default:
		ret, err = -fuse.EIO, err
		return
	}

	fsNode.Lock()
	defer fsNode.Unlock()

	callContext, cancel := deriveCallContext(fs.ctx)
	defer cancel()

	if ret, err = fsNode.Create(callContext, unixfs.TFile); err != nil {
		return
	}

	var nodeStat *fuse.Stat_t
	if nodeStat, err = fsNode.InitMetadata(callContext); err != nil {
		ret = -fuse.EIO
		return
	}

	// TODO update parent meta

	return
}
