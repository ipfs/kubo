package fusemount

import (
	"context"
	"errors"
	"fmt"
	"io"
	gopath "path"

	coreiface "gx/ipfs/QmXLwxifxwfc2bAwq6rdjbYqAsGzWsDE9RM5TWMGtykyj6/interface-go-ipfs-core"
	coreoptions "gx/ipfs/QmXLwxifxwfc2bAwq6rdjbYqAsGzWsDE9RM5TWMGtykyj6/interface-go-ipfs-core/options"

	"github.com/billziss-gh/cgofuse/fuse"

	files "gx/ipfs/QmQmhotPUzVrMEWNK3x1R5jQ5ZHWyL7tVUrmRPjrBrvyCb/go-ipfs-files"
	chunk "gx/ipfs/QmYmZ81dU5nnmBFy5MmktXLZpt8QCWhRJd6M1uxVF6vke8/go-ipfs-chunker"
	mfs "gx/ipfs/Qmb74fRYPgpjYzoBV7PAVNmP3DQaRrh8dHdKE4PwnF3cRx/go-mfs"
	"gx/ipfs/QmcYUTQ7tBZeH1CLsZM2S3xhMEZdvUgXvbjhpMsLDpk3oJ/go-unixfs/mod"
)

//TODO: set atime on success; use defer to check return
func (fs *FUSEIPFS) Read(path string, buff []byte, ofst int64, fh uint64) int {
	fs.RLock()
	log.Debugf("Read - Request [%X]{+%d}%q", fh, ofst, path)

	if ofst < 0 {
		log.Errorf("Read - Invalid offset {%d}[%X]%q", ofst, fh, path)
		fs.RUnlock()
		return -fuse.EINVAL
	}

	//TODO: [everywhere] change handle lookups to be like this; just to reduce
	/*
		fh, err := LookupFileHandle(fh)
		if err { ... }
		fh.record.Lock()
		...
		requiresFusePath(fh.record)
		requiresIo(fh.io)
		...
		fh.Record.Unlock()
	*/

	h, err := fs.LookupFileHandle(fh)
	if err != nil {
		fs.RUnlock()
		log.Errorf("Read - [%X]%q: %s", fh, path, err)
		if err == errInvalidHandle {
			return -fuse.EBADF
		}
		return -fuse.EIO
	}

	h.record.Lock() // write lock; handle cursor is modified
	fs.RUnlock()
	defer h.record.Unlock()

	//TODO: inspect need to flush here
	//if fh != handle.lastCaller { flush }

	if fileBound, err := h.io.Size(); err == nil {
		if ofst >= fileBound {
			return 0 // this is unique from fuseSuccess
		}
	}

	if ofst != 0 {
		_, err = h.io.Seek(ofst, io.SeekStart)
		if err != nil {
			log.Errorf("Read - seek error: %s", err)
			return -fuse.EIO
		}
	}

	readBytes, err := h.io.Read(buff)
	if err != nil && err != io.EOF {
		log.Errorf("Read - error: %s", err)
	}
	return readBytes
}

func (fs *FUSEIPFS) yieldFileIO(fsNode fusePath) (FsFile, error) {
	//TODO: cast to concrete type and change yield parameters to accept them
	switch fsNode.(type) {
	case *mfsNode:
		return mfsYieldFileIO(fs.filesRoot, fsNode.String()[frs:])
	case *ipfsNode:
		return fs.coreYieldFileIO(fsNode)
	case *ipnsKey:
		return fs.keyYieldFileIO(fsNode)
	case *ipnsNode:
		return fs.nameYieldFileIO(fsNode)
	default:
		return nil, fmt.Errorf("unexpected IO request {%T}%q", fsNode, fsNode.String())
	}
}

func mfsYieldFileIO(filesRoot *mfs.Root, path string) (FsFile, error) {
	mfsNode, err := mfs.Lookup(filesRoot, path)
	if err != nil {
		return nil, err
	}

	mfsFile, ok := mfsNode.(*mfs.File)
	if !ok {
		return nil, fmt.Errorf("File IO requested for non-file, type: %v %q", mfsNode.Type(), path)
	}

	//TODO: change arity of yield to accept i/o request flag
	return &mfsFileIo{ff: mfsFile, ioFlags: mfs.Flags{Read: true, Write: true, Sync: mfsSync}}, nil
}

//TODO: we'll have to pass and store write flags on this; for now rely on 🤠 to maintain permissions
//TODO: some kind of local write buffer
type mfsFileIo struct {
	ff *mfs.File
	//fd mfs.FileDescriptor

	//XXX: this is not ideal, we're duplicating state here to circumvent mfs's 1 (writable) file descriptor limit
	//this is likely suboptimal
	ioFlags mfs.Flags
	cursor  int64
}

