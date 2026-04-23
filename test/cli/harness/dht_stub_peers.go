package harness

import (
	"context"
	"encoding/hex"
	"sync"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-kad-dht/records"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

// stubPeerPool manages ephemeral in-process libp2p/DHT peers for
// TEST_DHT_STUB mode.
//
// All peers share a single in-memory ProviderStore. This store is
// NOT shared with the kubo daemons; it lives in the test process.
// When a kubo daemon sends ADD_PROVIDER to any ephemeral peer, the
// record is stored in this shared store. When another kubo daemon
// queries GET_PROVIDERS from any peer, it finds the record because
// all peers see the same store. The kubo daemons communicate with
// the ephemeral peers via real DHT protocol messages over loopback
// TCP.
type stubPeerPool struct {
	hosts  []host.Host
	dhts   []*dht.IpfsDHT
	store  *sharedMemStore
	cancel context.CancelFunc
}

// stubDHTPeerCount is the number of ephemeral DHT peers to create.
// Matches amino.DefaultBucketSize (K=20 in Kademlia), ensuring
// GetClosestPeers always finds enough peers for provide replication.
const stubDHTPeerCount = 20

// newStubPeerPool creates count ephemeral DHT peers on loopback and
// mesh-connects them.
func newStubPeerPool(count int) (*stubPeerPool, error) {
	ctx, cancel := context.WithCancel(context.Background())

	store := &sharedMemStore{data: make(map[string][]peer.AddrInfo)}

	hosts := make([]host.Host, 0, count)
	dhts := make([]*dht.IpfsDHT, 0, count)

	cleanup := func() {
		for _, d := range dhts {
			d.Close()
		}
		for _, h := range hosts {
			h.Close()
		}
		cancel()
	}

	for range count {
		h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
		if err != nil {
			cleanup()
			return nil, err
		}
		d, err := dht.New(ctx, h,
			dht.Mode(dht.ModeServer),
			dht.ProviderStore(store),
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
		hosts:  hosts,
		dhts:   dhts,
		store:  store,
		cancel: cancel,
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
	p.cancel()
}

// sharedMemStore implements records.ProviderStore with a shared
// in-memory map. All ephemeral peers reference the same instance
// so any peer can answer provider queries for any CID.
type sharedMemStore struct {
	mu   sync.RWMutex
	data map[string][]peer.AddrInfo
}

var _ records.ProviderStore = (*sharedMemStore)(nil)

func (s *sharedMemStore) AddProvider(_ context.Context, key []byte, prov peer.AddrInfo) error {
	h := hex.EncodeToString(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.data[h] {
		if existing.ID == prov.ID {
			return nil
		}
	}
	s.data[h] = append(s.data[h], prov)
	return nil
}

func (s *sharedMemStore) GetProviders(_ context.Context, key []byte) ([]peer.AddrInfo, error) {
	h := hex.EncodeToString(key)
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data[h], nil
}

func (s *sharedMemStore) Close() error { return nil }
