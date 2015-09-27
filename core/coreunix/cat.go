package coreunix

import (
	core "github.com/ipfs/go-ipfs/core"
	path "github.com/ipfs/go-ipfs/path"
	uio "github.com/ipfs/go-ipfs/unixfs/io"
)

func Cat(n *core.IpfsNode, pstr string) (*uio.DagReader, error) {
	p := path.FromString(pstr)
	dagNode, err := n.Resolver.ResolvePath(n.Context(), p)
	if err != nil {
		return nil, err
	}
	return uio.NewDagReader(n.Context(), dagNode, n.DAG)
}
