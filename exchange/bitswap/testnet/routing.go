package bitswap

import (
	"errors"
	"math/rand"
	"sync"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	bsnet "github.com/jbenet/go-ipfs/exchange/bitswap/network"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

type RoutingServer interface {
	// TODO
	Announce(*peer.Peer, u.Key) error

	// TODO
	Providers(u.Key) []*peer.Peer

	// TODO
	// Returns a Routing instance configured to query this hash table
	Client(*peer.Peer) bsnet.Routing
}

func VirtualRoutingServer() RoutingServer {
	return &hashTable{
		providers: make(map[u.Key]peer.Map),
	}
}

type hashTable struct {
	lock      sync.RWMutex
	providers map[u.Key]peer.Map
}

var TODO = errors.New("TODO")

func (rs *hashTable) Announce(p *peer.Peer, k u.Key) error {
	rs.lock.Lock()
	defer rs.lock.Unlock()

	_, ok := rs.providers[k]
	if !ok {
		rs.providers[k] = make(peer.Map)
	}
	rs.providers[k][p.Key()] = p
	return nil
}

func (rs *hashTable) Providers(k u.Key) []*peer.Peer {
	rs.lock.RLock()
	defer rs.lock.RUnlock()
	ret := make([]*peer.Peer, 0)
	peerset, ok := rs.providers[k]
	if !ok {
		return ret
	}
	for _, peer := range peerset {
		ret = append(ret, peer)
	}

	for i := range ret {
		j := rand.Intn(i + 1)
		ret[i], ret[j] = ret[j], ret[i]
	}

	return ret
}

// TODO
func (rs *hashTable) Client(p *peer.Peer) bsnet.Routing {
	return &routingClient{
		peer:      p,
		hashTable: rs,
	}
}

type routingClient struct {
	peer      *peer.Peer
	hashTable RoutingServer
}

func (a *routingClient) FindProvidersAsync(ctx context.Context, k u.Key, max int) <-chan *peer.Peer {
	out := make(chan *peer.Peer)
	go func() {
		defer close(out)
		for i, p := range a.hashTable.Providers(k) {
			if max <= i {
				return
			}
			select {
			case out <- p:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

func (a *routingClient) Provide(key u.Key) error {
	return a.hashTable.Announce(a.peer, key)
}