//allows for multiple handles to a single mfs node
func (mio *mfsFileIo) mfsOpenShim() (mfs.FileDescriptor, error) {
	fd, err := mio.ff.Open(mio.ioFlags)
	if err != nil {
		return nil, err
	}
	if mio.cursor != 0 {
		mio.cursor, err = fd.Seek(mio.cursor, io.SeekStart)
		if err != nil {
			return nil, err
		}
	}
	return fd, nil
}

func (mio *mfsFileIo) Size() (int64, error) {
	fd, err := mio.ff.Open(mio.ioFlags)
	if err != nil {
		log.Errorf("mio Size I/O sunk %X:%s", fd, err)
		return int64(-fuse.EIO), err
	}

	defer fd.Close()
	return fd.Size()
}

func (mio *mfsFileIo) Close() error {
	return nil
	//return mio.fd.Close()
}

func (mio *mfsFileIo) Seek(offset int64, whence int) (int64, error) {
	if whence > io.SeekEnd {
		return int64(-fuse.EINVAL), errors.New("invalid whence value")
	}

	if offset == 0 {
		return mio.cursor, nil
	}

	switch whence {
	case io.SeekStart:
		mio.cursor = offset
	case io.SeekCurrent:
		mio.cursor += offset
	case io.SeekEnd:
		if offset > 0 {
			return int64(-fuse.EINVAL), errors.New("invalid offset value")
		}
		s, err := mio.Size() //TODO: avoid re-opening fd in Size() if we can
		if err != nil {
			log.Errorf("mio Seek I/O sunk: %s", err)
			return int64(-fuse.EIO), err
		}
		mio.cursor = s + offset
	}

	return fuseSuccess, nil
}

func (mio *mfsFileIo) Read(buff []byte) (int, error) {
	fd, err := mio.mfsOpenShim()
	if err != nil {
		log.Errorf("mio Read I/O sunk %X:%s", fd, err)
		return -fuse.EIO, err
	}
	defer fd.Close()

	readBytes, err := fd.Read(buff)
	if readBytes >= 1 {
		mio.cursor += int64(readBytes)
	}
	if err != nil {
		if err == io.EOF {
			return readBytes, err
		}

		log.Errorf("mio Read I/O sunk %X:%s", fd, err)
		return -fuse.EIO, err
	}
	return readBytes, nil
}

//TODO: look into this; speak with shcomatis
// API syncs on close by default; see mfsOpenShim(); every op should force a sync as a result of that
// ideally we want to only sync on demand
func (mio *mfsFileIo) Sync() int {
	/*
		if err := mio.fd.Flush(); err != nil {
			return -fuse.EIO
		}
	*/
	return fuseSuccess
}

func (mio *mfsFileIo) Write(buff []byte, ofst int64) (int, error) {
	var (
		written int
		err     error
	)

	fd, err := mio.mfsOpenShim()
	if err != nil {
		log.Errorf("mio Write I/O sunk %X:%s", fd, err)
		return -fuse.EIO, err
	}
	defer fd.Close()

	if ofst == 0 && mio.cursor == 0 {
		written, err = fd.Write(buff)
	} else {
		written, err = fd.WriteAt(buff, ofst)
	}
	if err != nil {
		log.Errorf("mio Write I/O sunk %X:%s", fd, err)
		return -fuse.EIO, err
	}
	mio.cursor += int64(written)

	return written, nil
}

func (mio *mfsFileIo) Truncate(size int64) (int, error) {
	fd, err := mio.mfsOpenShim()
	if err != nil {
		log.Errorf("mio Truncate I/O sunk %X:%s", fd, err)
		return -fuse.EIO, err
	}
	defer fd.Close()

	err = fd.Truncate(size)
	if err != nil {
		return -fuse.EIO, err
	}
	return fuseSuccess, nil
}

func (fs *FUSEIPFS) coreYieldFileIO(curNode coreiface.Path) (FsFile, error) {
	var err error
	apiNode, err := fs.core.Unixfs().Get(fs.ctx, curNode)
	if err != nil {
		return nil, err
	}

	fIo, ok := apiNode.(files.File)
	if !ok {
		return nil, fmt.Errorf("%q is not a file", curNode.String())
	}

	return &corePIo{fd: fIo}, nil
}

type corePIo struct {
	fd files.File
}

