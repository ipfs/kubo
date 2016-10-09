package coreunix

import (
	"context"

	core "github.com/ipfs/go-ipfs/core"
	dag "github.com/ipfs/go-ipfs/merkledag"
	path "github.com/ipfs/go-ipfs/path"
	uio "github.com/ipfs/go-ipfs/unixfs/io"
)

func Cat(ctx context.Context, n *core.IpfsNode, pstr string) (*uio.DagReader, error) {
	dagNode, err := core.Resolve(ctx, n, path.Path(pstr))
	if err != nil {
		return nil, err
	}

	dnpb, ok := dagNode.(*dag.ProtoNode)
	if !ok {
		return nil, dag.ErrNotProtobuf
	}

	return uio.NewDagReader(ctx, dnpb, n.DAG)
}
