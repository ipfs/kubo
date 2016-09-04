// Package mock provides a virtual routing server. To use it, create a virtual
// routing server and use the Client() method to get a routing client
// (IpfsRouting). The server quacks like a DHT but is really a local in-memory
// hash table.
package mockrouting

import (
	delay "github.com/ipfs/go-ipfs/thirdparty/delay"
	peer "gx/ipfs/QmWXjJo15p4pzT7cayEwZi2sWgJqLnGDof6ZGMh9xBgU1p/go-libp2p-peer"
	pstore "gx/ipfs/QmXhhVSpXMUjpf9XgQDyePxug2iHm8ZvZD99aA9N6kuqMN/go-libp2p-peerstore"
	routing "gx/ipfs/QmYQadj3iegqmRPWjaWMRc8DG52hZa2HMkmyPkto5chDvs/go-libp2p-routing"
	"gx/ipfs/QmYpVUnnedgGrp6cX2pBii5HRQgcSr778FiKVe7o7nF5Z3/go-testutil"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
	ds "gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore"
	key "gx/ipfs/Qmce4Y4zg3sYr7xKM5UueS67vhNni6EeWgCRnb7MbLJMew/go-key"
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
