package coreapi

import (
	"context"
	"errors"
	"fmt"

	"github.com/ipfs/boxo/namesys"
	"github.com/ipfs/kubo/tracing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/ipfs/boxo/path"
	ipfspathresolver "github.com/ipfs/boxo/path/resolver"
	ipld "github.com/ipfs/go-ipld-format"
	coreiface "github.com/ipfs/kubo/core/coreiface"
)

// ResolveNode resolves the path `p` using Unixfs resolver, gets and returns the
// resolved Node.
func (api *CoreAPI) ResolveNode(ctx context.Context, p path.Path) (ipld.Node, error) {
	ctx, span := tracing.Span(ctx, "CoreAPI", "ResolveNode", trace.WithAttributes(attribute.String("path", p.String())))
	defer span.End()

	rp, _, err := api.ResolvePath(ctx, p)
	if err != nil {
		return nil, err
	}

	node, err := api.dag.Get(ctx, rp.RootCid())
	if err != nil {
		return nil, err
	}
	return node, nil
}

// ResolvePath resolves the path `p` using Unixfs resolver, returns the
// resolved path.
func (api *CoreAPI) ResolvePath(ctx context.Context, p path.Path) (path.ImmutablePath, []string, error) {
	ctx, span := tracing.Span(ctx, "CoreAPI", "ResolvePath", trace.WithAttributes(attribute.String("path", p.String())))
	defer span.End()

	res, err := namesys.Resolve(ctx, api.namesys, p)
	if errors.Is(err, namesys.ErrNoNamesys) {
		return path.ImmutablePath{}, nil, coreiface.ErrOffline
	} else if err != nil {
		return path.ImmutablePath{}, nil, err
	}
	p = res.Path

	var resolver ipfspathresolver.Resolver
	switch p.Namespace() {
	case path.IPLDNamespace:
		resolver = api.ipldPathResolver
	case path.IPFSNamespace:
		resolver = api.unixFSPathResolver
	default:
		return path.ImmutablePath{}, nil, fmt.Errorf("unsupported path namespace: %s", p.Namespace())
	}

	imPath, err := path.NewImmutablePath(p)
	if err != nil {
		return path.ImmutablePath{}, nil, err
	}

	node, remainder, err := resolver.ResolveToLastNode(ctx, imPath)
	if err != nil {
		return path.ImmutablePath{}, nil, err
	}

	segments := []string{p.Namespace(), node.String()}
	segments = append(segments, remainder...)

	p, err = path.NewPathFromSegments(segments...)
	if err != nil {
		return path.ImmutablePath{}, nil, err
	}

	imPath, err = path.NewImmutablePath(p)
	if err != nil {
		return path.ImmutablePath{}, nil, err
	}

	return imPath, remainder, nil
}
