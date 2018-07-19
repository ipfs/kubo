package iface

import (
	"context"

	"github.com/ipfs/go-ipfs/core/coreapi/interface/options"

	peer "gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
	pstore "gx/ipfs/Qmda4cPRvSRyox3SqgJN6DfSZGU5TtHufPTp9uXjFj71X6/go-libp2p-peerstore"
)

// DhtAPI specifies the interface to the DHT
type DhtAPI interface {
	// FindPeer queries the DHT for all of the multiaddresses associated with a
	// Peer ID
	FindPeer(context.Context, peer.ID) (pstore.PeerInfo, error)

	// FindProviders finds peers in the DHT who can provide a specific value
	// given a key.
	FindProviders(context.Context, Path, ...options.DhtFindProvidersOption) (<-chan pstore.PeerInfo, error)

	// Provide announces to the network that you are providing given values
	Provide(context.Context, Path, ...options.DhtProvideOption) error
}
