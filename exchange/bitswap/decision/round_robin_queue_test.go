// Round-Robin Queue Tests
// =======================

package decision

import (
    "testing"

	peer "gx/ipfs/QmWNY7dV54ZDYmTA1ykVdwNCqC11mpU4zSUp6XDpLTH9eG/go-libp2p-peer"
	testutil "gx/ipfs/QmeDA8gNhvRTsbrjEieay5wezupJDiky8xvCzDABbsGzmp/go-testutil"
)

// Peer Management
// ---------------

func TestRRQWeightAllocationsConstant(t *testing.T) {

    roundBurst := 50
    rrq := newRRQueueCustom(Simple, roundBurst)
    numPeers := 5

    expected := int(float64(roundBurst) / float64(numPeers))

    for i := 0; i < numPeers; i++ {
        // get random peer ID
        peerID := testutil.RandPeerIDFatal(t)
        // get a cid
        //cid := cid.NewCidV0(u.Hash([]byte(fmt.Sprint(i))))
        // generate new ledger, and set its value to 1
        receipt := newLedger(peerID).Receipt()
        receipt.Value = float64(1)

        rrq.UpdateWeight(peerID, receipt)
    }

    rrq.InitRound()

    if len(rrq.allocations) != numPeers {
        t.Fatalf("Expected %d allocations, got %d", numPeers,
                 len(rrq.allocations))
    }
    for i, rrp := range rrq.allocations {
        if rrp.allocation != expected {
            t.Fatalf("Bad allocation: peer %d -- expected %d, got %d",
                     i, expected, rrp.allocation)
        }
    }

    rrq.ResetAllocations()
    if rrq.NumPeers() != 0 {
        t.Fatalf("Resetting allocations failed. Should be 0 allocations, but there are %d", 
                rrq.NumPeers())
    }

    if len(rrq.weights) != numPeers {
        t.Fatalf("Resetting allocations also affected the weights. This shouldn't happen.")
    }
}

func TestRRQWeightAllocationsVarying(t *testing.T) {
    roundBurst := 50
    rrq := newRRQueueCustom(Simple, roundBurst)
    numPeers := 5
    expected := make(map[peer.ID]int)

    for i := 0; i < numPeers; i++ {
        // get random peer ID
        peerID := testutil.RandPeerIDFatal(t)
        // generate new ledger, and set its value to 1
        receipt := newLedger(peerID).Receipt()
        receipt.Value = float64(i)

        rrq.UpdateWeight(peerID, receipt)

        allocation := int(float64(i * roundBurst) / float64(sum1ToN(numPeers-1)))
        expected[peerID] = allocation
    }
    rrq.InitRound()

    // check that the correct number of peers were allocated (numPeers - 1
    // because one peer has a weight of 0)
    if len(rrq.allocations) != numPeers - 1 {
        t.Fatalf("Expected %d allocations, got %d", numPeers - 1,
                 len(rrq.allocations))
    }

    // check that all peers received the correct allocation
    for _, val := range rrq.allocations {
        if expected[val.id] != val.allocation {
            t.Fatalf("Bad allocation: peer %s -- expected %d, got %d",
                     val.id, expected[val.id], val.allocation)
        }
    }
}

func TestRRQPopHeadShift(t *testing.T) {
    roundBurst := 50
    rrq := newRRQueueCustom(Simple, roundBurst)
    numPeers := 5

    for i := 0; i < numPeers; i++ {
        // get random peer ID
        peerID := testutil.RandPeerIDFatal(t)
        // get a cid
        //cid := cid.NewCidV0(u.Hash([]byte(fmt.Sprint(i))))
        // generate new ledger, and set its value to 1
        receipt := newLedger(peerID).Receipt()
        receipt.Value = float64(1)

        rrq.UpdateWeight(peerID, receipt)
    }
    rrq.InitRound()

    original := make([]*RRPeer, rrq.NumPeers())
    for i, val := range rrq.allocations {
        original[i] = val
    }

    // shift as many times as there are allocations
    for i := 0; i < rrq.NumPeers(); i++ {
        rrq.Shift()
    }
    // check that the shifts acted as identity
    if len(original) != rrq.NumPeers() {
        t.Fatalf("Expected %d allocations, got %d", len(original), rrq.NumPeers())
    }
    for i, val := range original {
        if rrq.allocations[i] != val {
            t.Fatalf("Allocations changed after shifts.")
        }
    }

    queue := make([]*RRPeer, rrq.NumPeers())
    // pop everything, check results are as expected
    i := 0
    for rrq.NumPeers() > 0 {
        peer := rrq.Head()
        queue[i] = peer
        i++
        rrq.Pop()
    }

    if len(original) != len(queue) {
        t.Fatalf("Expected %d allocations, got %d", len(original), len(queue))
    }
    for i, val := range original {
        if queue[i] != val {
            t.Fatalf("Peers did not pop in expected order.")
        }
    }
}

// Helper Functions
// ----------------

func sum1ToN(n int) int {
    sum := 0
    for i := 1; i <= n; i++ {
        sum += i
    }
    return sum
}
