package coreapi

import (
	"context"

	ipld "github.com/ipfs/go-ipld-format"
	dag "github.com/ipfs/go-merkledag"
)

type dagAPI struct {
	ipld.DAGService

	core *CoreAPI
}

type pinningAdder CoreAPI

func (adder *pinningAdder) Add(ctx context.Context, nd ipld.Node) error {

	return adder.dag.Add(ctx, nd)
}

func (adder *pinningAdder) AddMany(ctx context.Context, nds []ipld.Node) error {

	return adder.dag.AddMany(ctx, nds) // err != nil {
	// 		return err
	// 	}
	//
	// 	cids := cid.NewSet()
	//
	// 	for _, nd := range nds {
	// 		c := nd.Cid()
	// 		if cids.Visit(c) {
	// 			err := adder.pinning.AddPin(pinPath, c, true)
	// 			if err != nil {
	// 				return err
	// 			}
	// 		}
	// 	}
	// 	return nil
}

func (api *dagAPI) Pinning() ipld.NodeAdder {
	return (*pinningAdder)(api.core)
}

func (api *dagAPI) Session(ctx context.Context) ipld.NodeGetter {
	return dag.NewSession(ctx, api.DAGService)
}

var (
	_ ipld.DAGService  = (*dagAPI)(nil)
	_ dag.SessionMaker = (*dagAPI)(nil)
)
