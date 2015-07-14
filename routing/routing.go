// package routing defines the interface for a routing system used by ipfs.
package routing

import (
	"errors"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	key "github.com/ipfs/go-ipfs/blocks/key"
	ci "github.com/ipfs/go-ipfs/p2p/crypto"
	peer "github.com/ipfs/go-ipfs/p2p/peer"
)

// ErrNotFound is returned when a search fails to find anything
var ErrNotFound = errors.New("routing: not found")

// IpfsRouting is the routing module interface
// It is implemented by things like DHTs, etc.
type IpfsRouting interface {
	FindProvidersAsync(context.Context, key.Key, int) <-chan peer.PeerInfo

	// Basic Put/Get

	// PutValue adds value corresponding to given Key.
	PutValue(context.Context, key.Key, []byte) error

	// GetValue searches for the value corresponding to given Key.
	GetValue(context.Context, key.Key) ([]byte, error)

	// Value provider layer of indirection.
	// This is what DSHTs (Coral and MainlineDHT) do to store large values in a DHT.

	// Announce that this node can provide value for given key
	Provide(context.Context, key.Key) error

	// Find specific Peer
	// FindPeer searches for a peer with given ID, returns a peer.PeerInfo
	// with relevant addresses.
	FindPeer(context.Context, peer.ID) (peer.PeerInfo, error)

	// Bootstrap allows callers to hint to the routing system to get into a
	// Boostrapped state
	Bootstrap(context.Context) error

	// TODO expose io.Closer or plain-old Close error
}

type PubKeyFetcher interface {
	GetPublicKey(context.Context, peer.ID) (ci.PubKey, error)
}

// KeyForPublicKey returns the key used to retrieve public keys
// from the dht.
func KeyForPublicKey(id peer.ID) key.Key {
	return key.Key("/pk/" + string(id))
}

func GetPublicKey(r IpfsRouting, ctx context.Context, pkhash []byte) (ci.PubKey, error) {
	if dht, ok := r.(PubKeyFetcher); ok {
		// If we have a DHT as our routing system, use optimized fetcher
		return dht.GetPublicKey(ctx, peer.ID(pkhash))
	} else {
		key := key.Key("/pk/" + string(pkhash))
		pkval, err := r.GetValue(ctx, key)
		if err != nil {
			return nil, err
		}

		// get PublicKey from node.Data
		return ci.UnmarshalPublicKey(pkval)
	}
}
