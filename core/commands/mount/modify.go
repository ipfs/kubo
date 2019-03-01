package fusemount

import (
	"github.com/billziss-gh/cgofuse/fuse"

	mfs "gx/ipfs/Qmb74fRYPgpjYzoBV7PAVNmP3DQaRrh8dHdKE4PwnF3cRx/go-mfs"
)

/* TODO
func (*FileSystemBase) Setxattr(path string, name string, value []byte, flags int) int
*/

//FIXME: obtain lock on oldpath, invalidate handles, etc.
//TODO: see what parts mfs errors on internally; don't reimplement
func (fs *FUSEIPFS) Rename(oldpath, newpath string) int {
	fs.Lock()
	defer fs.Unlock()
	if oldpath == newpath {
		return fuseSuccess
	}

	//TODO: If either the old or new argument names a symbolic link, rename() shall operate on the symbolic link itself, and shall not resolve the last component of the argument.

	/* TODO
	if type(oldpath) != type(newpath) {
	    if type(newpath) == directory {
		return -fuse.EISDIR
	    } else {
		return -fuse.ENOTDIR
	    }
	}
	*/

	/* TODO: current
	lookup src
	check src.mode for type
	if exists; fusePut(srcNode, dstPath)

	ipldPut(src *ipld.Node, dst string)
	lookup dst
	if exists; compare types
	check dst.mode for write access; create()


	*/

	//oldNode, newNode := fs

	//TODO: enforce: Write access permission is required for both the directory containing old and the directory containing new.
	if err := mfs.Mv(fs.filesRoot, oldpath[frs:], newpath[frs:]); err != nil {
		log.Errorf("Rename - %s", err)
		return -fuse.ENOENT //TODO: real error
	}
	return fuseSuccess
}

/* inline this
func (fs FUSEIPFS) ipldPut(nd *ipld.Node, path string) {
	target, err := fs.LookupPath(path)
	if err == nil {
		//file exists, check type compatibility
	}
	if err != nil && err != os.ErrNotExist {
		//bad things
	}
}
*/

//TODO: document; filesystem locks; FS writes = would alter Lookup(), node writes = alters node data (meta or actual)
func (fs *FUSEIPFS) Utimens(path string, tmsp []fuse.Timespec) int {
	fs.RLock()
	log.Debugf("Utimens - Request %v %q", tmsp, path)

	fsNode, err := fs.LookupPath(path)
	if err != nil {
		log.Error(err) //TODO: msg
		fs.RUnlock()
		return -fuse.ENOENT
	}

	fsNode.Lock()
	fs.RUnlock()
	fStat := fsNode.Stat()
	fStat.Atim = tmsp[0]
	fStat.Mtim = tmsp[1]
	fsNode.Unlock()
	return fuseSuccess
}

func (fs *FUSEIPFS) Truncate(path string, size int64, fh uint64) int {
	fs.Lock()
	defer fs.Unlock()
	log.Debugf("Truncate - req [%X]{%d}%q", fh, size, path)

	if size < 0 {
		return -fuse.EINVAL
	}

	/* TODO [POISX]
	    if size > max-size {
		return -fuse.EFBIG
	    }
	*/

	if h, err := fs.LookupFileHandle(fh); err == nil {
		h.record.Lock()
		defer h.record.Unlock()
		fErr, gErr := h.io.Truncate(size)
		if gErr != nil {
			log.Errorf("Truncate - [%X]%q:%s", fh, path, gErr)
		} else {
			h.record.Stat().Size = size
		}
		return fErr
	}

	fsNode, err := fs.LookupPath(path)
	if err != nil {
		log.Errorf("Truncate - %q:%s", path, err)
		return -fuse.ENOENT
	}

	fsNode.Lock()
	defer fsNode.Unlock()

	io, err := fs.yieldFileIO(fsNode)
	if err != nil {
		log.Errorf("Truncate - %q:%s", path, err)
		return -fuse.EIO
	}

	fErr, gErr := io.Truncate(size)
	if gErr != nil {
		log.Errorf("Truncate - %q:%s", path, gErr)
	} else {
		fsNode.Stat().Size = size
	}
	return fErr
}
