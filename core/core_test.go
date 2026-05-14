package core

import (
	"os"
	"path/filepath"
	"testing"

	context "context"

	"github.com/ipfs/kubo/repo"

	"github.com/ipfs/boxo/filestore"
	"github.com/ipfs/boxo/keystore"
	datastore "github.com/ipfs/go-datastore"
	syncds "github.com/ipfs/go-datastore/sync"
	config "github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core/node/libp2p"
	golib "github.com/libp2p/go-libp2p"
	ddht "github.com/libp2p/go-libp2p-kad-dht/dual"
	"github.com/libp2p/go-libp2p-kad-dht/fullrt"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	pstore "github.com/libp2p/go-libp2p/core/peerstore"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
)

func TestInitialization(t *testing.T) {
	ctx := context.Background()
	id := testIdentity

	good := []*config.Config{
		{
			Identity: id,
			Addresses: config.Addresses{
				Swarm: []string{"/ip4/0.0.0.0/tcp/4001", "/ip4/0.0.0.0/udp/4001/quic-v1"},
				API:   []string{"/ip4/127.0.0.1/tcp/8000"},
			},
		},

		{
			Identity: id,
			Addresses: config.Addresses{
				Swarm: []string{"/ip4/0.0.0.0/tcp/4001", "/ip4/0.0.0.0/udp/4001/quic-v1"},
				API:   []string{"/ip4/127.0.0.1/tcp/8000"},
			},
		},
	}

	bad := []*config.Config{
		{},
	}

	for i, c := range good {
		r := &repo.Mock{
			C: *c,
			D: syncds.MutexWrap(datastore.NewMapDatastore()),
		}
		n, err := NewNode(ctx, &BuildCfg{Repo: r})
		if n == nil || err != nil {
			t.Error("Should have constructed.", i, err)
		}
	}

	for i, c := range bad {
		r := &repo.Mock{
			C: *c,
			D: syncds.MutexWrap(datastore.NewMapDatastore()),
		}
		n, err := NewNode(ctx, &BuildCfg{Repo: r})
		if n != nil || err == nil {
			t.Error("Should have failed to construct.", i)
		}
	}
}

var testIdentity = config.Identity{
	PeerID:  "QmNgdzLieYi8tgfo2WfTUzNVH5hQK9oAYGVf6dxN12NrHt",
	PrivKey: "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",
}

// mockHostOption creates a HostOption that uses the provided mocknet.
// Inlined to avoid import cycle with core/mock package.
func mockHostOption(mn mocknet.Mocknet) libp2p.HostOption {
	return func(id peer.ID, ps pstore.Peerstore, opts ...golib.Option) (host.Host, error) {
		var cfg golib.Config
		if err := cfg.Apply(opts...); err != nil {
			return nil, err
		}

		// The mocknet does not use the provided libp2p.Option. This options include
		// the listening addresses we want our peer listening on. Therefore, we have
		// to manually parse the configuration and add them here.
		ps.AddAddrs(id, cfg.ListenAddrs, pstore.PermanentAddrTTL)
		return mn.AddPeerWithPeerstore(id, ps)
	}
}

func TestHasActiveDHTClient(t *testing.T) {
	// Test 1: nil DHTClient
	t.Run("nil DHTClient", func(t *testing.T) {
		node := &IpfsNode{
			DHTClient: nil,
		}
		if node.HasActiveDHTClient() {
			t.Error("Expected false for nil DHTClient")
		}
	})

	// Test 2: Typed nil *ddht.DHT (common case when Routing.Type=delegated)
	t.Run("typed nil ddht.DHT", func(t *testing.T) {
		node := &IpfsNode{
			DHTClient: (*ddht.DHT)(nil),
		}
		if node.HasActiveDHTClient() {
			t.Error("Expected false for typed nil *ddht.DHT")
		}
	})

	// Test 3: Typed nil *fullrt.FullRT (accelerated DHT client)
	t.Run("typed nil fullrt.FullRT", func(t *testing.T) {
		node := &IpfsNode{
			DHTClient: (*fullrt.FullRT)(nil),
		}
		if node.HasActiveDHTClient() {
			t.Error("Expected false for typed nil *fullrt.FullRT")
		}
	})

	// Test 4: routinghelpers.Null no-op router (Routing.Type=none)
	t.Run("routinghelpers.Null", func(t *testing.T) {
		node := &IpfsNode{
			DHTClient: routinghelpers.Null{},
		}
		if node.HasActiveDHTClient() {
			t.Error("Expected false for routinghelpers.Null")
		}
	})

	// Test 5: Valid standard dual DHT (Routing.Type=auto/dht/dhtclient)
	t.Run("valid standard dual DHT", func(t *testing.T) {
		ctx := context.Background()
		mn := mocknet.New()
		defer mn.Close()

		ds := syncds.MutexWrap(datastore.NewMapDatastore())
		c := config.Config{}
		c.Identity = testIdentity
		c.Addresses.Swarm = []string{"/ip4/0.0.0.0/tcp/4001"}

		r := &repo.Mock{
			C: c,
			D: ds,
			K: keystore.NewMemKeystore(),
			F: filestore.NewFileManager(ds, filepath.Dir(os.TempDir())),
		}

		node, err := NewNode(ctx, &BuildCfg{
			Routing: libp2p.DHTServerOption,
			Repo:    r,
			Host:    mockHostOption(mn),
			Online:  true,
		})
		if err != nil {
			t.Fatalf("Failed to create node with DHT: %v", err)
		}
		defer node.Close()

		// First verify test setup created the expected DHT type
		if node.DHTClient == nil {
			t.Fatalf("Test setup failed: DHTClient is nil")
		}

		if _, ok := node.DHTClient.(*ddht.DHT); !ok {
			t.Fatalf("Test setup failed: expected DHTClient to be *ddht.DHT, got %T", node.DHTClient)
		}

		// Now verify HasActiveDHTClient() correctly identifies it as active
		if !node.HasActiveDHTClient() {
			t.Error("Expected true for valid dual DHT client")
		}
	})

	// Test 6: Valid accelerated DHT client (Routing.Type=autoclient)
	t.Run("valid accelerated DHT client", func(t *testing.T) {
		ctx := context.Background()
		mn := mocknet.New()
		defer mn.Close()

		ds := syncds.MutexWrap(datastore.NewMapDatastore())
		c := config.Config{}
		c.Identity = testIdentity
		c.Addresses.Swarm = []string{"/ip4/0.0.0.0/tcp/4001"}
		c.Routing.AcceleratedDHTClient = config.True

		r := &repo.Mock{
			C: c,
			D: ds,
			K: keystore.NewMemKeystore(),
			F: filestore.NewFileManager(ds, filepath.Dir(os.TempDir())),
		}

		node, err := NewNode(ctx, &BuildCfg{
			Routing: libp2p.DHTOption,
			Repo:    r,
			Host:    mockHostOption(mn),
			Online:  true,
		})
		if err != nil {
			t.Fatalf("Failed to create node with accelerated DHT: %v", err)
		}
		defer node.Close()

		// First verify test setup created the expected accelerated DHT type
		if node.DHTClient == nil {
			t.Fatalf("Test setup failed: DHTClient is nil")
		}

		if _, ok := node.DHTClient.(*fullrt.FullRT); !ok {
			t.Fatalf("Test setup failed: expected DHTClient to be *fullrt.FullRT, got %T", node.DHTClient)
		}

		// Now verify HasActiveDHTClient() correctly identifies it as active
		if !node.HasActiveDHTClient() {
			t.Error("Expected true for valid accelerated DHT client")
		}
	})
}
