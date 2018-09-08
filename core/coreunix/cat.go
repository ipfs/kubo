package coreunix

import (
	"context"

	core "github.com/ipfs/go-ipfs/core"
	path "gx/ipfs/QmNgXoHgXU1HzNb2HEZmRww9fDKE9NfDsvQwWLHiKHpvKM/go-path"
	resolver "gx/ipfs/QmNgXoHgXU1HzNb2HEZmRww9fDKE9NfDsvQwWLHiKHpvKM/go-path/resolver"
	uio "gx/ipfs/QmWAfTyD6KEBm7bzqNRBPvqKrZCDtn5PGbs9V1DKfnVK59/go-unixfs/io"
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
