package coreapi

import (
	"context"
	"errors"
	"fmt"
	"strings"

	blockservice "github.com/ipfs/boxo/blockservice"
	blockstore "github.com/ipfs/boxo/blockstore"
	offline "github.com/ipfs/boxo/exchange/offline"
	dag "github.com/ipfs/boxo/ipld/merkledag"
	"github.com/ipfs/boxo/path"
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

type RoutingAPI CoreAPI

func (api *RoutingAPI) Get(ctx context.Context, key string) ([]byte, error) {
	if !api.nd.IsOnline {
		return nil, coreiface.ErrOffline
	}

	dhtKey, err := normalizeKey(key)
	if err != nil {
		return nil, err
	}

	return api.routing.GetValue(ctx, dhtKey)
}

func (api *RoutingAPI) Put(ctx context.Context, key string, value []byte, opts ...caopts.RoutingPutOption) error {
	options, err := caopts.RoutingPutOptions(opts...)
	if err != nil {
		return err
	}

	err = api.checkOnline(options.AllowOffline)
	if err != nil {
		return err
	}

	dhtKey, err := normalizeKey(key)
	if err != nil {
		return err
	}

	return api.routing.PutValue(ctx, dhtKey, value)
}

func normalizeKey(s string) (string, error) {
	parts := strings.Split(s, "/")
	if len(parts) != 3 ||
		parts[0] != "" ||
		!(parts[1] == "ipns" || parts[1] == "pk") {
		return "", errors.New("invalid key")
	}

	k, err := peer.Decode(parts[2])
	if err != nil {
		return "", err
	}
	return strings.Join(append(parts[:2], string(k)), "/"), nil
}

func (api *RoutingAPI) FindPeer(ctx context.Context, p peer.ID) (peer.AddrInfo, error) {
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

func (api *RoutingAPI) FindProviders(ctx context.Context, p path.Path, opts ...caopts.RoutingFindProvidersOption) (<-chan peer.AddrInfo, error) {
	ctx, span := tracing.Span(ctx, "CoreAPI.DhtAPI", "FindProviders", trace.WithAttributes(attribute.String("path", p.String())))
	defer span.End()

	settings, err := caopts.RoutingFindProvidersOptions(opts...)
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
		return nil, errors.New("number of providers must be greater than 0")
	}

	pchan := api.routing.FindProvidersAsync(ctx, rp.RootCid(), numProviders)
	return pchan, nil
}

func (api *RoutingAPI) Provide(ctx context.Context, path path.Path, opts ...caopts.RoutingProvideOption) error {
	ctx, span := tracing.Span(ctx, "CoreAPI.DhtAPI", "Provide", trace.WithAttributes(attribute.String("path", path.String())))
	defer span.End()

	settings, err := caopts.RoutingProvideOptions(opts...)
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
		err = provideKeysRec(ctx, api.routing, api.blockstore, []cid.Cid{c})
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

func provideKeysRec(ctx context.Context, r routing.Routing, bs blockstore.Blockstore, cids []cid.Cid) error {
	provided := cidutil.NewStreamingSet()

	errCh := make(chan error)
	go func() {
		dserv := dag.NewDAGService(blockservice.New(bs, offline.Exchange(bs)))
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

func (api *RoutingAPI) core() coreiface.CoreAPI {
	return (*CoreAPI)(api)
}
