//go:build !nofuse && !openbsd && !netbsd && !plan9

package ipns

import (
	"context"
	"os"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)

type Link struct {
	Target string
}

func (l *Link) Attr(ctx context.Context, a *fuse.Attr) error {
	log.Debug("Link attr.")
	// TODO: wire TTL from IPNS record (capped at Ipns.MaxCacheTTL) here instead of 0
	a.Valid = 0
	a.Mode = os.ModeSymlink | 0o555
	a.Uid = uint32(os.Getuid())
	a.Gid = uint32(os.Getgid())
	return nil
}

func (l *Link) Readlink(ctx context.Context, req *fuse.ReadlinkRequest) (string, error) {
	log.Debugf("ReadLink: %s", l.Target)
	return l.Target, nil
}

var _ fs.NodeReadlinker = (*Link)(nil)
