package cli

import (
	"testing"
	"time"

	"github.com/ipfs/kubo/test/cli/harness"
	. "github.com/ipfs/kubo/test/cli/testutils"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
)

func TestPeering(t *testing.T) {
	t.Parallel()

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
		}, time.Minute, 10*time.Millisecond, "%d -> %d not peered", from.ID, to.ID)
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

	assertPeerings := func(h *harness.Harness, nodes []*harness.Node, peerings []harness.Peering) {
		ForEachPar(peerings, func(peering harness.Peering) {
			assertPeered(h, nodes[peering.From], nodes[peering.To])
		})
	}

	t.Run("bidirectional peering should work (simultaneous connect)", func(t *testing.T) {
		t.Parallel()
		peerings := []harness.Peering{{From: 0, To: 1}, {From: 1, To: 0}, {From: 1, To: 2}}
		h, nodes := harness.CreatePeerNodes(t, 3, peerings)

		nodes.StartDaemons()
		assertPeerings(h, nodes, peerings)

		nodes[0].Disconnect(nodes[1])
		assertPeerings(h, nodes, peerings)
	})

	t.Run("1 should reconnect to 2 when 2 disconnects from 1", func(t *testing.T) {
		t.Parallel()
		peerings := []harness.Peering{{From: 0, To: 1}, {From: 1, To: 0}, {From: 1, To: 2}}
		h, nodes := harness.CreatePeerNodes(t, 3, peerings)

		nodes.StartDaemons()
		assertPeerings(h, nodes, peerings)

		nodes[2].Disconnect(nodes[1])
		assertPeerings(h, nodes, peerings)
	})

	t.Run("1 will peer with 2 when it comes online", func(t *testing.T) {
		t.Parallel()
		peerings := []harness.Peering{{From: 0, To: 1}, {From: 1, To: 0}, {From: 1, To: 2}}
		h, nodes := harness.CreatePeerNodes(t, 3, peerings)

		nodes[0].StartDaemon()
		nodes[1].StartDaemon()
		assertPeerings(h, nodes, []harness.Peering{{From: 0, To: 1}, {From: 1, To: 0}})

		nodes[2].StartDaemon()
		assertPeerings(h, nodes, peerings)
	})

	t.Run("1 will re-peer with 2 when it disconnects and then comes back online", func(t *testing.T) {
		t.Parallel()
		peerings := []harness.Peering{{From: 0, To: 1}, {From: 1, To: 0}, {From: 1, To: 2}}
		h, nodes := harness.CreatePeerNodes(t, 3, peerings)

		nodes.StartDaemons()
		assertPeerings(h, nodes, peerings)

		nodes[2].StopDaemon()
		assertNotPeered(h, nodes[1], nodes[2])

		nodes[2].StartDaemon()
		assertPeerings(h, nodes, peerings)
	})
}
