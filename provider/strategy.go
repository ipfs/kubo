package provider

// TODO: The strategy module is going to change so that it just
// calls Provide on a given provider instead of returning a channel.

import (
	"context"
	"github.com/ipfs/go-merkledag"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
)

func NewProvideAllStrategy(dag ipld.DAGService) Strategy {
	return func(ctx context.Context, root cid.Cid) <-chan cid.Cid {
		cids := make(chan cid.Cid)
		go func() {
			select {
			case <-ctx.Done():
				return
			case cids <- root:
			}
			merkledag.EnumerateChildren(ctx, merkledag.GetLinksWithDAG(dag), root, func(cid cid.Cid) bool {
				select {
				case <-ctx.Done():
					return false
				case cids <- root:
				}
				return true
			})
			close(cids)
		}()
		return cids
	}
}
