package mockrouting

import (
	"math/rand"
	"sync"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
	"github.com/jbenet/go-ipfs/util/testutil"
)

// server is the mockrouting.Client's private interface to the routing server
type server interface {
	Announce(peer.PeerInfo, u.Key) error
	Providers(u.Key) []peer.PeerInfo

	Server
}

// s is an implementation of the private server interface
type s struct {
	delayConf DelayConfig

	lock      sync.RWMutex
	providers map[u.Key]map[peer.ID]providerRecord
}

type providerRecord struct {
	Peer    peer.PeerInfo
	Created time.Time
}

func (rs *s) Announce(p peer.PeerInfo, k u.Key) error {
	rs.lock.Lock()
	defer rs.lock.Unlock()

	_, ok := rs.providers[k]
	if !ok {
		rs.providers[k] = make(map[peer.ID]providerRecord)
	}
	rs.providers[k][p.ID] = providerRecord{
		Created: time.Now(),
		Peer:    p,
	}
	return nil
}

func (rs *s) Providers(k u.Key) []peer.PeerInfo {
	rs.delayConf.Query.Wait() // before locking

	rs.lock.RLock()
	defer rs.lock.RUnlock()

	var ret []peer.PeerInfo
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

func (rs *s) Client(p testutil.Peer) Client {
	return rs.ClientWithDatastore(context.Background(), p, ds.NewMapDatastore())
}

func (rs *s) ClientWithDatastore(_ context.Context, p testutil.Peer, datastore ds.Datastore) Client {
	return &client{
		peer:      p,
		datastore: ds.NewMapDatastore(),
		server:    rs,
	}
}
