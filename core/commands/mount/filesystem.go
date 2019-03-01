package fusemount

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"sync"

	"github.com/billziss-gh/cgofuse/fuse"
	mi "github.com/ipfs/go-ipfs/core/commands/mount/interface"

	dag "gx/ipfs/QmPJNbVw8o3ohC43ppSXyNXwYKsWShG4zygnirHptfbHri/go-merkledag"
	coreiface "gx/ipfs/QmXLwxifxwfc2bAwq6rdjbYqAsGzWsDE9RM5TWMGtykyj6/interface-go-ipfs-core"
	coreoptions "gx/ipfs/QmXLwxifxwfc2bAwq6rdjbYqAsGzWsDE9RM5TWMGtykyj6/interface-go-ipfs-core/options"
	mfs "gx/ipfs/Qmb74fRYPgpjYzoBV7PAVNmP3DQaRrh8dHdKE4PwnF3cRx/go-mfs"
	logging "gx/ipfs/QmbkT7eMTyXfpeyB3ZMxxcxg7XH8t6uXp49jqzz4HB7BGF/go-log"
)

// NOTE: readdirplus isn't supported on all platforms, being aware of this reduces duplicate metadata construction
// alter InvokeMount mount to have opts struct opts{core:coreapi, readdir:bool...})
var (
	log = logging.Logger("mount")

	//dbg stuff
	fReaddirPlus    = false //TODO: this has to be passed to us, this is only set here for debugging
	mfsSync         = false
	cidCacheEnabled = true

	errNoLink        = errors.New("not a symlink")
	errInvalidHandle = errors.New("invalid handle")
	errNoKey         = errors.New("key not found")
	errInvalidPath   = errors.New("invalid path")
	errInvalidArg    = errors.New("invalid argument")
	errReadOnly      = errors.New("read only section")
)

const fuseSuccess = 0

//TODO: platform specific routine mountArgs()
func InvokeMount(mountPoint string, filesRoot *mfs.Root, api coreiface.CoreAPI, ctx context.Context) (fsi mi.Interface, err error) {
	//TODO: if mountPoint == default; platform switch; win32 = \\.\ipfs\$mountpoint

	hcf := make(chan error)
	//TODO: mux the parent context with out own context, for now they're the same
	fs := &FUSEIPFS{core: api, filesRoot: filesRoot, signal: hcf, parentCtx: ctx, ctx: ctx, mountPoint: mountPoint}
	fs.fuseHost = fuse.NewFileSystemHost(fs)

	//TODO: fsh.SetCapReaddirPlus(true)
	fs.fuseHost.SetCapCaseInsensitive(false)

	//FIXME: cgofuse has its own signal/interrupt handler; need to ctrl+c twice
	go func() {
		defer func() {
			//TODO: insert platform specific separation here
			if r := recover(); r != nil {
				if typedR, ok := r.(string); ok {
					if typedR == "cgofuse: cannot find winfsp" {
						err = errors.New("WinFSP(http://www.secfs.net/winfsp/) is required for mount on this platform, but it was not found")
					}
					err = errors.New(typedR)
					hcf <- err
					return
				}
				err = fmt.Errorf("Mount panicked! %v", r)
				hcf <- err
			}
		}()

		//triggers init() which returns on signal channel
		if runtime.GOOS == "windows" {
			//FIXME: Volume prefix result in WinFSP err c000000d
			// ^ breaks UNC paths
			//fsh.Mount(mountPoint, []string{"-o uid=-1,gid=-1,fstypename=IPFS", "--VolumePrefix=\\ipfs"})
			fs.fuseHost.Mount(mountPoint, []string{"-o uid=-1,gid=-1,fstypename=IPFS"})
		} else {
			fs.fuseHost.Mount(mountPoint, nil)
		}
	}()

	if err = <-hcf; err != nil {
		log.Error(err)
		return
	}

	fs.active = true
	fsi = fs
	return
}

