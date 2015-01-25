package coreunix

import (
	"io"

	core "github.com/jbenet/go-ipfs/core"
	uio "github.com/jbenet/go-ipfs/unixfs/io"
)

func Cat(n *core.IpfsNode, path string) (io.Reader, error) {
	dagNode, err := n.Resolver.ResolvePath(path)
	if err != nil {
		return nil, err
	}
	return uio.NewDagReader(n.ContextGroup.Context(), dagNode, n.DAG)
}
