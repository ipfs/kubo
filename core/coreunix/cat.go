package coreunix

import (
	"io"

	core "github.com/jbenet/go-ipfs/core"
	path "github.com/jbenet/go-ipfs/path"
	uio "github.com/jbenet/go-ipfs/unixfs/io"
)

func Cat(n *core.IpfsNode, p path.Path) (io.Reader, error) {
	dagNode, err := n.Resolver.ResolvePath(p)
	if err != nil {
		return nil, err
	}
	return uio.NewDagReader(n.ContextGroup.Context(), dagNode, n.DAG)
}
