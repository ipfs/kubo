package mockrouting

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"gx/ipfs/QmeDA8gNhvRTsbrjEieay5wezupJDiky8xvCzDABbsGzmp/go-testutil"

	peer "gx/ipfs/QmWNY7dV54ZDYmTA1ykVdwNCqC11mpU4zSUp6XDpLTH9eG/go-libp2p-peer"
	pstore "gx/ipfs/QmYijbtjCxFEjSXaudaQAUz3LN5VKLssm8WCUsRoqzXmQR/go-libp2p-peerstore"
	ds "gx/ipfs/QmdHG8MAuARdGHxx4rPQASLcvhz24fzjSQq7AJRAQEorq5/go-datastore"
	dssync "gx/ipfs/QmdHG8MAuARdGHxx4rPQASLcvhz24fzjSQq7AJRAQEorq5/go-datastore/sync"
	cid "gx/ipfs/QmeSrf6pzut73u6zLQkRFQ3ygt3k6XFT2kjdYP8Tnkwwyg/go-cid"
)

// server is the mockrouting.Client's private interface to the routing server
type server interface {
	Announce(pstore.PeerInfo, *cid.Cid) error
	Providers(*cid.Cid) []pstore.PeerInfo

	Server
}

// s is an implementation of the private server interface
type s struct {
	delayConf DelayConfig

	lock      sync.RWMutex
	providers map[string]map[peer.ID]providerRecord
}

type providerRecord struct {
	Peer    pstore.PeerInfo
	Created time.Time
}

func (rs *s) Announce(p pstore.PeerInfo, c *cid.Cid) error {
	rs.lock.Lock()
	defer rs.lock.Unlock()

	k := c.KeyString()

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

func (rs *s) Providers(c *cid.Cid) []pstore.PeerInfo {
	rs.delayConf.Query.Wait() // before locking

	rs.lock.RLock()
	defer rs.lock.RUnlock()
	k := c.KeyString()

	var ret []pstore.PeerInfo
	records, ok := rs.providers[k]
	if !ok {
		return ret
	}
	for _, r := range records {
		if time.Since(r.Created) > rs.delayConf.ValueVisibility.Get() {
			ret = append(ret, r.Peer)
		}
	}

	for i := range ret {
		j := rand.Intn(i + 1)
		ret[i], ret[j] = ret[j], ret[i]
	}

	return ret
}

func (rs *s) Client(p testutil.Identity) Client {
	return rs.ClientWithDatastore(context.Background(), p, dssync.MutexWrap(ds.NewMapDatastore()))
}

func (rs *s) ClientWithDatastore(_ context.Context, p testutil.Identity, datastore ds.Datastore) Client {
	return &client{
		peer:      p,
		datastore: datastore,
		server:    rs,
	}
}
