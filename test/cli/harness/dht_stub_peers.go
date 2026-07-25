package harness

import (
	"context"

	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

// stubPeerPool manages ephemeral in-process libp2p/DHT peers for
// TEST_DHT_STUB mode.
//
// All peers share a single in-memory provider datastore. This store is
// NOT shared with the kubo daemons; it lives in the test process.
// When a kubo daemon sends ADD_PROVIDER to any ephemeral peer, the
// record is stored in this shared store. When another kubo daemon
// queries GET_PROVIDERS from any peer, it finds the record because
// all peers see the same store. The kubo daemons communicate with
// the ephemeral peers via real DHT protocol messages over loopback
// TCP.
type stubPeerPool struct {
	hosts []host.Host
	dhts  []*dht.IpfsDHT
}

// stubDHTPeerCount is the number of ephemeral DHT peers to create.
// Matches amino.DefaultBucketSize (K=20 in Kademlia), ensuring
// GetClosestPeers always finds enough peers for provide replication.
//
// The shared provider datastore also depends on this equality: because
// every ADD_PROVIDER replicates to K peers, it reaches all stub peers,
// keeping each peer's peerstore (provider addresses) and provider read
// cache coherent with the shared datastore. With more than K peers, a
// peer missed by an ADD_PROVIDER could answer GET_PROVIDERS with empty
// addresses or a stale cached set.
const stubDHTPeerCount = 20

// newStubPeerPool creates count ephemeral DHT peers on loopback and
// mesh-connects them.
func newStubPeerPool(count int) (*stubPeerPool, error) {
	store := dssync.MutexWrap(ds.NewMapDatastore())

	hosts := make([]host.Host, 0, count)
	dhts := make([]*dht.IpfsDHT, 0, count)

	cleanup := func() {
		for _, d := range dhts {
			d.Close()
		}
		for _, h := range hosts {
			h.Close()
		}
	}

	for range count {
		h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
		if err != nil {
			cleanup()
			return nil, err
		}
		d, err := dht.New(h,
			dht.Mode(dht.ModeServer),
			dht.ProviderDatastore(store),
			dht.AddressFilter(nil),
			dht.DisableAutoRefresh(),
			dht.BootstrapPeers(),
		)
		if err != nil {
			h.Close()
			cleanup()
			return nil, err
		}
		hosts = append(hosts, h)
		dhts = append(dhts, d)
	}

	// Full-mesh connect so routing tables are populated.
	ctx := context.Background()
	for i, h := range hosts {
		for j, other := range hosts {
			if i == j {
				continue
			}
			ai := peer.AddrInfo{ID: other.ID(), Addrs: other.Addrs()}
			if err := h.Connect(ctx, ai); err != nil {
				cleanup()
				return nil, err
			}
		}
	}

	return &stubPeerPool{
		hosts: hosts,
		dhts:  dhts,
	}, nil
}

func (p *stubPeerPool) Close() {
	if p == nil {
		return
	}
	for _, d := range p.dhts {
		d.Close()
	}
	for _, h := range p.hosts {
		h.Close()
	}
}
