// Package mock provides a virtual routing server. To use it, create a virtual
// routing server and use the Client() method to get a routing client
// (IpfsRouting). The server quacks like a DHT but is really a local in-memory
// hash table.
package mockrouting

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	peer "github.com/jbenet/go-ipfs/peer"
	routing "github.com/jbenet/go-ipfs/routing"
	u "github.com/jbenet/go-ipfs/util"
	delay "github.com/jbenet/go-ipfs/util/delay"
)

// Server provides mockrouting Clients
type Server interface {
	Client(p peer.Peer) Client
	ClientWithDatastore(peer.Peer, ds.Datastore) Client
}

// Client implements IpfsRouting
type Client interface {
	FindProviders(context.Context, u.Key) ([]peer.Peer, error)

	routing.IpfsRouting
}

// NewServer returns a mockrouting Server
func NewServer() Server {
	return NewServerWithDelay(delay.Fixed(0))
}

// NewServerWithDelay returns a mockrouting Server with a delay!
func NewServerWithDelay(d delay.D) Server {
	return &s{
		providers: make(map[u.Key]peer.Map),
		delay:     d,
	}
}
