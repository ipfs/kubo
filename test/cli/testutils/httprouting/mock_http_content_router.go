package httprouting

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ipfs/boxo/ipns"
	"github.com/ipfs/boxo/routing/http/server"
	"github.com/ipfs/boxo/routing/http/types"
	"github.com/ipfs/boxo/routing/http/types/iter"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
)

// MockHTTPContentRouter provides /routing/v1
// (https://specs.ipfs.tech/routing/http-routing-v1/) server implementation
// based on github.com/ipfs/boxo/routing/http/server
type MockHTTPContentRouter struct {
	m                   sync.Mutex
	provideBitswapCalls int
	findProvidersCalls  int
	findPeersCalls      int
	providers           map[cid.Cid][]types.Record
	peers               map[peer.ID][]*types.PeerRecord
	Debug               bool
}

func (r *MockHTTPContentRouter) FindProviders(ctx context.Context, key cid.Cid, limit int) (iter.ResultIter[types.Record], error) {
	if r.Debug {
		fmt.Printf("MockHTTPContentRouter.FindProviders(%s)\n", key.String())
	}
	r.m.Lock()
	defer r.m.Unlock()
	r.findProvidersCalls++
	if r.providers == nil {
		r.providers = make(map[cid.Cid][]types.Record)
	}
	records, found := r.providers[key]
	if !found {
		return iter.FromSlice([]iter.Result[types.Record]{}), nil
	}
	results := make([]iter.Result[types.Record], len(records))
	for i, rec := range records {
		results[i] = iter.Result[types.Record]{Val: rec}
		if r.Debug {
			fmt.Printf("MockHTTPContentRouter.FindProviders(%s) result: %+v\n", key.String(), rec)
		}
	}
	return iter.FromSlice(results), nil
}

// nolint deprecated
func (r *MockHTTPContentRouter) ProvideBitswap(ctx context.Context, req *server.BitswapWriteProvideRequest) (time.Duration, error) {
	r.m.Lock()
	defer r.m.Unlock()
	r.provideBitswapCalls++
	return 0, nil
}

func (r *MockHTTPContentRouter) FindPeers(ctx context.Context, pid peer.ID, limit int) (iter.ResultIter[*types.PeerRecord], error) {
	r.m.Lock()
	defer r.m.Unlock()
	r.findPeersCalls++

	if r.peers == nil {
		r.peers = make(map[peer.ID][]*types.PeerRecord)
	}
	records, found := r.peers[pid]
	if !found {
		return iter.FromSlice([]iter.Result[*types.PeerRecord]{}), nil
	}

	results := make([]iter.Result[*types.PeerRecord], len(records))
	for i, rec := range records {
		results[i] = iter.Result[*types.PeerRecord]{Val: rec}
		if r.Debug {
			fmt.Printf("MockHTTPContentRouter.FindPeers(%s) result: %+v\n", pid.String(), rec)
		}
	}
	return iter.FromSlice(results), nil
}

func (r *MockHTTPContentRouter) GetIPNS(ctx context.Context, name ipns.Name) (*ipns.Record, error) {
	return nil, routing.ErrNotSupported
}

func (r *MockHTTPContentRouter) PutIPNS(ctx context.Context, name ipns.Name, rec *ipns.Record) error {
	return routing.ErrNotSupported
}

func (r *MockHTTPContentRouter) NumFindProvidersCalls() int {
	r.m.Lock()
	defer r.m.Unlock()
	return r.findProvidersCalls
}

// AddProvider adds a record for a given CID
func (r *MockHTTPContentRouter) AddProvider(key cid.Cid, record types.Record) {
	r.m.Lock()
	defer r.m.Unlock()
	if r.providers == nil {
		r.providers = make(map[cid.Cid][]types.Record)
	}
	r.providers[key] = append(r.providers[key], record)

	peerRecord, ok := record.(*types.PeerRecord)
	if ok {
		if r.peers == nil {
			r.peers = make(map[peer.ID][]*types.PeerRecord)
		}
		pid := peerRecord.ID
		r.peers[*pid] = append(r.peers[*pid], peerRecord)
	}
}
