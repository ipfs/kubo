//go:build !nofuse && !openbsd && !netbsd && !plan9

package ipns

import (
	"context"
	"os"

	"github.com/seaweedfs/fuse"
	"github.com/seaweedfs/fuse/fs"
)

type Link struct {
	Target string
}

func (l *Link) Attr(ctx context.Context, a *fuse.Attr) error {
	log.Debug("Link attr.")
	a.Mode = os.ModeSymlink | 0o555
	return nil
}

func (l *Link) Readlink(ctx context.Context, req *fuse.ReadlinkRequest) (string, error) {
	log.Debugf("ReadLink: %s", l.Target)
	return l.Target, nil
}

var _ fs.NodeReadlinker = (*Link)(nil)