type FUSEIPFS struct {
	// index's (lookup) lock
	// RLock should be retained for lookups
	// Lock should be retained when altering index (including cache)
	sync.RWMutex

	// provided by InvokeMount()
	core      coreiface.CoreAPI
	filesRoot *mfs.Root
	//for compliance with go-ipfs daemon
	parentCtx  context.Context
	signal     chan error
	mountPoint string
	fuseHost   *fuse.FileSystemHost // pointer to "self"; used to die via self.fuseHost.Unmount()

	// set in init
	active    bool
	ctx       context.Context
	roots     []directoryEntry
	cc        cidCache
	mountTime fuse.Timespec

	// NOTE: heap equivalent; prevent Go gc claiming our objects between FS operations
	fileHandles map[uint64]*fileHandle
	dirHandles  map[uint64]*dirHandle
	nameRoots   map[string]*mfs.Root //TODO: see note on nameYieldFileIO
}

/*
func (ri *rootIndex) requestFh() uint64 {
	ri.Lock()
	for {
		ri.indexCursor++
		if ri.indexCursor <= specialIndexEnd {
			ri.indexCursor = passiveIndexLen
		}

		if _, ok := ri.activeHandles[ri.indexCursor]; !ok { // empty index
			ri.activeHandles[ri.indexCursor] = nil
			ri.Unlock()
			return ri.indexCursor
		}
	}
}
*/

func (fs *FUSEIPFS) Init() {
	fs.Lock()
	defer fs.Unlock()
	log.Debug("init")

	// return error on channel set by our invoker
	var chanErr error
	defer func() {
		fs.signal <- chanErr
	}()

	if chanErr = fs.cc.Init(); chanErr != nil {
		log.Errorf("[FATAL] cache could not be initialized: %s", chanErr)
		return
	}

	fs.roots = []directoryEntry{
		directoryEntry{label: "ipfs"},
		directoryEntry{label: "ipns"},
		directoryEntry{label: filesNamespace},
	}

	oAPI, chanErr := fs.core.WithOptions(coreoptions.Api.Offline(true))
	if chanErr != nil {
		log.Errorf("[FATAL] offline API could not be initialized: %s", chanErr)
		return
	}

	nameKeys, chanErr := fs.core.Key().List(fs.ctx)
	if chanErr != nil {
		log.Errorf("[FATAL] IPNS keys could not be listed: %s", chanErr)
		return
	}

	fs.nameRoots = make(map[string]*mfs.Root)
	for _, key := range nameKeys {
		keyNode, scopedErr := oAPI.ResolveNode(fs.ctx, key.Path())
		if scopedErr != nil {
			log.Warning("IPNS key %q could not be resolved: %s", key.Name(), scopedErr)
			fs.nameRoots[key.Name()] = nil
			continue
		}

		pbNode, ok := keyNode.(*dag.ProtoNode)
		if !ok {
			log.Warningf("IPNS key %q has incompatible type %T", key.Name(), keyNode)
			fs.nameRoots[key.Name()] = nil
			continue
		}

		keyRoot, scopedErr := mfs.NewRoot(fs.ctx, fs.core.Dag(), pbNode, ipnsPublisher(key.Name(), oAPI.Name()))
		if scopedErr != nil {
			log.Warningf("IPNS key %q could not be mapped to MFS root: %s", key.Name(), scopedErr)
			fs.nameRoots[key.Name()] = nil
			continue
		}
		fs.nameRoots[key.Name()] = keyRoot
	}

	fs.fileHandles = make(map[uint64]*fileHandle)
	fs.dirHandles = make(map[uint64]*dirHandle)

	fs.mountTime = fuse.Now()

	//TODO: implement for real
	go fs.dbgBackgroundRoutine()
	log.Debug("init finished")
}

type fileHandle struct {
	record fusePath
	io     FsFile
}

type dirHandle struct {
	record fusePath
	io     FsDirectory
}

