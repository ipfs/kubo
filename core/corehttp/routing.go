package corehttp

import (
	"context"
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

func (r *contentRouter) GetClosestPeers(ctx context.Context, pid peer.ID) (iter.ResultIter[*types.PeerRecord], error) {
	// Per the spec, if the peer ID is empty, we should use self.
	if pid == "" {
		pid = r.n.Identity
	}

	peers, err := r.n.DHT.WAN.GetClosestPeers(ctx, string(pid))
	if err != nil {
		return nil, err
	}

	lanPeers, err := r.n.DHT.LAN.GetClosestPeers(ctx, string(pid))
	if err != nil {
		return nil, err
	}
	peers = append(peers, lanPeers...)

	// We have some DHT-closest peers. Find addresses for them.  We can
	// use any routers for that, we can find records for DHT peers on
	// non-DHT routers with whatever protocols. FIXME: right? right??
	var records []*types.PeerRecord
	for _, p := range peers {
		record := types.PeerRecord{
			ID:     &p,
			Schema: types.SchemaPeer,
			// we dont seem to care about protocol/extra infos
			// FIXME: should FindPeers care? That info seems to
			// not cross the FindPeer API.
		}
		// FindPeers will an iterator with a single item because
		// that's how it's implemented above. Treat it as if it
		// returned several records for a peer anyways. And merge them into the one above.
		peerIter, err := r.FindPeers(ctx, p, -1)
		if err != nil {
			continue
		}
		defer peerIter.Close()

		for peerIter.Next() {
			val := peerIter.Val()
			if val.Err != nil {
				continue
			}
			record.Addrs = append(record.Addrs, val.Val.Addrs...)
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
