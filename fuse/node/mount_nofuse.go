//go:build !windows && nofuse
// +build !windows,nofuse

package node

import (
	"errors"

	core "github.com/ipfs/go-ipfs/core"
)

func Mount(node *core.IpfsNode, fsdir, nsdir string) error {
	return errors.New("not compiled in")
}
