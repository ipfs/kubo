package routing

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

// IpfsRouting is the routing module interface
// It is implemented by things like DHTs, etc.
type IpfsRouting interface {
	FindProvidersAsync(context.Context, u.Key, int) <-chan *peer.Peer

	// Basic Put/Get

	// PutValue adds value corresponding to given Key.
	PutValue(context.Context, u.Key, []byte) error

	// GetValue searches for the value corresponding to given Key.
	GetValue(context.Context, u.Key) ([]byte, error)

	// Value provider layer of indirection.
	// This is what DSHTs (Coral and MainlineDHT) do to store large values in a DHT.

	// Announce that this node can provide value for given key
	Provide(context.Context, u.Key) error

	// Find specific Peer
	// FindPeer searches for a peer with given ID.
	FindPeer(context.Context, peer.ID) (*peer.Peer, error)
}
