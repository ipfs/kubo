package mockrouting

import (
	"math/rand"
	"sync"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
	delay "github.com/jbenet/go-ipfs/util/delay"
)

// server is the mockrouting.Client's private interface to the routing server
type server interface {
	Announce(peer.Peer, u.Key) error
	Providers(u.Key) []peer.Peer

	Server
}

// s is an implementation of the private server interface
type s struct {
	delay delay.D

	lock      sync.RWMutex
	providers map[u.Key]peer.Map
}

func (rs *s) Announce(p peer.Peer, k u.Key) error {
	rs.delay.Wait() // before locking

	rs.lock.Lock()
	defer rs.lock.Unlock()

	_, ok := rs.providers[k]
	if !ok {
		rs.providers[k] = make(peer.Map)
	}
	rs.providers[k][p.Key()] = p
	return nil
}

func (rs *s) Providers(k u.Key) []peer.Peer {
	rs.delay.Wait() // before locking

	rs.lock.RLock()
	defer rs.lock.RUnlock()

	var ret []peer.Peer
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

func (rs *s) Client(p peer.Peer) Client {
	return rs.ClientWithDatastore(p, ds.NewMapDatastore())
}

func (rs *s) ClientWithDatastore(p peer.Peer, datastore ds.Datastore) Client {
	return &client{
		peer:      p,
		datastore: ds.NewMapDatastore(),
		server:    rs,
	}
}
