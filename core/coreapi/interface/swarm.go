package iface

import (
	"context"
	"errors"
	"time"

	net "gx/ipfs/QmSTaEYUgDe1r581hxyd2u9582Hgp3KX4wGwYbRqz2u9Qh/go-libp2p-net"
	pstore "gx/ipfs/QmWtCpWB39Rzc2xTB75MKorsxNpo3TyecTEN24CJ3KVohE/go-libp2p-peerstore"
	ma "gx/ipfs/QmYmsdtJ3HsodkePE3eU3TsCaP2YvPZJ4LoXnNkDE5Tpt7/go-multiaddr"
	"gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	"gx/ipfs/QmbNepETomvmXfz1X5pHNFD2QuPqnqi47dTd94QJWSorQ3/go-libp2p-peer"
)

var (
	ErrNotConnected = errors.New("not connected")
	ErrConnNotFound = errors.New("conn not found")
)

// ConnectionInfo contains information about a peer
type ConnectionInfo interface {
	// ID returns PeerID
	ID() peer.ID

	// Address returns the multiaddress via which we are connected with the peer
	Address() ma.Multiaddr

	// Direction returns which way the connection was established
	Direction() net.Direction

	// Latency returns last known round trip time to the peer
	Latency() (time.Duration, error)

	// Streams returns list of streams established with the peer
	Streams() ([]protocol.ID, error)
}

// SwarmAPI specifies the interface to libp2p swarm
type SwarmAPI interface {
	// Connect to a given peer
	Connect(context.Context, pstore.PeerInfo) error

	// Disconnect from a given address
	Disconnect(context.Context, ma.Multiaddr) error

	// Peers returns the list of peers we are connected to
	Peers(context.Context) ([]ConnectionInfo, error)

	// KnownAddrs returns the list of all addresses this node is aware of
	KnownAddrs(context.Context) (map[peer.ID][]ma.Multiaddr, error)

	// LocalAddrs returns the list of announced listening addresses
	LocalAddrs(context.Context) ([]ma.Multiaddr, error)

	// ListenAddrs returns the list of all listening addresses
	ListenAddrs(context.Context) ([]ma.Multiaddr, error)
}
