package coreapi

import (
	"context"

	dag "github.com/ipfs/boxo/ipld/merkledag"
	"github.com/ipfs/boxo/ipld/merkledag/dagutils"
	ft "github.com/ipfs/boxo/ipld/unixfs"
	"github.com/ipfs/boxo/path"
	coreiface "github.com/ipfs/kubo/core/coreiface"
	caopts "github.com/ipfs/kubo/core/coreiface/options"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/ipfs/kubo/tracing"
)

type ObjectAPI CoreAPI

type Link struct {
	Name, Hash string
	Size       uint64
}

type Node struct {
	Links []Link
	Data  string
}

func (api *ObjectAPI) AddLink(ctx context.Context, base path.Path, name string, child path.Path, opts ...caopts.ObjectAddLinkOption) (path.ImmutablePath, error) {
	ctx, span := tracing.Span(ctx, "CoreAPI.ObjectAPI", "AddLink", trace.WithAttributes(
		attribute.String("base", base.String()),
		attribute.String("name", name),
		attribute.String("child", child.String()),
	))
	defer span.End()

	options, err := caopts.ObjectAddLinkOptions(opts...)
	if err != nil {
		return path.ImmutablePath{}, err
	}
	span.SetAttributes(attribute.Bool("create", options.Create))

	baseNd, err := api.core().ResolveNode(ctx, base)
	if err != nil {
		return path.ImmutablePath{}, err
	}

	childNd, err := api.core().ResolveNode(ctx, child)
	if err != nil {
		return path.ImmutablePath{}, err
	}

	basePb, ok := baseNd.(*dag.ProtoNode)
	if !ok {
		return path.ImmutablePath{}, dag.ErrNotProtobuf
	}

	var createfunc func() *dag.ProtoNode
	if options.Create {
		createfunc = ft.EmptyDirNode
	}

	e := dagutils.NewDagEditor(basePb, api.dag)

	err = e.InsertNodeAtPath(ctx, name, childNd, createfunc)
	if err != nil {
		return path.ImmutablePath{}, err
	}

	nnode, err := e.Finalize(ctx, api.dag)
	if err != nil {
		return path.ImmutablePath{}, err
	}

	return path.FromCid(nnode.Cid()), nil
}

func (api *ObjectAPI) RmLink(ctx context.Context, base path.Path, link string) (path.ImmutablePath, error) {
	ctx, span := tracing.Span(ctx, "CoreAPI.ObjectAPI", "RmLink", trace.WithAttributes(
		attribute.String("base", base.String()),
		attribute.String("link", link)),
	)
	defer span.End()

	baseNd, err := api.core().ResolveNode(ctx, base)
	if err != nil {
		return path.ImmutablePath{}, err
	}

	basePb, ok := baseNd.(*dag.ProtoNode)
	if !ok {
		return path.ImmutablePath{}, dag.ErrNotProtobuf
	}

	e := dagutils.NewDagEditor(basePb, api.dag)

	err = e.RmLink(ctx, link)
	if err != nil {
		return path.ImmutablePath{}, err
	}

	nnode, err := e.Finalize(ctx, api.dag)
	if err != nil {
		return path.ImmutablePath{}, err
	}

	return path.FromCid(nnode.Cid()), nil
}

func (api *ObjectAPI) Diff(ctx context.Context, before path.Path, after path.Path) ([]coreiface.ObjectChange, error) {
	ctx, span := tracing.Span(ctx, "CoreAPI.ObjectAPI", "Diff", trace.WithAttributes(
		attribute.String("before", before.String()),
		attribute.String("after", after.String()),
	))
	defer span.End()

	beforeNd, err := api.core().ResolveNode(ctx, before)
	if err != nil {
		return nil, err
	}

	afterNd, err := api.core().ResolveNode(ctx, after)
	if err != nil {
		return nil, err
	}

	changes, err := dagutils.Diff(ctx, api.dag, beforeNd, afterNd)
	if err != nil {
		return nil, err
	}

	out := make([]coreiface.ObjectChange, len(changes))
	for i, change := range changes {
		out[i] = coreiface.ObjectChange{
			Type: coreiface.ChangeType(change.Type),
			Path: change.Path,
		}

		if change.Before.Defined() {
			out[i].Before = path.FromCid(change.Before)
		}

		if change.After.Defined() {
			out[i].After = path.FromCid(change.After)
		}
	}

	return out, nil
}

func (api *ObjectAPI) core() coreiface.CoreAPI {
	return (*CoreAPI)(api)
}
