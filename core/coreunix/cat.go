package coreunix

import (
	"io"

	core "github.com/jbenet/go-ipfs/core"
	path "github.com/jbenet/go-ipfs/path"
	uio "github.com/jbenet/go-ipfs/unixfs/io"
)

func Cat(n *core.IpfsNode, pstr string) (io.Reader, error) {
	p := path.FromString(pstr)
	dagNode, err := n.Resolver.ResolvePath(p)
	if err != nil {
		return nil, err
	}
	return uio.NewDagReader(n.ContextGroup.Context(), dagNode, n.DAG)
}
