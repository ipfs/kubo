package coreunix

import (
	core "github.com/ipfs/go-ipfs/core"
	path "github.com/ipfs/go-ipfs/path"
	uio "github.com/ipfs/go-ipfs/unixfs/io"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

func Cat(ctx context.Context, n *core.IpfsNode, pstr string) (*uio.DagReader, error) {
	dagNode, err := core.Resolve(ctx, n, path.Path(pstr))
	if err != nil {
		return nil, err
	}
	return uio.NewDagReader(ctx, dagNode, n.DAG)
}
