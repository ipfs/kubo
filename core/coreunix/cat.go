package coreunix

import (
	"io"

	core "github.com/jbenet/go-ipfs/core"
	uio "github.com/jbenet/go-ipfs/unixfs/io"
	u "github.com/jbenet/go-ipfs/util"
)

func Cat(n *core.IpfsNode, k u.Key) (io.Reader, error) {
	dagNode, err := n.Resolve(k)
	if err != nil {
		return nil, err
	}
	return uio.NewDagReader(dagNode, n.DAG)
}
