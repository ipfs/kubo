//go:build (linux || darwin || freebsd) && !nofuse

package ipns

import (
	"context"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

type Link struct {
	fs.Inode
	Target string
}

func (l *Link) Getattr(_ context.Context, _ fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	log.Debug("Link attr.")
	out.Attr.Mode = 0o555
	return 0
}

func (l *Link) Readlink(_ context.Context) ([]byte, syscall.Errno) {
	log.Debugf("ReadLink: %s", l.Target)
	return []byte(l.Target), 0
}

var (
	_ fs.NodeGetattrer  = (*Link)(nil)
	_ fs.NodeReadlinker = (*Link)(nil)
)
