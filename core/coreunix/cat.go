package coreunix

import (
	"context"

	core "github.com/ipfs/go-ipfs/core"
	path "gx/ipfs/QmPqCBrmkm7jNfYi7xFS7mUZsrN6DEumBMrxLnL7axNJx1/go-path"
	resolver "gx/ipfs/QmPqCBrmkm7jNfYi7xFS7mUZsrN6DEumBMrxLnL7axNJx1/go-path/resolver"
	uio "gx/ipfs/QmWJRM6rLjXGEXb5JkKu17Y68eJtCFcKPyRhb8JH2ELZ2Q/go-unixfs/io"
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
