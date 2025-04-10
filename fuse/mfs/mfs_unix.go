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

	"bazil.org/fuse"
	"bazil.org/fuse/fs"

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
}

// Directory attributes.
func (dir *Dir) Attr(ctx context.Context, attr *fuse.Attr) error {
	attr.Mode = os.FileMode(os.ModeDir | 0755)
	attr.Size = 4096
	attr.Blocks = 8
	return nil
}

// Access files in a directory.
func (dir *Dir) Lookup(ctx context.Context, req *fuse.LookupRequest, resp *fuse.LookupResponse) (fs.Node, error) {
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

// List MFS directory (ls).
func (dir *Dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
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

// FUSE adapter for MFS files.
type File struct {
	mfsFile *mfs.File
}

// File attributes.
func (file *File) Attr(ctx context.Context, attr *fuse.Attr) error {
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
	return &FileHandler{
		mfsFD: fd,
	}, nil
}

// Wrapper for MFS's file descriptor that conforms to the FUSE fs.Handler
// interface.
type FileHandler struct {
	mfsFD mfs.FileDescriptor
	mu    sync.Mutex
}

// Read a opened MFS file.
func (fh *FileHandler) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	_, err := fh.mfsFD.Seek(req.Offset, io.SeekStart)
	if err != nil {
		return err
	}

	buf := make([]byte, req.Size)
	l, err := fh.mfsFD.Read(buf)

	resp.Data = buf[:l]

	switch err {
	case nil, io.EOF, io.ErrUnexpectedEOF:
		return nil
	default:
		return err
	}
}

// Create new filesystem.
func NewFileSystem(ipfs *core.IpfsNode) fs.FS {
	return &FileSystem{
		root: Dir{
			mfsDir: ipfs.FilesRoot.GetDirectory(),
		},
	}
}
