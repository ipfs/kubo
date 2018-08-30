package coreunix

import (
	"context"

	core "github.com/ipfs/go-ipfs/core"
	path "gx/ipfs/QmTKaiDxQqVxmA1bRipSuP7hnTSgnMSmEa98NYeS6fcoiv/go-path"
	resolver "gx/ipfs/QmTKaiDxQqVxmA1bRipSuP7hnTSgnMSmEa98NYeS6fcoiv/go-path/resolver"
	uio "gx/ipfs/QmVNEJ5Vk1e2G5kHMiuVbpD6VQZiK1oS6aWZKjcUQW7hEy/go-unixfs/io"
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
