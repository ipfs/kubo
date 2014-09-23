package mockrouter

import (
	"errors"
	"math/rand"
	"sync"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"
	peer "github.com/jbenet/go-ipfs/peer"
	routing "github.com/jbenet/go-ipfs/routing"
	u "github.com/jbenet/go-ipfs/util"
)

var _ routing.IpfsRouting = &MockRouter{}

type MockRouter struct {
	datastore ds.Datastore
	hashTable RoutingServer
	peer      *peer.Peer
}

func NewMockRouter(local *peer.Peer, dstore ds.Datastore) *MockRouter {
	return &MockRouter{
		datastore: dstore,
		peer:      local,
		hashTable: VirtualRoutingServer(),
	}
}

func (mr *MockRouter) SetRoutingServer(rs RoutingServer) {
	mr.hashTable = rs
}

func (mr *MockRouter) PutValue(ctx context.Context, key u.Key, val []byte) error {
	return mr.datastore.Put(ds.NewKey(string(key)), val)
}

func (mr *MockRouter) GetValue(ctx context.Context, key u.Key) ([]byte, error) {
	v, err := mr.datastore.Get(ds.NewKey(string(key)))
	if err != nil {
		return nil, err
	}

	data, ok := v.([]byte)
	if !ok {
		return nil, errors.New("could not cast value from datastore")
	}

	return data, nil
}

func (mr *MockRouter) FindProviders(ctx context.Context, key u.Key) ([]*peer.Peer, error) {
	return nil, nil
}

func (mr *MockRouter) FindPeer(ctx context.Context, pid peer.ID) (*peer.Peer, error) {
	return nil, nil
}

func (mr *MockRouter) FindProvidersAsync(ctx context.Context, k u.Key, max int) <-chan *peer.Peer {
	out := make(chan *peer.Peer)
	go func() {
		defer close(out)
		for i, p := range mr.hashTable.Providers(k) {
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

func (mr *MockRouter) Provide(_ context.Context, key u.Key) error {
	return mr.hashTable.Announce(mr.peer, key)
}

type RoutingServer interface {
	Announce(*peer.Peer, u.Key) error

	Providers(u.Key) []*peer.Peer
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
