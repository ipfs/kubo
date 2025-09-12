package iface

import (
	"context"

	"github.com/ipfs/boxo/path"
	"github.com/ipfs/kubo/core/coreiface/options"
	"github.com/libp2p/go-libp2p/core/peer"
)

// RoutingAPI specifies the interface to the routing layer.
type RoutingAPI interface {
	// Get retrieves the best value for a given key
	Get(context.Context, string) ([]byte, error)

	// Put sets a value for a given key
	Put(ctx context.Context, key string, value []byte, opts ...options.RoutingPutOption) error

	// FindPeer queries the routing system for all the multiaddresses associated
	// with the given [peer.ID].
	FindPeer(context.Context, peer.ID) (peer.AddrInfo, error)

	// FindProviders finds the peers in the routing system who can provide a specific
	// value given a key.
	FindProviders(context.Context, path.Path, ...options.RoutingFindProvidersOption) (<-chan peer.AddrInfo, error)

	// Provide announces to the network that you are providing given values
	Provide(context.Context, path.Path, ...options.RoutingProvideOption) error
}
