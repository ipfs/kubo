// +build !nofuse

package ipns

import (
	"context"
	"os"

	"gx/ipfs/QmexRhMPjQUc8LcMYfjZWVdqRDtCKUkW1YAFQbJ1fjfAxX/fuse"
	"gx/ipfs/QmexRhMPjQUc8LcMYfjZWVdqRDtCKUkW1YAFQbJ1fjfAxX/fuse/fs"
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
