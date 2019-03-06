package fusemount

import (
	"context"
	"errors"
	"fmt"
	"os"
	gopath "path"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"github.com/billziss-gh/cgofuse/fuse"
	mi "github.com/ipfs/go-ipfs/core/commands/mount/interface"

	coreiface "gx/ipfs/QmXLwxifxwfc2bAwq6rdjbYqAsGzWsDE9RM5TWMGtykyj6/interface-go-ipfs-core"
	coreoptions "gx/ipfs/QmXLwxifxwfc2bAwq6rdjbYqAsGzWsDE9RM5TWMGtykyj6/interface-go-ipfs-core/options"
	mfs "gx/ipfs/Qmb74fRYPgpjYzoBV7PAVNmP3DQaRrh8dHdKE4PwnF3cRx/go-mfs"
	logging "gx/ipfs/QmbkT7eMTyXfpeyB3ZMxxcxg7XH8t6uXp49jqzz4HB7BGF/go-log"
	unixfs "gx/ipfs/QmcYUTQ7tBZeH1CLsZM2S3xhMEZdvUgXvbjhpMsLDpk3oJ/go-unixfs"
	upb "gx/ipfs/QmcYUTQ7tBZeH1CLsZM2S3xhMEZdvUgXvbjhpMsLDpk3oJ/go-unixfs/pb"
)

// NOTE: readdirplus isn't supported on all platforms, being aware of this reduces duplicate metadata construction
// alter InvokeMount mount to have opts struct opts{core:coreapi, readdir:bool...})
var (
	log = logging.Logger("mount")

	//dbg stuff
	fReaddirPlus    = false //TODO: this has to be passed to us, this is only set here for debugging
	mfsSync         = false
	cidCacheEnabled = true

	//TODO: replace errNoLink (likely with errIOType)
	errNoLink         = errors.New("not a symlink")
	errInvalidHandle  = errors.New("invalid handle")
	errNoKey          = errors.New("key not found")
	errInvalidPath    = errors.New("invalid path")
	errInvalidArg     = errors.New("invalid argument")
	errReadOnly       = errors.New("read only section")
	errIOType         = errors.New("requested IO for node does not match node type")
	errUnexpected     = errors.New("unexpected node type")
	errNotInitialized = errors.New("node metadata is not initialized")
	errRoot           = errors.New("root initialization exception")
	errRecurse        = errors.New("hit recursion limit")
)

type typeToken = uint
type FsType = upb.Data_DataType

const (
	fuseSuccess = 0
)

const (
	invalidIndex = ^uint64(0)

	filesNamespace  = "files"
	filesRootPath   = "/" + filesNamespace
	filesRootPrefix = filesRootPath + "/"
	frs             = len(filesRootPath) //TODO: remove this; makes purpose less obvious where used
)

//TODO: configurable
const (
	callTimeout  = 10 * time.Second
	entryTimeout = callTimeout
	ipnsTTL      = 30 * time.Second
)

//context key tokens
type lookupKey struct{}
type dagKey struct{}
type lookupFn func(string) (fusePath, error)

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
	nameRoots nameRootIndex
	handles   fsHandles
	//TODO: ipnsKeys map[string:keyname]{fusePath,IoType?}
	cc        cidCache
	mountTime fuse.Timespec

	// NOTE: heap equivalent; prevent Go gc claiming our objects between FS operations
	//nameRoots map[string]*mfs.Root //TODO: see note on nameYieldFileIO

	/* implementation note
	Handles are addresses of IO objects/interfaces
	to assure they're not garbage collected, we store the IO's record in the filesystem scope during Open()
	the record itself stores the IO object on it
	in a map who's index is the same int (address of the IO object)
	*/
	/*
		Open(){
		    inode, iointerface, err := record.YieldIo(type) {
			io, err := yieldIo(record)
			inode := &io
			record.handles[
		    }
		    fs.handles[inode] = record
		}
		Read(){
		    io, err := record.GetIo(fh)
		    io.Read()
		}
	*/
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

	var subroots = [...]string{"/ipfs", "/ipns", filesRootPrefix}
	//TODO + keycount + ipns keys
	var ipnsKeys []coreiface.Key
	if ipnsKeys, chanErr = fs.core.Key().List(fs.ctx); chanErr != nil {
		log.Errorf("[FATAL] ipns keys could not be initialized: %s", chanErr)
		return
	}

	//TODO: shadow var check; must assign to chanErr
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

	fs.nameRoots = &mfsSharedIndex{roots: make(map[string]*mfsReference, len(nameKeys)+1)} // +Files API
	fs.handles = make(fsHandles)
	fs.mountTime = fuse.Now()

	log.Debug("init finished: %s", fs.mountTime)
}

