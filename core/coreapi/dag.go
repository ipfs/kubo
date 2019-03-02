package coreapi

import (
	"context"

	"github.com/ipfs/go-ipfs/pin"

	cid "github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
)

type dagAPI struct {
	ipld.DAGService

	core *CoreAPI
}

type pinningAdder CoreAPI

func (adder *pinningAdder) Add(ctx context.Context, nd ipld.Node) error {
	defer adder.blockstore.PinLock().Unlock()

	if err := adder.dag.Add(ctx, nd); err != nil {
		return err
	}

	adder.pinning.PinWithMode(nd.Cid(), pin.Recursive)

	return adder.pinning.Flush()
}

func (adder *pinningAdder) AddMany(ctx context.Context, nds []ipld.Node) error {
	defer adder.blockstore.PinLock().Unlock()

	if err := adder.dag.AddMany(ctx, nds); err != nil {
		return err
	}

	cids := cid.NewSet()

	for _, nd := range nds {
		c := nd.Cid()
		if cids.Visit(c) {
			adder.pinning.PinWithMode(c, pin.Recursive)
		}
	}

	return adder.pinning.Flush()
}

func (api *dagAPI) Pinning() ipld.NodeAdder {
	return (*pinningAdder)(api.core)
}
