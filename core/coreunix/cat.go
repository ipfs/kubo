package coreunix

import (
	"context"

	core "github.com/ipfs/go-ipfs/core"
	uio "gx/ipfs/QmWdTRLi3H7ZJQ8s7NYo8oitz5JHEEPKLn1QPMsJVWg2Ew/go-unixfs/io"
	path "gx/ipfs/Qme34dT9spiPgunbueNtziRX4SvfLHDFZQvmTBVK8p4qNT/go-path"
	resolver "gx/ipfs/Qme34dT9spiPgunbueNtziRX4SvfLHDFZQvmTBVK8p4qNT/go-path/resolver"
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
