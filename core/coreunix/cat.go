package coreunix

import (
	"context"

	core "github.com/ipfs/go-ipfs/core"
	path "gx/ipfs/QmTG5WFmAM4uAnqGskeAPijdpTmmNDLJNCQ71NqfdvC6hV/go-path"
	resolver "gx/ipfs/QmTG5WFmAM4uAnqGskeAPijdpTmmNDLJNCQ71NqfdvC6hV/go-path/resolver"
	uio "gx/ipfs/QmTbas51oodp3ZJrqsWYs1yqSxcD7LEJBv4djRV2VrY8wv/go-unixfs/io"
)

func Cat(ctx context.Context, n *core.IpfsNode, pstr string) (uio.DagReader, error) {
	r := &resolver.Resolver{
		DAG:         n.DAG,
		ResolveOnce: uio.ResolveUnixfsOnce,
	}

	dagNode, err := core.Resolve(ctx, n.Namesys, r, path.Path(pstr))
	if err != nil {
		return nil, err
	}

	return uio.NewDagReader(ctx, dagNode, n.DAG)
}
