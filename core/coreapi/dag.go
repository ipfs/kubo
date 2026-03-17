package coreapi

import (
	"context"
	"fmt"

	dag "github.com/ipfs/boxo/ipld/merkledag"
	"github.com/ipfs/boxo/ipld/merkledag/traverse"
	"github.com/ipfs/boxo/path"
	pin "github.com/ipfs/boxo/pinning/pinner"
	cid "github.com/ipfs/go-cid"
	coreiface "github.com/ipfs/kubo/core/coreiface"
	"github.com/ipfs/kubo/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	ipld "github.com/ipfs/go-ipld-format"
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

func (api *dagAPI) Stat(ctx context.Context, p path.Path) (*coreiface.DagStatResult, error) {
	rp, remainder, err := api.core.ResolvePath(ctx, p)
	if err != nil {
		return nil, err
	}
	if len(remainder) > 0 {
		return nil, fmt.Errorf("cannot return size for anything other than a DAG with a root CID")
	}
	nodeGetter := dag.NewSession(ctx, api.DAGService)
	obj, err := nodeGetter.Get(ctx, rp.RootCid())
	if err != nil {
		return nil, err
	}
	result := &coreiface.DagStatResult{}
	err = traverse.Traverse(obj, traverse.Options{
		DAG:   nodeGetter,
		Order: traverse.DFSPre,
		Func: func(current traverse.State) error {
			result.Size += uint64(len(current.Node.RawData()))
			result.NumBlocks++
			return nil
		},
		SkipDuplicates: true,
	})
	if err != nil {
		return nil, fmt.Errorf("error traversing DAG: %w", err)
	}
	return result, nil
}

var (
	_ ipld.DAGService  = (*dagAPI)(nil)
	_ dag.SessionMaker = (*dagAPI)(nil)
)
