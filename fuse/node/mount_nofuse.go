// +build !windows,nofuse

package node

import (
	"errors"

	core "github.com/ipfs/go-ipfs/core"
)

type errNeedFuseVersion error // used in tests, needed in OSX

func Mount(node *core.IpfsNode, fsdir, nsdir string) error {
	return errors.New("not compiled in")
}
