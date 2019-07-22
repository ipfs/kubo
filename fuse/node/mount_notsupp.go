// +build !nofuse,openbsd !nofuse,netbsd

package node

import (
	"errors"

	core "github.com/ipfs/go-ipfs/core"
)

func Mount(node *core.IpfsNode, fsdir, nsdir string) error {
	return errors.New("FUSE not supported on OpenBSD or NetBSD. See #5334 (https://git.io/fjMuC).")
}
