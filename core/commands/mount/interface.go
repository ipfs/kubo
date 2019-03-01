package fusemount

import (
	"fmt"
	"io"
	"sync"
	"time"
	"unsafe"

	coreiface "gx/ipfs/QmXLwxifxwfc2bAwq6rdjbYqAsGzWsDE9RM5TWMGtykyj6/interface-go-ipfs-core"

	"github.com/billziss-gh/cgofuse/fuse"
)

type RWLocker interface {
	sync.Locker
	RLock()
	RUnlock()
}

type FsFile interface {
	io.Reader
	io.Seeker
	io.Closer
	Size() (int64, error)
	Write(buff []byte, ofst int64) (int, error)
	Sync() int
	Truncate(size int64) (int, error)
}

type FsDirectory interface {
	//Parent() Directory
	//Entries(ofst int64) <-chan directoryEntry
	Entries() int
	Read(offset int64) <-chan DirectoryMessage
}

type fusePath interface {
	coreiface.Path
	RWLocker

	//TODO: reconsider approach
	Stat() *fuse.Stat_t
	Handles() *[]uint64
}

// FIXME: even if it's unlikely, we need to assure addr == invalidIndex is never true
func (fs *FUSEIPFS) newDirHandle(fsNode fusePath) (uint64, error) {
	//TODO: check path/node is actually a directory
	//return -fuse.ENOTDIR

	var (
		fsDir FsDirectory
		err   error
	)
	if canAsync(fsNode) {
		const timeout = 2 * time.Second                          //reset per entry in stream reader routine; TODO: configurable
		fsDir, err = fs.yieldAsyncDirIO(fs.ctx, timeout, fsNode) // Read inherits this context
	} else {
		fsDir, err = fs.yieldDirIO(fsNode)
	}

	if err != nil {
		return invalidIndex, fmt.Errorf("could not yield directory IO: %s", err)
	}

	hs := &dirHandle{record: fsNode, io: fsDir}
	fh := uint64(uintptr(unsafe.Pointer(hs)))
	*fsNode.Handles() = append(*fsNode.Handles(), fh)
	fs.dirHandles[fh] = hs
	return fh, nil
}

func (fs *FUSEIPFS) newFileHandle(fsNode fusePath) (uint64, error) {
	pIo, err := fs.yieldFileIO(fsNode)
	if err != nil {
		return invalidIndex, err
	}

	hs := &fileHandle{record: fsNode, io: pIo}

	fh := uint64(uintptr(unsafe.Pointer(hs)))
	*fsNode.Handles() = append(*fsNode.Handles(), fh)
	fs.fileHandles[fh] = hs
	return fh, nil
}

func (fs *FUSEIPFS) releaseFileHandle(fh uint64) (ret int) {
	if fh == invalidIndex {
		log.Errorf("releaseHandle - input handle is invalid")
		return -fuse.EBADF
	}

	defer func() {
		if r := recover(); r != nil {
			log.Errorf("releaseHandle recovered from panic, likely invalid handle: %v", r)
			ret = -fuse.EBADF
		}
	}()

	hs := (*fileHandle)(unsafe.Pointer(uintptr(fh)))

	handleGroup := hs.record.Handles()
	for i, cFh := range *handleGroup {
		if cFh == fh {
			*handleGroup = append((*handleGroup)[:i], (*handleGroup)[i+1:]...)

			if hs.io != nil {
				if err := hs.io.Close(); err != nil {
					log.Error(err)
				}
			}

			//Go runtime free-able
			fs.fileHandles[fh] = nil
			delete(fs.fileHandles, fh)
			ret = fuseSuccess
			return
		}
	}
	log.Errorf("releaseHandle - handle was detected as valid but was not associated with node %q", hs.record.String())
	ret = -fuse.EBADF
	return
}

func (fs *FUSEIPFS) releaseDirHandle(fh uint64) (ret int) {
	if fh == invalidIndex {
		log.Errorf("releaseDirHandle - input handle is invalid")
		ret = -fuse.EBADF
		return
	}

	defer func() {
		if r := recover(); r != nil {
			log.Errorf("releaseDirHandle recovered from panic, likely invalid handle: %v", r)
			ret = -fuse.EBADF
		}
	}()
	hs := (*dirHandle)(unsafe.Pointer(uintptr(fh)))

	handleGroup := hs.record.Handles()
	for i, cFh := range *handleGroup {
		if cFh == fh {
			*handleGroup = append((*handleGroup)[:i], (*handleGroup)[i+1:]...)

			//Go runtime free-able
			fs.dirHandles[fh] = nil
			delete(fs.dirHandles, fh)
			ret = fuseSuccess
			return
		}
	}
	log.Errorf("releaseDirHandle - handle was detected as valid but was not associated with node %q", hs.record.String())
	ret = -fuse.EBADF
	return
}

type AvailType = bool

const (
	aFiles       AvailType = false
	aDirectories AvailType = true
)

//NOTE: caller should retain FS Lock
func (fs *FUSEIPFS) AvailableHandles(directories AvailType) uint64 {
	if directories {
		return (invalidIndex - 1) - uint64(len(fs.dirHandles))
	}
	return (invalidIndex - 1) - uint64(len(fs.fileHandles))
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