func (fs *FUSEIPFS) Open(path string, flags int) (int, uint64) {
	fs.Lock()
	defer fs.Unlock()
	log.Debugf("Open - Request {%X}%q", flags, path)

	if fs.AvailableHandles(aFiles) == 0 {
		log.Error("Open - all handle slots are filled")
		return -fuse.ENFILE, invalidIndex
	}

	if flags&fuse.O_CREAT != 0 {
		if flags&fuse.O_EXCL != 0 {
			_, err := fs.LookupPath(path)
			if err == nil {
				log.Errorf("Open/Create - exclusive flag provided but %q already exists", path)
				return -fuse.EEXIST, invalidIndex
			}
		}
		fErr, gErr := fs.mknod(path)
		if gErr != nil {
			log.Errorf("Open/Create - %q: %s", path, gErr)
			return fErr, invalidIndex
		}
	}

	fsNode, err := fs.LookupPath(path)
	if err != nil {
		log.Errorf("Open - path err: %s", err)
		return -fuse.ENOENT, invalidIndex
	}
	fsNode.Lock()
	defer fsNode.Unlock()

	fh, err := fs.newFileHandle(fsNode) //TODO: flags
	if err != nil {
		log.Errorf("Open - sunk %q:%s", fsNode.String(), err)
		return -fuse.EIO, invalidIndex
	}

	log.Debugf("Open - Assigned [%X]%q", fh, fsNode)
	return fuseSuccess, fh
}

func (fs *FUSEIPFS) Opendir(path string) (int, uint64) {
	fs.Lock()
	defer fs.Unlock()
	log.Debugf("Opendir - Request %q", path)

	if fs.AvailableHandles(aDirectories) == 0 {
		log.Error("Opendir - all handle slots are filled")
		return -fuse.ENFILE, invalidIndex
	}

	fsNode, err := fs.LookupPath(path)
	if err != nil {
		log.Errorf("Opendir - path err %q: %s", path, err)
		return -fuse.ENOENT, invalidIndex
	}
	fsNode.Lock()
	defer fsNode.Unlock()

	if !fReaddirPlus {
		fh, err := fs.newDirHandle(fsNode)
		if err != nil {
			log.Errorf("Opendir - %s", err)
			return -fuse.ENOENT, invalidIndex
		}
		log.Debugf("Opendir - Assigned [%X]%q", fh, fsNode)
		return fuseSuccess, fh
	}

	return -fuse.EACCES, invalidIndex
}

//TODO: [educational/compiler] how costly is defer vs {ret = x; unlock; return ret}
func (fs *FUSEIPFS) Release(path string, fh uint64) int {
	log.Debugf("Release - [%X]%q", fh, path)
	fs.Lock()
	defer fs.Unlock()
	return fs.releaseFileHandle(fh)
}

func (fs *FUSEIPFS) Releasedir(path string, fh uint64) int {
	log.Debugf("Releasedir - [%X]%q", fh, path)
	fs.Lock()
	defer fs.Unlock()
	return fs.releaseDirHandle(fh)
}

//TODO: implement
func (fs *FUSEIPFS) Chmod(path string, mode uint32) int {
	log.Errorf("Chmod [%X]%q", mode, path)
	return 0
}

func (fs *FUSEIPFS) Chown(path string, uid, gid uint32) int {

	log.Errorf("Chmod [%d:%d]%q", uid, gid, path)
	return 0
}

//HCF
func (fs *FUSEIPFS) Destroy() {
	log.Debugf("Destroy requested")

	//NOTE: ideally the invoker would release us at some point
	// we could require a pointer in InvokeMount() and nil it here though
	// fs.daemonPtr = nil; cleanup...; return
	fs.active = false
	fs.mountPoint = ""

	if !mfsSync { //TODO: do this anyway?
		if err := mfs.FlushPath(fs.filesRoot, "/"); err != nil {
			log.Errorf("MFS failed to sync: %s", err)
		}
	}

	//TODO: close our context
}

func (fs *FUSEIPFS) Flush(path string, fh uint64) int {
	fs.RLock()
	log.Debugf("Flush - Request [%X]%q", fh, path)

	h, err := fs.LookupFileHandle(fh)
	if err != nil {
		fs.RUnlock()
		log.Errorf("Flush - bad request [%X]%q: %s", fh, path, err)
		if err == errInvalidHandle {
			return -fuse.EBADF //TODO: we might want to change this to EIO since Flush is called on Close which implies the handle should have been valid
		}
		return -fuse.EIO
	}
	h.record.Lock()
	fs.RUnlock()
	defer h.record.Unlock()

	if !h.record.Mutable() {
		return fuseSuccess
	}

	return h.io.Sync()
}

