//go:build !windows && nofuse
// +build !windows,nofuse

package node

import (
	"errors"

	core "github.com/ipfs/kubo/core"
)

var errNotCompiled = errors.New("not compiled in")

func Mount(node *core.IpfsNode, fsdir, nsdir, mfsdir string) error {
	return errNotCompiled
}

func Unmount(node *core.IpfsNode) {
	return errNotCompiled
}
