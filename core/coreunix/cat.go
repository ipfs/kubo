package coreunix

import (
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	core "github.com/ipfs/go-ipfs/core"
	path "github.com/ipfs/go-ipfs/path"
	uio "github.com/ipfs/go-ipfs/unixfs/io"
)

func Cat(ctx context.Context, n *core.IpfsNode, pstr string) (*uio.DagReader, error) {
	p := path.FromString(pstr)
	dagNode, err := n.Resolver.ResolvePath(ctx, p)
	if err != nil {
		return nil, err
	}
	return uio.NewDagReader(ctx, dagNode, n.DAG)
}
