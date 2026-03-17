package corehttp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/ipfs/boxo/gateway"
	"github.com/ipfs/boxo/ipns"
	"github.com/ipfs/boxo/routing/http/server"
	"github.com/ipfs/boxo/routing/http/types"
	"github.com/ipfs/boxo/routing/http/types/iter"
	cid "github.com/ipfs/go-cid"
	core "github.com/ipfs/kubo/core"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-kad-dht/dual"
	"github.com/libp2p/go-libp2p-kad-dht/fullrt"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
)

func RoutingOption() ServeOption {
	return func(n *core.IpfsNode, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		_, headers, err := getGatewayConfig(n)
		if err != nil {
			return nil, err
		}

		handler := server.Handler(&contentRouter{n})
		handler = gateway.NewHeaders(headers).ApplyCors().Wrap(handler)
		mux.Handle("/routing/v1/", handler)
		return mux, nil
	}
}

type contentRouter struct {
	n *core.IpfsNode
}

func (r *contentRouter) FindProviders(ctx context.Context, key cid.Cid, limit int) (iter.ResultIter[types.Record], error) {
	ctx, cancel := context.WithCancel(ctx)
	ch := r.n.Routing.FindProvidersAsync(ctx, key, limit)
	return iter.ToResultIter[types.Record](&peerChanIter{
		ch:     ch,
		cancel: cancel,
	}), nil
}

// nolint deprecated
func (r *contentRouter) ProvideBitswap(ctx context.Context, req *server.BitswapWriteProvideRequest) (time.Duration, error) {
	return 0, routing.ErrNotSupported
}

func (r *contentRouter) FindPeers(ctx context.Context, pid peer.ID, limit int) (iter.ResultIter[*types.PeerRecord], error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	addr, err := r.n.Routing.FindPeer(ctx, pid)
	if err != nil {
		return nil, err
	}

	rec := &types.PeerRecord{
		Schema: types.SchemaPeer,
		ID:     &addr.ID,
	}

	for _, addr := range addr.Addrs {
		rec.Addrs = append(rec.Addrs, types.Multiaddr{Multiaddr: addr})
	}

	return iter.ToResultIter[*types.PeerRecord](iter.FromSlice[*types.PeerRecord]([]*types.PeerRecord{rec})), nil
}

func (r *contentRouter) GetIPNS(ctx context.Context, name ipns.Name) (*ipns.Record, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	raw, err := r.n.Routing.GetValue(ctx, string(name.RoutingKey()))
	if err != nil {
		return nil, err
	}

	return ipns.UnmarshalRecord(raw)
}

func (r *contentRouter) PutIPNS(ctx context.Context, name ipns.Name, record *ipns.Record) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	raw, err := ipns.MarshalRecord(record)
	if err != nil {
		return err
	}

	// The caller guarantees that name matches the record. This is double checked
	// by the internals of PutValue.
	return r.n.Routing.PutValue(ctx, string(name.RoutingKey()), raw)
}

func (r *contentRouter) GetClosestPeers(ctx context.Context, key cid.Cid) (iter.ResultIter[*types.PeerRecord], error) {
	// Per the spec, if the peer ID is empty, we should use self.
	if key == cid.Undef {
		return nil, errors.New("GetClosestPeers key is undefined")
	}

	keyStr := string(key.Hash())
	var peers []peer.ID
	var err error

	if r.n.DHTClient == nil {
		return nil, fmt.Errorf("GetClosestPeers not supported: DHT is not available")
	}

	switch dhtClient := r.n.DHTClient.(type) {
	case *dual.DHT:
		// Only use WAN DHT for public HTTP Routing API.
		// LAN DHT contains private network peers that should not be exposed publicly.
		if dhtClient.WAN == nil {
			return nil, fmt.Errorf("GetClosestPeers not supported: WAN DHT is not available")
		}
		peers, err = dhtClient.WAN.GetClosestPeers(ctx, keyStr)
	case *fullrt.FullRT:
		peers, err = dhtClient.GetClosestPeers(ctx, keyStr)
	case *dht.IpfsDHT:
		peers, err = dhtClient.GetClosestPeers(ctx, keyStr)
	default:
		return nil, fmt.Errorf("GetClosestPeers not supported for DHT type %T", r.n.DHTClient)
	}

	if err != nil {
		return nil, err
	}

	// We have some DHT-closest peers. Find addresses for them.
	// The addresses should be in the peerstore.
	records := make([]*types.PeerRecord, 0, len(peers))
	for _, p := range peers {
		addrs := r.n.Peerstore.Addrs(p)
		rAddrs := make([]types.Multiaddr, len(addrs))
		for i, addr := range addrs {
			rAddrs[i] = types.Multiaddr{Multiaddr: addr}
		}
		record := types.PeerRecord{
			ID:     &p,
			Schema: types.SchemaPeer,
			Addrs:  rAddrs,
		}
		records = append(records, &record)
	}

	return iter.ToResultIter(iter.FromSlice(records)), nil
}

type peerChanIter struct {
	ch     <-chan peer.AddrInfo
	cancel context.CancelFunc
	next   *peer.AddrInfo
}

func (it *peerChanIter) Next() bool {
	addr, ok := <-it.ch
	if ok {
		it.next = &addr
		return true
	}
	it.next = nil
	return false
}

func (it *peerChanIter) Val() types.Record {
	if it.next == nil {
		return nil
	}

	rec := &types.PeerRecord{
		Schema: types.SchemaPeer,
		ID:     &it.next.ID,
	}

	for _, addr := range it.next.Addrs {
		rec.Addrs = append(rec.Addrs, types.Multiaddr{Multiaddr: addr})
	}

	return rec
}

func (it *peerChanIter) Close() error {
	it.cancel()
	return nil
}
