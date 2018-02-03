package merkledag

import (
	"context"

	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"
)

// ErrorService implements ipld.DAGService, returning 'Err' for every call.
type ErrorService struct {
	Err error
}

var _ ipld.DAGService = (*ErrorService)(nil)

func (cs *ErrorService) Add(ctx context.Context, nd ipld.Node) error {
	return cs.Err
}

func (cs *ErrorService) AddMany(ctx context.Context, nds []ipld.Node) error {
	return cs.Err
}

func (cs *ErrorService) Get(ctx context.Context, c *cid.Cid) (ipld.Node, error) {
	return nil, cs.Err
}

func (cs *ErrorService) GetMany(ctx context.Context, cids []*cid.Cid) <-chan *ipld.NodeOption {
	ch := make(chan *ipld.NodeOption)
	close(ch)
	return ch
}

func (cs *ErrorService) Remove(ctx context.Context, c *cid.Cid) error {
	return cs.Err
}

func (cs *ErrorService) RemoveMany(ctx context.Context, cids []*cid.Cid) error {
	return cs.Err
}
