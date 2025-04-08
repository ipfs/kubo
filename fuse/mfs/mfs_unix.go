//go:build (linux || darwin || freebsd || netbsd || openbsd) && !nofuse
// +build linux darwin freebsd netbsd openbsd
// +build !nofuse

package mfs

import (
	"context"
	"os"
	"strings"
	"syscall"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"

	"github.com/ipfs/boxo/mfs"
	"github.com/ipfs/kubo/core"
	"github.com/spaolacci/murmur3"
)

// FUSE filesystem mounted at /mfs.
type FileSystem struct {
	root Dir
}

// Get filesystem root.
func (fs *FileSystem) Root() (fs.Node, error) {
	return &fs.root, nil
}

// Inode numbers generated with murmur3 of the file path.
func GetInode(path string) uint64 {
	return uint64(murmur3.Sum32([]byte(path)))
}

// FUSE Adapter for MFS directories.
type Dir struct {
	mfsDir *mfs.Directory
}

// Directory attributes.
func (dir *Dir) Attr(ctx context.Context, attr *fuse.Attr) error {
	attr.Mode = os.FileMode(os.ModeDir | 0755)
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
		mfsDir := mfsNode.(*mfs.Directory)
		result := Dir{
			mfsDir: mfsDir,
		}
		return &result, nil
	case mfs.TFile:
		mfsFile := mfsNode.(*mfs.File)
		result := File{
			mfsFile: mfsFile,
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
			Inode: GetInode(strings.Join([]string{dir.mfsDir.Path(), node.Name}, "/")),
			Type:  fuse.DT_File,
			Name:  node.Name,
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

// Create new filesystem.
func NewFileSystem(ipfs *core.IpfsNode) fs.FS {
	return &FileSystem{
		root: Dir{
			mfsDir: ipfs.FilesRoot.GetDirectory(),
		},
	}
}
