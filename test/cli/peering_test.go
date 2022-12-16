package cli

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	. "github.com/ipfs/kubo/test/cli/testutils"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
)

func TestPeering(t *testing.T) {
	t.Parallel()

	type peering struct {
		from int
		to   int
	}

	newRandPort := func() int {
		n := rand.Int()
		return 3000 + (n % 1000)
	}

	containsPeerID := func(p peer.ID, peers []peer.ID) bool {
		for _, peerID := range peers {
			if p == peerID {
				return true
			}
		}
		return false
	}

	assertPeered := func(h *harness.Harness, from *harness.Node, to *harness.Node) {
		assert.Eventuallyf(t, func() bool {
			fromPeers := from.Peers()
			if len(fromPeers) == 0 {
				return false
			}
			var fromPeerIDs []peer.ID
			for _, p := range fromPeers {
				fromPeerIDs = append(fromPeerIDs, h.ExtractPeerID(p))
			}
			return containsPeerID(to.PeerID(), fromPeerIDs)
		}, 20*time.Second, 10*time.Millisecond, "%d -> %d not peered", from.ID, to.ID)
	}

	assertNotPeered := func(h *harness.Harness, from *harness.Node, to *harness.Node) {
		assert.Eventuallyf(t, func() bool {
			fromPeers := from.Peers()
			if len(fromPeers) == 0 {
				return false
			}
			var fromPeerIDs []peer.ID
			for _, p := range fromPeers {
				fromPeerIDs = append(fromPeerIDs, h.ExtractPeerID(p))
			}
			return !containsPeerID(to.PeerID(), fromPeerIDs)
		}, 20*time.Second, 10*time.Millisecond, "%d -> %d peered", from.ID, to.ID)
	}

	assertPeerings := func(h *harness.Harness, nodes []*harness.Node, peerings []peering) {
		ForEachPar(peerings, func(peering peering) {
			assertPeered(h, nodes[peering.from], nodes[peering.to])
		})
	}

	createNodes := func(t *testing.T, n int, peerings []peering) (*harness.Harness, harness.Nodes) {
		h := harness.NewT(t)
		nodes := h.NewNodes(n).Init()
		nodes.ForEachPar(func(node *harness.Node) {
			node.UpdateConfig(func(cfg *config.Config) {
				cfg.Routing.Type = config.NewOptionalString("none")
				cfg.Addresses.Swarm = []string{fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", newRandPort())}
			})

		})

		for _, peering := range peerings {
			nodes[peering.from].PeerWith(nodes[peering.to])
		}

		return h, nodes
	}

	t.Run("bidirectional peering should work (simultaneous connect)", func(t *testing.T) {
		t.Parallel()
		peerings := []peering{{from: 0, to: 1}, {from: 1, to: 0}, {from: 1, to: 2}}
		h, nodes := createNodes(t, 3, peerings)

		nodes.StartDaemons()
		assertPeerings(h, nodes, peerings)

		nodes[0].Disconnect(nodes[1])
		assertPeerings(h, nodes, peerings)
	})

	t.Run("1 should reconnect to 2 when 2 disconnects from 1", func(t *testing.T) {
		t.Parallel()
		peerings := []peering{{from: 0, to: 1}, {from: 1, to: 0}, {from: 1, to: 2}}
		h, nodes := createNodes(t, 3, peerings)

		nodes.StartDaemons()
		assertPeerings(h, nodes, peerings)

		nodes[2].Disconnect(nodes[1])
		assertPeerings(h, nodes, peerings)
	})

	t.Run("1 will peer with 2 when it comes online", func(t *testing.T) {
		t.Parallel()
		peerings := []peering{{from: 0, to: 1}, {from: 1, to: 0}, {from: 1, to: 2}}
		h, nodes := createNodes(t, 3, peerings)

		nodes[0].StartDaemon()
		nodes[1].StartDaemon()
		assertPeerings(h, nodes, []peering{{from: 0, to: 1}, {from: 1, to: 0}})

		nodes[2].StartDaemon()
		assertPeerings(h, nodes, peerings)
	})

	t.Run("1 will re-peer with 2 when it disconnects and then comes back online", func(t *testing.T) {
		t.Parallel()
		peerings := []peering{{from: 0, to: 1}, {from: 1, to: 0}, {from: 1, to: 2}}
		h, nodes := createNodes(t, 3, peerings)

		nodes.StartDaemons()
		assertPeerings(h, nodes, peerings)

		nodes[2].StopDaemon()
		assertNotPeered(h, nodes[1], nodes[2])

		nodes[2].StartDaemon()
		assertPeerings(h, nodes, peerings)
	})
}
