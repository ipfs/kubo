package mockrouting

import (
	"math/rand"
	"sync"
	"time"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

// server is the mockrouting.Client's private interface to the routing server
type server interface {
	Announce(peer.Peer, u.Key) error
	Providers(u.Key) []peer.Peer

	Server
}

// s is an implementation of the private server interface
type s struct {
	delayConf DelayConfig

	lock      sync.RWMutex
	providers map[u.Key]map[u.Key]providerRecord
}

type providerRecord struct {
	Peer    peer.Peer
	Created time.Time
}

func (rs *s) Announce(p peer.Peer, k u.Key) error {
	rs.lock.Lock()
	defer rs.lock.Unlock()

	_, ok := rs.providers[k]
	if !ok {
		rs.providers[k] = make(map[u.Key]providerRecord)
	}
	rs.providers[k][p.Key()] = providerRecord{
		Created: time.Now(),
		Peer:    p,
	}
	return nil
}

func (rs *s) Providers(k u.Key) []peer.Peer {
	rs.delayConf.Query.Wait() // before locking

	rs.lock.RLock()
	defer rs.lock.RUnlock()

	var ret []peer.Peer
	records, ok := rs.providers[k]
	if !ok {
		return ret
	}
	for _, r := range records {
		if time.Now().Sub(r.Created) > rs.delayConf.ValueVisibility.Get() {
			ret = append(ret, r.Peer)
		}
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
