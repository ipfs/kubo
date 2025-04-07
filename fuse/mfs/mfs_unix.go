//go:build (linux || darwin || freebsd || netbsd || openbsd) && !nofuse
// +build linux darwin freebsd netbsd openbsd
// +build !nofuse

package mfs

import (
	"context"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"

	"github.com/ipfs/kubo/core"
)

type FileSystem struct {
	root Root
}

func (fs FileSystem) Root() (fs.Node, error) {
	return fs.root, nil
}

type Root struct{}

func (root Root) Attr(ctx context.Context, attr *fuse.Attr) error {
	return nil
}

func NewFileSystem(*core.IpfsNode) fs.FS {
	return FileSystem{
		root: Root{},
	}
}
