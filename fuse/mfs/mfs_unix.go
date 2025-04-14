//go:build (linux || darwin || freebsd || netbsd || openbsd) && !nofuse
// +build linux darwin freebsd netbsd openbsd
// +build !nofuse

package mfs

import (
	"context"
	"io"
	"os"
	"sync"
	"syscall"
	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"

	dag "github.com/ipfs/boxo/ipld/merkledag"
	ft "github.com/ipfs/boxo/ipld/unixfs"
	"github.com/ipfs/boxo/mfs"
	"github.com/ipfs/kubo/core"
)

// FUSE filesystem mounted at /mfs.
type FileSystem struct {
	root Dir
}

// Get filesystem root.
func (fs *FileSystem) Root() (fs.Node, error) {
	return &fs.root, nil
}

// FUSE Adapter for MFS directories.
type Dir struct {
	mfsDir *mfs.Directory
	mu     sync.RWMutex
}

// Directory attributes (stat).
func (dir *Dir) Attr(ctx context.Context, attr *fuse.Attr) error {
	attr.Mode = os.FileMode(os.ModeDir | 0755)
	attr.Size = 4096
	attr.Blocks = 8
	return nil
}

// Access files in a directory.
func (dir *Dir) Lookup(ctx context.Context, req *fuse.LookupRequest, resp *fuse.LookupResponse) (fs.Node, error) {
	dir.mu.RLock()
	defer dir.mu.RUnlock()

	mfsNode, err := dir.mfsDir.Child(req.Name)
	if err != nil {
		return nil, syscall.Errno(syscall.ENOENT)
	}
	switch mfsNode.Type() {
	case mfs.TDir:
		result := Dir{
			mfsDir: mfsNode.(*mfs.Directory),
		}
		return &result, nil
	case mfs.TFile:
		result := File{
			mfsFile: mfsNode.(*mfs.File),
		}
		return &result, nil
	}

	return nil, syscall.Errno(syscall.ENOENT)
}

// List (ls) MFS directory.
func (dir *Dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	dir.mu.RLock()
	defer dir.mu.RUnlock()

	var res []fuse.Dirent
	nodes, err := dir.mfsDir.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, node := range nodes {
		res = append(res, fuse.Dirent{
			Type: fuse.DT_File,
			Name: node.Name,
		})
	}
	return res, nil
}

// Mkdir (mkdir) in MFS.
func (dir *Dir) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	dir.mu.Lock()
	defer dir.mu.Unlock()

	mfsDir, err := dir.mfsDir.Mkdir(req.Name)
	if err != nil {
		return nil, err
	}
	return &Dir{
		mfsDir: mfsDir,
	}, nil
}

// Remove (rm/rmdir) an MFS file.
func (dir *Dir) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	dir.mu.Lock()
	defer dir.mu.Unlock()

	// Check for empty directory.
	if req.Dir {
		targetNode, err := dir.mfsDir.Child(req.Name)
		if err != nil {
			return err
		}
		target := targetNode.(*mfs.Directory)

		children, err := target.ListNames(ctx)
		if err != nil {
			return err
		}
		if len(children) > 0 {
			return os.ErrExist
		}
	}
	err := dir.mfsDir.Unlink(req.Name)
	if err != nil {
		return err
	}
	return dir.mfsDir.Flush()
}

// Move (mv) an MFS file.
func (dir *Dir) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) error {
	dir.mu.Lock()
	defer dir.mu.Unlock()

	file, err := dir.mfsDir.Child(req.OldName)
	if err != nil {
		return err
	}
	node, err := file.GetNode()
	if err != nil {
		return err
	}
	targetDir := newDir.(*Dir)

	err = targetDir.mfsDir.AddChild(req.NewName, node)
	if err != nil {
		return err
	}

	return dir.mfsDir.Unlink(req.OldName)
}

// Create (touch) an MFS file.
func (dir *Dir) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	dir.mu.Lock()
	defer dir.mu.Unlock()

	node := dag.NodeWithData(ft.FilePBData(nil, 0))
	if err := node.SetCidBuilder(dir.mfsDir.GetCidBuilder()); err != nil {
		return nil, nil, err
	}

	if err := dir.mfsDir.AddChild(req.Name, node); err != nil {
		return nil, nil, err
	}

	if err := dir.mfsDir.Flush(); err != nil {
		return nil, nil, err
	}

	mfsNode, err := dir.mfsDir.Child(req.Name)
	if err != nil {
		return nil, nil, err
	}
	mfsNode.SetModTime(time.Now())

	mfsFile := mfsNode.(*mfs.File)

	file := File{
		mfsFile: mfsFile,
	}

	// Read access flags and create a handler.
	accessMode := req.Flags & fuse.OpenAccessModeMask
	flags := mfs.Flags{
		Read:  accessMode == fuse.OpenReadOnly || accessMode == fuse.OpenReadWrite,
		Write: accessMode == fuse.OpenWriteOnly || accessMode == fuse.OpenReadWrite,
		Sync:  req.Flags|fuse.OpenSync > 0,
	}

	fd, err := mfsFile.Open(flags)
	if err != nil {
		return nil, nil, err
	}
	handler := FileHandler{
		mfsFD: fd,
	}

	return &file, &handler, nil
}

