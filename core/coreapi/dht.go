package coreapi

import (
	"context"
	"fmt"

	blockservice "github.com/ipfs/boxo/blockservice"
	blockstore "github.com/ipfs/boxo/blockstore"
	dag "github.com/ipfs/boxo/ipld/merkledag"
	"github.com/ipfs/boxo/path"
	"github.com/ipfs/boxo/provider"
	cid "github.com/ipfs/go-cid"
	cidutil "github.com/ipfs/go-cidutil"
	coreiface "github.com/ipfs/kubo/core/coreiface"
	caopts "github.com/ipfs/kubo/core/coreiface/options"
	"github.com/ipfs/kubo/tracing"
	peer "github.com/libp2p/go-libp2p/core/peer"
	routing "github.com/libp2p/go-libp2p/core/routing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type DhtAPI CoreAPI

func (api *DhtAPI) FindPeer(ctx context.Context, p peer.ID) (peer.AddrInfo, error) {
	ctx, span := tracing.Span(ctx, "CoreAPI.DhtAPI", "FindPeer", trace.WithAttributes(attribute.String("peer", p.String())))
	defer span.End()
	err := api.checkOnline(false)
	if err != nil {
		return peer.AddrInfo{}, err
	}

	pi, err := api.routing.FindPeer(ctx, peer.ID(p))
	if err != nil {
		return peer.AddrInfo{}, err
	}

	return pi, nil
}

func (api *DhtAPI) FindProviders(ctx context.Context, p path.Path, opts ...caopts.DhtFindProvidersOption) (<-chan peer.AddrInfo, error) {
	ctx, span := tracing.Span(ctx, "CoreAPI.DhtAPI", "FindProviders", trace.WithAttributes(attribute.String("path", p.String())))
	defer span.End()

	settings, err := caopts.DhtFindProvidersOptions(opts...)
	if err != nil {
		return nil, err
	}
	span.SetAttributes(attribute.Int("numproviders", settings.NumProviders))

	err = api.checkOnline(false)
	if err != nil {
		return nil, err
	}

	rp, _, err := api.core().ResolvePath(ctx, p)
	if err != nil {
		return nil, err
	}

	numProviders := settings.NumProviders
	if numProviders < 1 {
		return nil, fmt.Errorf("number of providers must be greater than 0")
	}

	pchan := api.routing.FindProvidersAsync(ctx, rp.RootCid(), numProviders)
	return pchan, nil
}

func (api *DhtAPI) Provide(ctx context.Context, path path.Path, opts ...caopts.DhtProvideOption) error {
	ctx, span := tracing.Span(ctx, "CoreAPI.DhtAPI", "Provide", trace.WithAttributes(attribute.String("path", path.String())))
	defer span.End()

	settings, err := caopts.DhtProvideOptions(opts...)
	if err != nil {
		return err
	}
	span.SetAttributes(attribute.Bool("recursive", settings.Recursive))

	err = api.checkOnline(false)
	if err != nil {
		return err
	}

	rp, _, err := api.core().ResolvePath(ctx, path)
	if err != nil {
		return err
	}

	c := rp.RootCid()

	has, err := api.blockstore.Has(ctx, c)
	if err != nil {
		return err
	}

	if !has {
		return fmt.Errorf("block %s not found locally, cannot provide", c)
	}

	if settings.Recursive {
		err = provideKeysRec(ctx, api.routing, api.blockstore, api.provider, []cid.Cid{c})
	} else {
		err = provideKeys(ctx, api.routing, []cid.Cid{c})
	}
	if err != nil {
		return err
	}

	return nil
}

func provideKeys(ctx context.Context, r routing.Routing, cids []cid.Cid) error {
	for _, c := range cids {
		err := r.Provide(ctx, c, true)
		if err != nil {
			return err
		}
	}
	return nil
}

func provideKeysRec(ctx context.Context, r routing.Routing, bs blockstore.Blockstore, prov provider.Provider, cids []cid.Cid) error {
	provided := cidutil.NewStreamingSet()

	errCh := make(chan error)
	go func() {
		// FIXME: we are recreating a dag and blockservice, maybe offline varients should be shared ?
		dserv := dag.NewDAGService(blockservice.New(bs, nil, blockservice.WithProvider(prov)))
		for _, c := range cids {
			err := dag.Walk(ctx, dag.GetLinksDirect(dserv), c, provided.Visitor(ctx))
			if err != nil {
				errCh <- err
			}
		}
	}()

	for {
		select {
		case k := <-provided.New:
			err := r.Provide(ctx, k, true)
			if err != nil {
				return err
			}
		case err := <-errCh:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (api *DhtAPI) core() coreiface.CoreAPI {
	return (*CoreAPI)(api)
}
