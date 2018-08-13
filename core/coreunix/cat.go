package coreunix

import (
	"context"

	core "github.com/ipfs/go-ipfs/core"
	path "gx/ipfs/QmV1W98rBAovVJGkeYHfqJ19JdT9dQbbWsCq9zPaMyrxYx/go-path"
	resolver "gx/ipfs/QmV1W98rBAovVJGkeYHfqJ19JdT9dQbbWsCq9zPaMyrxYx/go-path/resolver"
	uio "gx/ipfs/QmagwbbPqiN1oa3SDMZvpTFE5tNuegF1ULtuJvA9EVzsJv/go-unixfs/io"
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
