package rpc

import (
	"context"

	"github.com/ipfs/boxo/path"
	caopts "github.com/ipfs/kubo/core/coreiface/options"
	"github.com/libp2p/go-libp2p/core/peer"
)

type DhtAPI HttpApi

// nolint deprecated
// Deprecated: use [RoutingAPI.FindPeer] instead.
func (api *DhtAPI) FindPeer(ctx context.Context, p peer.ID) (peer.AddrInfo, error) {
	return api.core().Routing().FindPeer(ctx, p)
}

// nolint deprecated
// Deprecated: use [RoutingAPI.FindProviders] instead.
func (api *DhtAPI) FindProviders(ctx context.Context, p path.Path, opts ...caopts.DhtFindProvidersOption) (<-chan peer.AddrInfo, error) {
	return api.core().Routing().FindProviders(ctx, p, opts...)
}

// nolint deprecated
// Deprecated: use [RoutingAPI.Provide] instead.
func (api *DhtAPI) Provide(ctx context.Context, p path.Path, opts ...caopts.DhtProvideOption) error {
	return api.core().Routing().Provide(ctx, p, opts...)
}

func (api *DhtAPI) core() *HttpApi {
	return (*HttpApi)(api)
}
