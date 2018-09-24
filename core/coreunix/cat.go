package coreunix

import (
	"context"

	core "github.com/ipfs/go-ipfs/core"
	uio "gx/ipfs/QmWqiuwk7ZzUFQvfBuQDwxPxyAQtNMxGYwZkjJuF6GgWQk/go-unixfs/io"
	path "gx/ipfs/Qmc17MNY1xUgiE2nopbi6KATWau9qcGZtdmKKuXvFMVUgc/go-path"
	resolver "gx/ipfs/Qmc17MNY1xUgiE2nopbi6KATWau9qcGZtdmKKuXvFMVUgc/go-path/resolver"
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
