package iface

import (
	"context"

	options "github.com/ipfs/go-ipfs/core/coreapi/interface/options"

	peer "gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
	ma "gx/ipfs/QmYmsdtJ3HsodkePE3eU3TsCaP2YvPZJ4LoXnNkDE5Tpt7/go-multiaddr"
)

// DhtAPI specifies the interface to the DHT
type DhtAPI interface {
	// FindPeer queries the DHT for all of the multiaddresses associated with a
	// Peer ID
	FindPeer(context.Context, peer.ID) (<-chan ma.Multiaddr, error)

	// FindProviders finds peers in the DHT who can provide a specific value
	// given a key.
	FindProviders(context.Context, Path, ...options.DhtFindProvidersOption) (<-chan peer.ID, error) //TODO: is path the right choice here?

	// WithNumProviders is an option for FindProviders which specifies the
	// number of peers to look for. Default is 20
	WithNumProviders(numProviders int) options.DhtFindProvidersOption

	// Provide announces to the network that you are providing given values
	Provide(context.Context, Path, ...options.DhtProvideOption) error

	// WithRecursive is an option for Provide which specifies whether to provide
	// the given path recursively
	WithRecursive(recursive bool) options.DhtProvideOption
}
