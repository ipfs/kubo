package fusemount

import (
	"context"
	"fmt"
	"io"
	"sync"

	coreiface "gx/ipfs/QmXLwxifxwfc2bAwq6rdjbYqAsGzWsDE9RM5TWMGtykyj6/interface-go-ipfs-core"
	mfs "gx/ipfs/Qmb74fRYPgpjYzoBV7PAVNmP3DQaRrh8dHdKE4PwnF3cRx/go-mfs"

	"github.com/billziss-gh/cgofuse/fuse"
)

type fsHandles map[uint64]fusePath
type nodeHandles map[uint64]interface{} // *FsFile | *FsDirectory

type fusePath interface {
	coreiface.Path
	RWLocker

	InitMetadata(context.Context) (*fuse.Stat_t, error)
	Stat() (*fuse.Stat_t, error)
	YieldIo(ctx context.Context, nodeType FsType) (io interface{}, err error)
	Handles() nodeHandles
	DestroyIo(handle uint64, nodeType FsType) (fuseReturn int, goErr error)
	Remove(context.Context) (int, error)
	Create(context.Context, FsType) (int, error)
	//GetIo(handle uint64) (io interface{}, err error) // FsFile | FsDirectory
	//Exists() bool
}

type RWLocker interface {
	sync.Locker
	RLock()
	RUnlock()
}

//NOTE: (int, error) pairs are translations of the appropriate return values across APIs, int for FUSE, error for Go
type FsFile interface {
	io.Reader
	io.Seeker
	io.Closer
	sync.Locker
	Size() (int64, error)
	Write(buff []byte, ofst int64) (int, error)
	Sync() (int, error)
	Truncate(size int64) (int, error)
	Record() fusePath
}

type FsDirectory interface {
	//Parent() Directory
	sync.Locker
	Entries() int
	Read(ctx context.Context, offset int64) <-chan directoryFuseEntry
	Record() fusePath
}

type FsLink interface {
	Target() string // Link
}

type FsRecord interface {
	Record() fusePath
}

//NOTE: caller should retain FS Lock
func (fs *FUSEIPFS) AvailableHandles() uint64 {
	return (invalidIndex - 1) - uint64(len(fs.handles))
}

func (fs *FUSEIPFS) Close() error {
	//log.whatever
	if !fs.fuseHost.Unmount() {
		return fmt.Errorf("Could not unmount %q, reason unknown", fs.mountPoint)
	}
	return nil
}

func (fs *FUSEIPFS) IsActive() bool {
	return fs.active
}

func (fs *FUSEIPFS) Where() string {
	return fs.mountPoint
}

type nameRootIndex interface {
	sync.Locker
	Register(string, *mfs.Root)
	Request(string) (*mfs.Root, error)
	Release(string)
}

type mfsSharedIndex struct {
	sync.Mutex
	roots map[string]*mfsReference
}

type mfsReference struct {
	root *mfs.Root
	int  // refcount
}

func (mi *mfsSharedIndex) Register(subrootPath string, mroot *mfs.Root) {
	mi.roots[subrootPath] = &mfsReference{*mfs.Roots: mroot, int: 1}
}

func (mi *mfsSharedIndex) Request(subrootPath string) (*mfs.Root, error) {
	index, ok := mi.roots[subrootPath]
	if !ok || index == nil {
		return nil, errNotInitialized
	}
	index.int++
	return index.root, nil
}

func (mi *mfsSharedIndex) Release(subrootPath string) {
	index, ok := mi.roots[subrootPath]
	if !ok || index == nil {
		log.Errorf("shared index %q not found", subrootPath)
		return // panic?
	}
	if index.int--; index.int == 0 {
		delete(mi.roots, subrootPath)
	}

	//= mfsReference{*mfs.Roots: mroot}
}