//FIXME read broken on large files sometimes?
//MFS too
func (cio *corePIo) Read(buff []byte) (int, error) {
	readBytes, err := cio.fd.Read(buff)
	if err != nil {
		if err == io.EOF {
			return readBytes, err
		}
		log.Errorf("cio Read I/O sunk %s", err)
		return -fuse.EIO, err
	}
	return readBytes, nil
}

func (cio *corePIo) Close() error {
	return cio.fd.Close()
}

func (cio *corePIo) Seek(offset int64, whence int) (int64, error) {
	return cio.fd.Seek(offset, whence)
}

func (cio *corePIo) Size() (int64, error) {
	return cio.fd.Size()
}

func (cio *corePIo) Write(buff []byte, ofst int64) (int, error) {
	return -fuse.EROFS, fmt.Errorf("Write requested on read only path")
}

func (cio *corePIo) Sync() int {
	log.Warning("Sync called on read only file")
	return -fuse.EINVAL
}

func (cio *corePIo) Truncate(int64) (int, error) {
	return -fuse.EROFS, errReadOnly
}

//TODO: [fs] free MFS roots when no references are using them instead of loading them all forever
// instantiate on demand
func (fs *FUSEIPFS) nameYieldFileIO(fsNode fusePath) (FsFile, error) {
	keyRoot, subPath, err := fs.ipnsMFSSplit(fsNode.String())
	if err != nil {
		globalNode, err := fs.resolveToGlobal(fsNode)
		if err != nil {
			return nil, err
		}
		return fs.coreYieldFileIO(globalNode)
	}
	return mfsYieldFileIO(keyRoot, subPath)
}

type keyFileIo struct {
	key  coreiface.Key
	name coreiface.NameAPI
	mod  *mod.DagModifier
}

func (fs *FUSEIPFS) keyYieldFileIO(fsNode fusePath) (FsFile, error) {
	_, keyName := gopath.Split(fsNode.String())

	oAPI, err := fs.core.WithOptions(coreoptions.Api.Offline(true))
	if err != nil {
		return nil, err
	}

	coreKey, err := resolveKeyName(fs.ctx, oAPI.Key(), keyName)
	if err != nil {
		return nil, err
	}

	ipldNode, err := oAPI.ResolveNode(fs.ctx, coreKey.Path())
	if err != nil {
		return nil, err
	}

	dmod, err := mod.NewDagModifier(fs.ctx, ipldNode, oAPI.Dag(), chunk.DefaultSplitter)
	if err != nil {
		return nil, err
	}

	return &keyFileIo{key: coreKey, name: oAPI.Name(), mod: dmod}, nil
}

func (kio *keyFileIo) Write(buff []byte, ofst int64) (int, error) {
	var (
		written int
		err     error
	)

	if ofst == 0 {
		written, err = kio.mod.Write(buff)
	} else {
		written, err = kio.mod.WriteAt(buff, ofst)
	}
	if err != nil {
		return -fuse.EIO, err
	}

	//TODO: [investigate] core.ResolveNode deadlocks if we write and publish this node, but don't commit it to the dag service
	/*
		if err = kio.mod.Sync(); err != nil {
			return -fuse.EIO, err
		}
	*/

	nd, err := kio.mod.GetNode()
	if err != nil {
		return -fuse.EIO, err
	}

	corePath, err := coreiface.ParsePath(nd.String())
	if err != nil {
		return -fuse.EIO, err
	}

	_, err = kio.name.Publish(context.TODO(), corePath, coreoptions.Name.Key(kio.key.Name()), coreoptions.Name.AllowOffline(true))
	if err != nil {
		return -fuse.EIO, err
	}

	return written, nil
}

func (kio *keyFileIo) Read(buff []byte) (int, error) {
	readBytes, err := kio.mod.Read(buff)
	if err != nil {
		if err == io.EOF {
			return readBytes, err
		}
		log.Errorf("kio Read I/O sunk %s", err)
		return -fuse.EIO, err
	}
	return readBytes, nil
}

func (*keyFileIo) Close() error {
	return nil
}

func (kio *keyFileIo) Seek(offset int64, whence int) (int64, error) {
	return kio.mod.Seek(offset, whence)
}

func (kio *keyFileIo) Size() (int64, error) {
	return kio.mod.Size()
}

func (kio *keyFileIo) Sync() int {
	if err := kio.mod.Sync(); err != nil {
		return -fuse.EIO
	}
	return fuseSuccess
}

func (kio *keyFileIo) Truncate(size int64) (int, error) {
	if err := kio.mod.Truncate(size); err != nil {
		return -fuse.EIO, err
	}
	return fuseSuccess, nil
}
