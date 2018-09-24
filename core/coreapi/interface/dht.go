package iface

import (
	"context"

	"github.com/ipfs/go-ipfs/core/coreapi/interface/options"

	peer "gx/ipfs/QmbNepETomvmXfz1X5pHNFD2QuPqnqi47dTd94QJWSorQ3/go-libp2p-peer"
	pstore "gx/ipfs/QmfAQMFpgDU2U4BXG64qVr8HSiictfWvkSBz7Y2oDj65st/go-libp2p-peerstore"
)

// DhtAPI specifies the interface to the DHT
// Note: This API will likely get deprecated in near future, see
// https://github.com/ipfs/interface-ipfs-core/issues/249 for more context.
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
