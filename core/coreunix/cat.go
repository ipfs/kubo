package coreunix

import (
	"context"

	core "github.com/ipfs/go-ipfs/core"
	uio "gx/ipfs/QmVxjT67BU1QZUPzSLNZT6DkDzVNfPfkzqNyJYFXxSH2hA/go-unixfs/io"
	path "gx/ipfs/QmY2QaawxgJw2rn7WsFNkWEYph3z2azpyYdrhAc1JctDmE/go-path"
	resolver "gx/ipfs/QmY2QaawxgJw2rn7WsFNkWEYph3z2azpyYdrhAc1JctDmE/go-path/resolver"
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
