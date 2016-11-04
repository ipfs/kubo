package io

import (
	"context"

	dag "github.com/ipfs/go-ipfs/merkledag"
	ft "github.com/ipfs/go-ipfs/unixfs"

	node "gx/ipfs/QmU7bFWQ793qmvNy7outdCaMfSDNk8uqhx4VNrxYj5fj5g/go-ipld-node"
)

func ResolveUnixfsOnce(ctx context.Context, ds dag.DAGService, nd node.Node, name string) (*node.Link, error) {
	pbnd, ok := nd.(*dag.ProtoNode)
	if !ok {
		lnk, _, err := nd.ResolveLink([]string{name})
		return lnk, err
	}

	upb, err := ft.FromBytes(pbnd.Data())
	if err != nil {
		// Not a unixfs node, use standard object traversal code
		lnk, _, err := nd.ResolveLink([]string{name})
		return lnk, err
	}

	switch upb.GetType() {
	/*
		case ft.THAMTShard:
			s, err := hamt.NewHamtFromDag(ds, nd)
			if err != nil {
				return nil, err
			}

			// TODO: optimized routine on HAMT for returning a dag.Link to avoid extra disk hits
			out, err := s.Find(ctx, name)
			if err != nil {
				return nil, err
			}

			return dag.MakeLink(out)
	*/
	default:
		lnk, _, err := nd.ResolveLink([]string{name})
		return lnk, err
	}
}
