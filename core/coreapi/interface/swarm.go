package iface

import (
	"context"
	"time"

	ma "gx/ipfs/QmWWQ2Txc2c6tqjsBpzg5Ar652cHPGNsQQp2SejkNmkUMb/go-multiaddr"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
)

// PeerInfo contains information about a peer
type PeerInfo interface {
	// ID returns PeerID
	ID() peer.ID

	// Address returns the multiaddress via which we are connected with the peer
	Address() ma.Multiaddr

	// Latency returns last known round trip time to the peer
	Latency(context.Context) (time.Duration, error)

	// Streams returns list of streams established with the peer
	// TODO: should this return multicodecs?
	Streams(context.Context) ([]string, error)
}

// SwarmAPI specifies the interface to libp2p swarm
type SwarmAPI interface {
	// Connect to a given address
	Connect(context.Context, ma.Multiaddr) error

	// Disconnect from a given address
	Disconnect(context.Context, ma.Multiaddr) error

	// Peers returns the list of peers we are connected to
	Peers(context.Context) ([]PeerInfo, error)
}
