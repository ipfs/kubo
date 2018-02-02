package merkledag

import (
	"context"

	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"
)

// ComboService implements ipld.DAGService, using 'Read' for all fetch methods,
// and 'Write' for all methods that add new objects.
type ComboService struct {
	Read  ipld.NodeGetter
	Write ipld.DAGService
}

var _ ipld.DAGService = (*ComboService)(nil)

func (cs *ComboService) Add(ctx context.Context, nd ipld.Node) error {
	return cs.Write.Add(ctx, nd)
}

func (cs *ComboService) AddMany(ctx context.Context, nds []ipld.Node) error {
	return cs.Write.AddMany(ctx, nds)
}

func (cs *ComboService) Get(ctx context.Context, c *cid.Cid) (ipld.Node, error) {
	return cs.Read.Get(ctx, c)
}

func (cs *ComboService) GetMany(ctx context.Context, cids []*cid.Cid) <-chan *ipld.NodeOption {
	return cs.Read.GetMany(ctx, cids)
}

func (cs *ComboService) Remove(ctx context.Context, c *cid.Cid) error {
	return cs.Write.Remove(ctx, c)
}

func (cs *ComboService) RemoveMany(ctx context.Context, cids []*cid.Cid) error {
	return cs.Write.RemoveMany(ctx, cids)
}
