package cli

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

// routingTableSnapshotKey is where go-libp2p-kad-dht saves the Amino DHT
// routing table in the repo datastore.
const routingTableSnapshotKey = "/kad-dht/routing-table/ipfs/kad/1.0.0"

// backupBootstrapPeersKey is where Kubo saves backup bootstrap peers while
// Routing.PersistDHTRoutingTable is off.
const backupBootstrapPeersKey = "/local/temp_bootstrap_peers"

// unreachableBootstrapPeer is a bootstrap peer nobody listens for, so every
// dial to it fails.
const unreachableBootstrapPeer = "/ip4/127.0.0.1/tcp/1/p2p/12D3KooWAr43wwcGtURU6Ck6R7cbvfii2unzaK2moqNYQmBQwWgN"

// routingTableSnapshot mirrors the snapshot layout of go-libp2p-kad-dht,
// which pins it with a golden test.
type routingTableSnapshot struct {
	Version int                        `json:"version"`
	SavedAt time.Time                  `json:"savedAt"`
	Peers   []routingTableSnapshotPeer `json:"peers"`
}

type routingTableSnapshotPeer struct {
	ID    string   `json:"id"`
	Addrs []string `json:"addrs"`
}

func TestRoutingTablePersistence(t *testing.T) {
	t.Parallel()

	t.Run("routing table is saved on shutdown", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)
		node := h.NewNode().Init()
		node.SetIPFSConfig("Routing.PersistDHTRoutingTable", true)
		h.BootstrapWithStubDHT(harness.Nodes{node})
		node.StartDaemon()

		require.Eventually(t, func() bool { return wanRoutingTableSize(node) >= 10 }, time.Minute, 200*time.Millisecond)
		shutdown(t, node)

		var snapshot routingTableSnapshot
		res := node.IPFS("diag", "datastore", "get", routingTableSnapshotKey)
		require.NoError(t, json.Unmarshal(res.Stdout.Bytes(), &snapshot))
		require.Equal(t, 1, snapshot.Version)
		require.GreaterOrEqual(t, len(snapshot.Peers), 10)
	})

	t.Run("saved peers stand in for unreachable bootstrap peers", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)
		node := h.NewNode().Init()
		saved := saveStubPeersAsSnapshot(t, h, node)

		node.StartDaemon()
		defer node.StopDaemon()
		require.Eventually(t, func() bool { return connectedToAny(node, saved) }, time.Minute, 200*time.Millisecond)
	})

	t.Run("saved peers are not dialed when disabled", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)
		node := h.NewNode().Init()
		saved := saveStubPeersAsSnapshot(t, h, node)
		node.SetIPFSConfig("Routing.PersistDHTRoutingTable", false)

		node.StartDaemon()
		defer node.StopDaemon()
		requireNeverConnected(t, node, saved)
	})

	t.Run("saved peers are not dialed with an empty Bootstrap list", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)
		node := h.NewNode().Init()
		saved := saveStubPeersAsSnapshot(t, h, node)
		node.SetIPFSConfig("Bootstrap", []string{})

		node.StartDaemon()
		defer node.StopDaemon()
		requireNeverConnected(t, node, saved)
	})

	t.Run("backup bootstrap peers are dropped", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.SetIPFSConfig("Routing.PersistDHTRoutingTable", true)
		node.SetIPFSConfig("Bootstrap", []string{unreachableBootstrapPeer})
		node.IPFS("diag", "datastore", "put", backupBootstrapPeersKey, `["`+unreachableBootstrapPeer+`"]`)

		node.StartDaemon()
		node.StopDaemon()
		res := node.RunIPFS("diag", "datastore", "get", backupBootstrapPeersKey)
		require.Contains(t, res.Stderr.String(), "key not found", "backup bootstrap peers were kept")
	})
}

// requireNeverConnected checks for ten seconds that node connects to none of
// ids. It is a plain loop rather than require.Never, which returns at its
// deadline without waiting for the last check: that check's ipfs command
// would then race the harness removing the repo.
func requireNeverConnected(t *testing.T, node *harness.Node, ids map[peer.ID]bool) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(500 * time.Millisecond) {
		require.False(t, connectedToAny(node, ids), "a saved peer was dialed")
	}
}

// saveStubPeersAsSnapshot writes the harness's stub DHT peers to node's repo
// as a routing table snapshot, turns Routing.PersistDHTRoutingTable on, and
// leaves node with a single, unreachable bootstrap peer. It returns the IDs of
// the saved peers.
func saveStubPeersAsSnapshot(t *testing.T, h *harness.Harness, node *harness.Node) map[peer.ID]bool {
	t.Helper()
	h.BootstrapWithStubDHT(harness.Nodes{node})

	snapshot := routingTableSnapshot{Version: 1, SavedAt: time.Now().UTC()}
	index := map[peer.ID]int{}
	for _, s := range node.ReadConfig().Bootstrap {
		ai, err := peer.AddrInfoFromString(s)
		require.NoError(t, err)
		i, ok := index[ai.ID]
		if !ok {
			i = len(snapshot.Peers)
			index[ai.ID] = i
			snapshot.Peers = append(snapshot.Peers, routingTableSnapshotPeer{ID: ai.ID.String()})
		}
		for _, a := range ai.Addrs {
			snapshot.Peers[i].Addrs = append(snapshot.Peers[i].Addrs, a.String())
		}
	}
	b, err := json.Marshal(snapshot)
	require.NoError(t, err)
	node.IPFS("diag", "datastore", "put", routingTableSnapshotKey, string(b))
	node.SetIPFSConfig("Bootstrap", []string{unreachableBootstrapPeer})
	node.SetIPFSConfig("Routing.PersistDHTRoutingTable", true)

	saved := make(map[peer.ID]bool, len(index))
	for id := range index {
		saved[id] = true
	}
	return saved
}

func connectedToAny(node *harness.Node, ids map[peer.ID]bool) bool {
	for _, a := range node.Peers() {
		ai, err := peer.AddrInfoFromP2pAddr(a)
		if err == nil && ids[ai.ID] {
			return true
		}
	}
	return false
}

func wanRoutingTableSize(node *harness.Node) int {
	res := node.RunIPFS("stats", "dht", "wan", "--enc=json")
	if res.Err != nil {
		return 0
	}
	var stat struct {
		Buckets []struct {
			Peers []json.RawMessage
		}
	}
	if json.Unmarshal(res.Stdout.Bytes(), &stat) != nil {
		return 0
	}
	size := 0
	for _, b := range stat.Buckets {
		size += len(b.Peers)
	}
	return size
}

// shutdown stops node's daemon with `ipfs shutdown`, which closes the DHT
// cleanly. StopDaemon escalates to signals that can cut that short.
func shutdown(t *testing.T, node *harness.Node) {
	t.Helper()
	exited := make(chan error, 1)
	go func() {
		state, err := node.Daemon.Cmd.Process.Wait()
		if err == nil && !state.Success() {
			err = errors.New(state.String())
		}
		exited <- err
	}()
	node.IPFS("shutdown")
	select {
	case err := <-exited:
		require.NoError(t, err)
	case <-time.After(time.Minute):
		_ = node.Daemon.Cmd.Process.Kill()
		t.Fatal("daemon did not exit after ipfs shutdown")
	}
}
