package node

import (
	"github.com/ipfs/kubo/core"
)

func Mount(node *core.IpfsNode, fsdir, nsdir, mfsdir string) error {
	// TODO
	// currently a no-op, but we don't want to return an error
	return nil
}

func Unmount(node *core.IpfsNode) {
	// TODO
	// currently a no-op
	return
}