func (fs *FUSEIPFS) Open(path string, flags int) (int, uint64) {
	fs.Lock()
	defer fs.Unlock()
	log.Debugf("Open - Request {%X}%q", flags, path)

	if fs.AvailableHandles() == 0 {
		log.Error("Open - all handle slots are filled")
		return -fuse.ENFILE, invalidIndex
	}

lookup:
	fsNode, indexErr := fs.shallowLookupPath(path)
	var nodeStat *fuse.Stat_t
	if indexErr == nil {
		var err error
		nodeStat, err = fsNode.Stat()
		if err != nil {
			log.Errorf("Open - %q: %s", path, err)
			return -fuse.EACCES, invalidIndex
		}
	}

	// POSIX specifications
	if flags&O_NOFOLLOW != 0 {
		if indexErr == nil {
			if nodeStat.Mode&fuse.S_IFMT == fuse.S_IFLNK {
				log.Errorf("Open - nofollow requested but %q is a link", path)
				return -fuse.ELOOP, invalidIndex
			}
		}
	}

	if flags&fuse.O_CREAT != 0 {
		switch indexErr {
		case os.ErrNotExist:
			nodeType := unixfs.TFile
			if flags&O_DIRECTORY != 0 {
				nodeType = unixfs.TDirectory
			}

			callContext, cancel := deriveCallContext(fs.ctx)
			defer cancel()
			fErr, gErr := fsNode.Create(callContext, nodeType)
			if gErr != nil {
				log.Errorf("Create - %q: %s", path, gErr)
				return fErr, invalidIndex
			}
			// node was created API side, clear create bits, jump back, and open it FS side
			// respecting link restrictions
			flags &^= (fuse.O_EXCL | fuse.O_CREAT)
			goto lookup

		case nil:
			if flags&fuse.O_EXCL != 0 {
				log.Errorf("Create - exclusive flag provided but %q already exists", path)
				return -fuse.EEXIST, invalidIndex
			}

			if nodeStat.Mode&fuse.S_IFMT == fuse.S_IFDIR {
				if flags&O_DIRECTORY == 0 {
					log.Error("Create - regular file requested but %q resolved to an existing directory", path)
					return -fuse.EISDIR, invalidIndex
				}
			}
		default:
			log.Errorf("Create - unexpected %q: %s", path, indexErr)
			return -fuse.EACCES, invalidIndex
		}
	}

	// Open proper -- resolves reference nodes
	fsNode, err := fs.LookupPath(path)
	if err != nil {
		log.Errorf("Open - path err: %s", err)
		return -fuse.ENOENT, invalidIndex
	}
	fsNode.Lock()
	defer fsNode.Unlock()

	nodeStat, err = fsNode.Stat()
	if err != nil {
		log.Errorf("Open - node %q not initialized", path)
		return -fuse.EACCES, invalidIndex
	}

	if nodeStat.Mode&fuse.S_IFMT != fuse.S_IFLNK {
		return -fuse.ELOOP, invalidIndex //NOTE: this should never happen, lookup should resolve all
	}

	// if request is dir but node is dir
	if (flags&O_DIRECTORY != 0) && (nodeStat.Mode&fuse.S_IFMT != fuse.S_IFDIR) {
		log.Error("Open - Directory requested but %q does not resolve to a directory", path)
		return -fuse.ENOTDIR, invalidIndex
	}

	// if request was file but node is dir
	if (flags&O_DIRECTORY == 0) && (nodeStat.Mode&fuse.S_IFMT == fuse.S_IFDIR) {
		log.Error("Open - regular file requested but %q resolved to a directory", path)
		return -fuse.EISDIR, invalidIndex
	}

	callContext, cancel := deriveCallContext(fs.ctx)
	defer cancel()

	// io is an interface that points to a struct (generic/void*)
	io, err := fsNode.YieldIo(callContext, unixfs.TFile)
	if err != nil {
		log.Errorf("Open - IO err %q %s", path, err)
		return -fuse.EIO, invalidIndex
	}

	// the address of io itself must remain the same across calls
	// as we are sharing it with the OS
	// we use the interface structure itself so that
	// on our side we can change data sources
	// without invalidating handles on the OS/client side
	ifPtr := &io                                     // void *ifPtr = (FsFile*) io;
	handle := uint64(uintptr(unsafe.Pointer(ifPtr))) // uint64_t handle = &ifPtr;
	fsNode.Handles()[handle] = ifPtr                 //GC prevention of our double pointer; free upon Release()

	log.Debugf("Open - Assigned [%X]%q", handle, fsNode)
	return fuseSuccess, handle
}

