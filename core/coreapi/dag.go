package coreapi

import (
	"context"

	dag "github.com/ipfs/boxo/ipld/merkledag"
	pin "github.com/ipfs/boxo/pinning/pinner"
	cid "github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/ipfs/kubo/tracing"
)

type dagAPI struct {
	ipld.DAGService

	core *CoreAPI
}

type pinningAdder CoreAPI

func (adder *pinningAdder) Add(ctx context.Context, nd ipld.Node) error {
	ctx, span := tracing.Span(ctx, "CoreAPI.PinningAdder", "Add", trace.WithAttributes(attribute.String("node", nd.String())))
	defer span.End()
	defer adder.blockstore.PinLock(ctx).Unlock(ctx)

	if err := adder.dag.Add(ctx, nd); err != nil {
		return err
	}

	if err := adder.pinning.PinWithMode(ctx, nd.Cid(), pin.Recursive, ""); err != nil {
		return err
	}

	return adder.pinning.Flush(ctx)
}

func (adder *pinningAdder) AddMany(ctx context.Context, nds []ipld.Node) error {
	ctx, span := tracing.Span(ctx, "CoreAPI.PinningAdder", "AddMany", trace.WithAttributes(attribute.Int("nodes.count", len(nds))))
	defer span.End()
	defer adder.blockstore.PinLock(ctx).Unlock(ctx)

	if err := adder.dag.AddMany(ctx, nds); err != nil {
		return err
	}

	cids := cid.NewSet()

	for _, nd := range nds {
		c := nd.Cid()
		if cids.Visit(c) {
			if err := adder.pinning.PinWithMode(ctx, c, pin.Recursive, ""); err != nil {
				return err
			}
		}
	}

	return adder.pinning.Flush(ctx)
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
