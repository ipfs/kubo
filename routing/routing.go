package routing

import (
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
	"time"
)

// IpfsRouting is the routing module interface
// It is implemented by things like DHTs, etc.
type IpfsRouting interface {

	// Basic Put/Get

	// PutValue adds value corresponding to given Key.
	PutValue(key u.Key, value []byte) error

	// GetValue searches for the value corresponding to given Key.
	GetValue(key u.Key, timeout time.Duration) ([]byte, error)

	// Value provider layer of indirection.
	// This is what DSHTs (Coral and MainlineDHT) do to store large values in a DHT.

	// Announce that this node can provide value for given key
	Provide(key u.Key) error

	// FindProviders searches for peers who can provide the value for given key.
	FindProviders(key u.Key, timeout time.Duration) ([]*peer.Peer, error)

	// Find specific Peer

	// FindPeer searches for a peer with given ID.
	FindPeer(id peer.ID, timeout time.Duration) (*peer.Peer, error)
}