func (fs *FUSEIPFS) Opendir(path string) (int, uint64) {
	fs.Lock()
	defer fs.Unlock()
	log.Debugf("Opendir - Request %q", path)

	if fs.AvailableHandles() == 0 {
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

	lookupFn := func(child string) (fusePath, error) {
		//return fs.LookupPath(gopath.Join(path, child))
		return fs.shallowLookupPath(gopath.Join(path, child))
	}

	directoryContext := context.WithValue(fs.ctx, lookupKey{}, lookupFn)
	io, err := fsNode.YieldIo(directoryContext, unixfs.TDirectory)
	if err != nil {
		log.Errorf("Opendir - IO err %q %s", path, err)
		return -fuse.EACCES, invalidIndex
	}

	ifPtr := &io // see comments on Open()
	handle := uint64(uintptr(unsafe.Pointer(ifPtr)))
	fsNode.Handles()[handle] = ifPtr
	return fuseSuccess, handle
}

func (fs *FUSEIPFS) Release(path string, fh uint64) int {
	log.Debugf("Release - [%X]%q", fh, path)
	fs.Lock()
	defer fs.Unlock()

	fsNode, ok := fs.handles[fh]
	if !ok {
		log.Errorf("Release - handle %X is invalid", fh)
		return -fuse.EBADF
	}
	fErr, gErr := fsNode.DestroyIo(fh, unixfs.TFile)
	if gErr != nil {
		log.Errorf("Release - %s", gErr)
	}

	// invalidate/free handle regardless of grace
	delete(fs.handles, fh)
	return fErr
}

func (fs *FUSEIPFS) Releasedir(path string, fh uint64) int {
	log.Debugf("Releasedir - [%X]%q", fh, path)
	fs.Lock()
	defer fs.Unlock()
	fsNode, ok := fs.handles[fh]
	if !ok {
		log.Errorf("Releasedir - handle %X is invalid", fh)
		return -fuse.EBADF
	}
	fErr, gErr := fsNode.DestroyIo(fh, unixfs.TDirectory)
	if gErr != nil {
		log.Errorf("Releasedir - %s", gErr)
	}

	// invalidate/free handle regardless of grace
	delete(fs.handles, fh)
	return fErr
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
	//TODO: close all handles
}

func (fs *FUSEIPFS) Flush(path string, fh uint64) int {
	fs.Lock()
	defer fs.Unlock()
	log.Debugf("Flush - Request [%X]%q", fh, path)

	fErr, gErr := syncWrap(fh, 0, true) //XXX: optional parameter 0 is ignored from flush
	if gErr != nil {
		log.Errorf("Flush - [%X]%q: %s", fh, path, gErr)
	}
	return fErr
}

func (fs *FUSEIPFS) Fsync(path string, datasync bool, fh uint64) int {
	fs.Lock()
	defer fs.Unlock()
	log.Debugf("Fsync - Request [%X]%q", fh, path)

	fErr, gErr := syncWrap(fh, unixfs.TFile, false)
	if gErr != nil {
		log.Errorf("Fsync - [%X]%q: %s", fh, path, gErr)
	}
	return fErr
}

func (fs *FUSEIPFS) Fsyncdir(path string, datasync bool, fh uint64) int {
	fs.Lock()
	defer fs.Unlock()
	log.Errorf("Fsyncdir - Request [%X]%q", fh, path)

	fErr, gErr := syncWrap(fh, unixfs.TDirectory, false)
	if gErr != nil {
		log.Errorf("Fsyncdir - [%X]%q: %s", fh, path, gErr)
	}
	return fErr
}

func syncWrap(fh uint64, nodeType FsType, isFlush bool) (int, error) {
	fsNode, io, err := invertedLookup(fh)
	if err != nil {
		if err == errInvalidHandle {
			return -fuse.EBADF, err
		}
		return -fuse.EIO, err
	}
	fsNode.Lock()
	defer fsNode.Unlock()
	if !fsNode.Mutable() {
		return fuseSuccess, nil
	}

	fStat, err := fsNode.Stat()
	if err != nil {
		return -fuse.EIO, err
	}

	if !isFlush { // flush is an untyped request
		if !typeCheck(fStat.Mode, nodeType) {
			return -fuse.EINVAL, errIOType
		}
	}

	switch nodeType {
	case unixfs.TFile:
		return io.(FsFile).Sync()
	case unixfs.TDirectory:
		//NOOP
		return fuseSuccess, nil
	default:
		return -fuse.EIO, errUnexpected
	}
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

	//TODO:
	//lIo, err := fsNode.YieldIo(nil, unixfs.TSymlink)
	//

	fStat, err := fsNode.Stat()
	if err != nil {
		log.Errorf("Readlink - %q: %s", path, err)
		return -fuse.EIO, ""
	}

	if !typeCheck(fStat.Mode, unixfs.TSymlink) {
		log.Errorf("Readlink - %q is not a symlink", path)
		return -fuse.EINVAL, ""
	}

	//TODO: need global path for core

	callContext, cancel := deriveCallContext(fs.ctx)
	defer cancel()
	ipldNode, err := fs.core.ResolveNode(callContext, fsNode)
	if err != nil {
		log.Errorf("Readlink - node resolution error: %s", err)
		return 0, ""
	}

	ufsNode, err := unixfs.ExtractFSNode(ipldNode)
	if err != nil {
		return -fuse.EIO, ""
	}

	target := string(ufsNode.Data())
	tLen := len(target)
	if tLen != int(fStat.Size) {
		log.Errorf("Readlink - target size mismatched node:%d != target:%d", fStat.Size, tLen)
		return -fuse.EIO, ""
	}

	return tLen, target
}

//NOTE: caller should retain FS (R)Lock
// caller need not do asserting check; it's rolled into an error
func (fs *FUSEIPFS) getIo(fh uint64, ioType FsType) (io interface{}, err error) {
	err = errInvalidHandle
	if fh == invalidIndex {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			log.Errorf("getIo recovered from panic, likely invalid handle: %v", r)
			io = nil
			err = errInvalidHandle
		}
	}()

	// L0 dereference address; equivalent to io = *fh with trap
	// uint -> cast to untyped pointer -> pointer is dereferenced -> type is checked
	switch ioType {
	case unixfs.TFile:
		if io, ok := (*(*interface{})(unsafe.Pointer(uintptr(fh)))).(FsFile); ok {
			return io, nil
		}
		return nil, errIOType
	case unixfs.TDirectory:
		if io, ok := (*(*interface{})(unsafe.Pointer(uintptr(fh)))).(FsDirectory); ok {

			return io, nil
		}
		return nil, errIOType
	case unixfs.TSymlink:
		if io, ok := (*(*interface{})(unsafe.Pointer(uintptr(fh)))).(FsLink); ok {
			return io, nil
		}
		return nil, errIOType
	default:
		return nil, errUnexpected
	}

	// L1 double map lookup
	// record lookup -> io lookup -> io
	/*
		if record, ok := fs.handles[fh]; ok {
			if io, ok := record.Handles()[fh]; ok {
				switch ioType {
				case unixfs.TFile:
					if _, ok := io.(FsFile); ok {
						return io, nil
					}
					return nil, errIOType
				case unixfs.TDirectory:
					if _, ok := io.(FsDirectory); ok {
						return io, nil
					}
					return nil, errIOType
				default:
					return nil, errUnexpected
				}
			}
		}
	*/
}