func (fs *FUSEIPFS) Fsync(path string, datasync bool, fh uint64) int {
	fs.RLock()
	log.Debugf("Fsync - Request [%X]%q", fh, path)

	h, err := fs.LookupFileHandle(fh)
	if err != nil {
		fs.RUnlock()
		log.Errorf("Fsync - bad request [%X]%q: %s", fh, path, err)
		if err == errInvalidHandle {
			return -fuse.EBADF
		}
		return -fuse.EIO
	}

	h.record.Lock()
	fs.RUnlock()
	defer h.record.Unlock()

	return h.io.Sync()
}

func (fs *FUSEIPFS) Fsyncdir(path string, datasync bool, fh uint64) int {
	fs.RLock()
	log.Errorf("Fsyncdir - Request [%X]%q", fh, path)

	fsDirNode, err := fs.LookupDirHandle(fh)
	if err != nil {
		fs.RUnlock()
		log.Errorf("Fsyncdir - [%X]%q: %s", fh, path, err)
		if err == errInvalidHandle {
			return -fuse.EBADF
		}
		return -fuse.EIO
	}
	fsDirNode.record.Lock()
	fs.RUnlock()
	defer fsDirNode.record.Unlock()

	/* FIXME: not implemented
	fsDirNode, err := dirFromHandle(fh)
	if err != nil {
		fs.Unlock()
		log.Errorf("Fsyncdir - [%X]%q: %s", fh, path, err)
		if err == errInvalidHandle {
			return -fuse.EBADF
		}
		return -fuse.EIO
	}

	return fsDirNode.Sync()
	*/
	return fuseSuccess
}

func (fs *FUSEIPFS) Getxattr(path, name string) (int, []byte) {
	log.Errorf("Getxattr")
	return -fuse.ENOSYS, nil
}

func (fs *FUSEIPFS) Listxattr(path string, fill func(name string) bool) int {
	log.Errorf("Listxattr")
	return -fuse.ENOSYS
}

func (fs *FUSEIPFS) Removexattr(path, name string) int {
	log.Errorf("Removexattr")
	return -fuse.ENOSYS
}

func (fs *FUSEIPFS) Setxattr(path, name string, value []byte, flags int) int {
	log.Errorf("Setxattr")
	return -fuse.ENOSYS
}

func (fs *FUSEIPFS) Readlink(path string) (int, string) {
	fs.RLock()
	log.Debugf("Readlink - Request %q", path)
	fsNode, err := fs.LookupPath(path)
	if err != nil {
		log.Debugf("Readlink - lookup: %s", err)
		fs.RUnlock()
		return -fuse.ENOENT, ""
	}

	fsNode.RLock()
	fs.RUnlock()
	defer fsNode.RUnlock()

	if isDevice(fsNode) {
		return -fuse.EINVAL, ""
	}

	if isReference(fsNode) {
		var err error
		fsNode, err = fs.resolveToGlobal(fsNode)
		if err != nil {
			log.Errorf("Readlink - node resolution error: %s", err)
			return -fuse.EIO, ""
		}
	}

	target, err := fs.fuseReadlink(fsNode)
	if err != nil {
		if err == errNoLink {
			log.Errorf("Readlink - %q is not a symlink", path)
			return -fuse.EINVAL, ""
		}
		log.Errorf("Readlink - unexpected link resolution error: %s", err)
		return -fuse.EIO, ""
	}

	return len(target), target
}

type handleErrorPair struct {
	fhi uint64
	err error
}

//TODO: test this
func (fs *FUSEIPFS) refreshFileSiblings(fh uint64, h *fileHandle) (failed []handleErrorPair) {
	handles := *h.record.Handles()
	if len(handles) == 1 && handles[0] == fh {
		return
	}

	for _, fhi := range handles {
		if fhi == fh {
			continue
		}
		curFh, err := fs.LookupFileHandle(fhi)
		if err != nil {
			failed = append(failed, handleErrorPair{fhi, err})
			continue
		}

		oCur, err := curFh.io.Seek(0, io.SeekCurrent)
		if err != nil {
			failed = append(failed, handleErrorPair{fhi, err})
			continue
		}
		err = curFh.io.Close()
		if err != nil {
			failed = append(failed, handleErrorPair{fhi, err})
			continue
		}
		curFh.io = nil

		posixIo, err := fs.yieldFileIO(h.record)
		if err != nil {
			failed = append(failed, handleErrorPair{fhi, err})
			continue
		}

		posixIo.Seek(oCur, io.SeekStart)
		curFh.io = posixIo
	}
	return
}
