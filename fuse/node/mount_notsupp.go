// Stub for platforms where go-fuse does not compile but the user
// has not set the nofuse build tag. Returns a clear error instead
// of a build failure. See https://github.com/ipfs/kubo/issues/5334.
//go:build (openbsd || netbsd || plan9) && !nofuse

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
