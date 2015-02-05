package ipns

import (
	"os"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/bazil.org/fuse"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/bazil.org/fuse/fs"
)

type Link struct {
	Target string
}

func (l *Link) Attr() fuse.Attr {
	log.Debug("Link attr.")
	return fuse.Attr{
		Mode: os.ModeSymlink | 0555,
	}
}

func (l *Link) Readlink(req *fuse.ReadlinkRequest, intr fs.Intr) (string, fuse.Error) {
	log.Debugf("ReadLink: %s", l.Target)
	return l.Target, nil
}
