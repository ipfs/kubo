//go:build (!nofuse && openbsd) || (!nofuse && netbsd) || (!nofuse && plan9)

package node

import (
	"errors"

	core "github.com/ipfs/kubo/core"
)

func Mount(node *core.IpfsNode, fsdir, nsdir, mfsdir string) error {
	return errors.New("FUSE not supported on OpenBSD or NetBSD. See #5334 (https://github.com/ipfs/kubo/issues/5334).")
}

func Unmount(node *core.IpfsNode) {
	return
}
