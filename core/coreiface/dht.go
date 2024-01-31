package iface

import (
	"context"

	"github.com/ipfs/boxo/path"
	"github.com/ipfs/kubo/core/coreiface/options"
	"github.com/libp2p/go-libp2p/core/peer"
)

// nolint deprecated
// Deprecated: use [RoutingAPI] instead.
type DhtAPI interface {
	// nolint deprecated
	// Deprecated: use [RoutingAPI.FindPeer] instead.
	FindPeer(context.Context, peer.ID) (peer.AddrInfo, error)

	// nolint deprecated
	// Deprecated: use [RoutingAPI.FindProviders] instead.
	FindProviders(context.Context, path.Path, ...options.DhtFindProvidersOption) (<-chan peer.AddrInfo, error)

	// nolint deprecated
	// Deprecated: use [RoutingAPI.Provide] instead.
	Provide(context.Context, path.Path, ...options.DhtProvideOption) error
}
