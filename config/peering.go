package config

import "github.com/libp2p/go-libp2p-core/peer"

// Peering configures the peering service.
type Peering struct {
	// Peer lists all peers with which this peer peers.
	Peers []peer.AddrInfo
}
