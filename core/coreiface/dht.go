package iface

import (
	"context"

	options "github.com/ipfs/go-ipfs/core/coreapi/interface/options"

	ma "gx/ipfs/QmWWQ2Txc2c6tqjsBpzg5Ar652cHPGNsQQp2SejkNmkUMb/go-multiaddr"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
)

// DhtAPI specifies the interface to the DHT
type DhtAPI interface {
	// FindPeer queries the DHT for all of the multiaddresses associated with a
	// Peer ID
	FindPeer(context.Context, peer.ID) (<-chan ma.Multiaddr, error)

	// FindProviders finds peers in the DHT who can provide a specific value
	// given a key.
	FindProviders(context.Context, Path) (<-chan peer.ID, error) //TODO: is path the right choice here?

	// Provide announces to the network that you are providing given values
	Provide(context.Context, Path, ...options.DhtProvideOption) error

	// WithRecursive is an option for Provide which specifies whether to provide
	// the given path recursively
	WithRecursive(recursive bool) options.DhtProvideOption
}