// FUSE adapter for MFS files.
type File struct {
	mfsFile *mfs.File
	mu      sync.RWMutex
}

// File attributes.
func (file *File) Attr(ctx context.Context, attr *fuse.Attr) error {
	file.mu.RLock()
	defer file.mu.RUnlock()

	size, err := file.mfsFile.Size()
	if err != nil {
		return err
	}
	attr.Size = uint64(size)
	if size%512 == 0 {
		attr.Blocks = uint64(size / 512)
	} else {
		attr.Blocks = uint64(size/512 + 1)
	}

	mtime, err := file.mfsFile.ModTime()
	if err != nil {
		return err
	}
	attr.Mtime = mtime

	mode, err := file.mfsFile.Mode()
	if err != nil {
		return err
	}
	attr.Mode = mode
	return nil
}

// Open an MFS file.
func (file *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	file.mu.Lock()
	defer file.mu.Unlock()

	accessMode := req.Flags & fuse.OpenAccessModeMask
	flags := mfs.Flags{
		Read:  accessMode == fuse.OpenReadOnly || accessMode == fuse.OpenReadWrite,
		Write: accessMode == fuse.OpenWriteOnly || accessMode == fuse.OpenReadWrite,
		Sync:  req.Flags|fuse.OpenSync > 0,
	}
	fd, err := file.mfsFile.Open(flags)
	if err != nil {
		return nil, err
	}

	if flags.Write {
		err := file.mfsFile.SetModTime(time.Now())
		if err != nil {
			return nil, err
		}
	}

	return &FileHandler{
		mfsFD: fd,
	}, nil
}

// Sync the file's contents to MFS.
func (file *File) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	file.mu.Lock()
	defer file.mu.Unlock()

	return file.mfsFile.Sync()
}

// Wrapper for MFS's file descriptor that conforms to the FUSE fs.Handler
// interface.
type FileHandler struct {
	mfsFD mfs.FileDescriptor
	mu    sync.Mutex
}

// Read a opened MFS file.
func (fh *FileHandler) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	buf := make([]byte, req.Size)

	l, err := func() (int, error) {
		fh.mu.Lock()
		defer fh.mu.Unlock()

		_, err := fh.mfsFD.Seek(req.Offset, io.SeekStart)
		if err != nil {
			return 0, err
		}
		return fh.mfsFD.Read(buf)
	}()

	resp.Data = buf[:l]

	switch err {
	case nil, io.EOF, io.ErrUnexpectedEOF:
		return nil
	default:
		return err
	}
}

// Write writes to an opened MFS file.
func (fh *FileHandler) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	l, err := fh.mfsFD.WriteAt(req.Data, req.Offset)
	if err != nil {
		return err
	}
	resp.Size = l

	return nil
}

// Flushes the file's buffer.
func (fh *FileHandler) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	return fh.mfsFD.Flush()
}

// Closes the file.
func (fh *FileHandler) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	return fh.mfsFD.Close()
}

// Create new filesystem.
func NewFileSystem(ipfs *core.IpfsNode) fs.FS {
	return &FileSystem{
		root: Dir{
			mfsDir: ipfs.FilesRoot.GetDirectory(),
		},
	}
}

// Check that our structs implement all the interfaces we want.
type mfDir interface {
	fs.Node
	fs.HandleReadDirAller
	fs.NodeRequestLookuper
	fs.NodeMkdirer
	fs.NodeRenamer
	fs.NodeRemover
	fs.NodeCreater
}

var _ mfDir = (*Dir)(nil)

type mfFile interface {
	fs.Node
	fs.NodeOpener
	fs.NodeFsyncer
}

var _ mfFile = (*File)(nil)

type mfHandler interface {
	fs.Handle
	fs.HandleReader
	fs.HandleWriter
	fs.HandleFlusher
	fs.HandleReleaser
}

var _ mfHandler = (*FileHandler)(nil)
