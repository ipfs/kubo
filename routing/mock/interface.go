// Package mock provides a virtual routing server. To use it, create a virtual
// routing server and use the Client() method to get a routing client
// (IpfsRouting). The server quacks like a DHT but is really a local in-memory
// hash table.
package mockrouting

import (
	key "github.com/ipfs/go-ipfs/blocks/key"
	routing "github.com/ipfs/go-ipfs/routing"
	delay "github.com/ipfs/go-ipfs/thirdparty/delay"
	"github.com/ipfs/go-ipfs/thirdparty/testutil"
	peer "gx/ipfs/QmTLeWzYxCE3G4TpArXpHSaa8xJqjm6JnxJdNSSDedJ2if/go-libp2p-peer"
	ds "gx/ipfs/QmZ6A6P6AMo8SR3jXAwzTuSU6B9R2Y4eqW2yW9VvfUayDN/go-datastore"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
	pstore "gx/ipfs/Qmax7ca82J81QamkGgEG2zqWj35B9QHrDgywzKb8nm8Twp/go-libp2p-peerstore"
)

// Server provides mockrouting Clients
type Server interface {
	Client(p testutil.Identity) Client
	ClientWithDatastore(context.Context, testutil.Identity, ds.Datastore) Client
}

// Client implements IpfsRouting
type Client interface {
	FindProviders(context.Context, key.Key) ([]pstore.PeerInfo, error)
	routing.IpfsRouting
}

// NewServer returns a mockrouting Server
func NewServer() Server {
	return NewServerWithDelay(DelayConfig{
		ValueVisibility: delay.Fixed(0),
		Query:           delay.Fixed(0),
	})
}

// NewServerWithDelay returns a mockrouting Server with a delay!
func NewServerWithDelay(conf DelayConfig) Server {
	return &s{
		providers: make(map[key.Key]map[peer.ID]providerRecord),
		delayConf: conf,
	}
}

type DelayConfig struct {
	// ValueVisibility is the time it takes for a value to be visible in the network
	// FIXME there _must_ be a better term for this
	ValueVisibility delay.D

	// Query is the time it takes to receive a response from a routing query
	Query delay.D
}
