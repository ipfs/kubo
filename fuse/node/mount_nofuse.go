// Stub when built with "go build -tags nofuse". Excludes windows
// which never has FUSE support regardless of build tags.
//go:build !windows && nofuse

package node

import (
	"errors"

	core "github.com/ipfs/kubo/core"
)

func Mount(node *core.IpfsNode, fsdir, nsdir, mfsdir string) error {
	return errors.New("not compiled in")
}

func Unmount(node *core.IpfsNode) {
	return
}
