// +build !nofuse

package ipns

import (
	"os"

	"context"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/bazil.org/fuse"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/bazil.org/fuse/fs"
)

type Link struct {
	Target string
}

func (l *Link) Attr(ctx context.Context, a *fuse.Attr) error {
	log.Debug("Link attr.")
	a.Mode = os.ModeSymlink | 0555
	return nil
}

func (l *Link) Readlink(ctx context.Context, req *fuse.ReadlinkRequest) (string, error) {
	log.Debugf("ReadLink: %s", l.Target)
	return l.Target, nil
}

var _ fs.NodeReadlinker = (*Link)(nil)
