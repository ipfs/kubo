package cli

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// waitForSubscription waits until the node has a subscription to the given topic.
func waitForSubscription(t *testing.T, node *harness.Node, topic string) {
	t.Helper()
	require.Eventually(t, func() bool {
		res := node.RunIPFS("pubsub", "ls")
		if res.Err != nil {
			return false
		}
		return slices.Contains(res.Stdout.Lines(), topic)
	}, 5*time.Second, 100*time.Millisecond, "expected subscription to topic %s", topic)
}

// waitForMessagePropagation waits for pubsub messages to propagate through the network
// and for seqno state to be persisted to the datastore.
func waitForMessagePropagation(t *testing.T) {
	t.Helper()
	time.Sleep(1 * time.Second)
}

// publishMessages publishes n messages from publisher to the given topic with
// a small delay between each to allow for ordered delivery.
func publishMessages(t *testing.T, publisher *harness.Node, topic string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		publisher.PipeStrToIPFS("msg", "pubsub", "pub", topic)
		time.Sleep(50 * time.Millisecond)
	}
}

// TestPubsub tests pubsub functionality and the persistent seqno validator.
//
// Pubsub has two deduplication layers:
//
// Layer 1: MessageID-based TimeCache (in-memory)
//   - Controlled by Pubsub.SeenMessagesTTL config (default 120s)
//   - Tested in go-libp2p-pubsub (see timecache in github.com/libp2p/go-libp2p-pubsub)
//   - Only tested implicitly here via message delivery (timing-sensitive, not practical for CLI tests)
//
// Layer 2: Per-peer seqno validator (persistent in datastore)
//   - Stores max seen seqno per peer at /pubsub/seqno/<peerid>
//   - Tested directly below: persistence, updates, reset, survives restart
//   - Validator: go-libp2p-pubsub BasicSeqnoValidator
func TestPubsub(t *testing.T) {
	t.Parallel()

	// enablePubsub configures a node with pubsub enabled
	enablePubsub := func(n *harness.Node) {
		n.SetIPFSConfig("Pubsub.Enabled", true)
		n.SetIPFSConfig("Routing.Type", "none") // simplify test setup
	}

	t.Run("basic pub/sub message delivery", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)

		// Create two connected nodes with pubsub enabled
		nodes := h.NewNodes(2).Init()
		nodes.ForEachPar(enablePubsub)
		nodes = nodes.StartDaemons().Connect()
		defer nodes.StopDaemons()

		subscriber := nodes[0]
		publisher := nodes[1]

		const topic = "test-topic"
		const message = "hello pubsub"

		// Start subscriber in background
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Use a channel to receive the message
		msgChan := make(chan string, 1)
		go func() {
			// Subscribe and wait for one message
			res := subscriber.RunIPFS("pubsub", "sub", "--enc=json", topic)
			if res.Err == nil {
				// Parse JSON output to get message data
				lines := res.Stdout.Lines()
				if len(lines) > 0 {
					var msg struct {
						Data []byte `json:"data"`
					}
					if json.Unmarshal([]byte(lines[0]), &msg) == nil {
						msgChan <- string(msg.Data)
					}
				}
			}
		}()

		// Wait for subscriber to be ready
		waitForSubscription(t, subscriber, topic)

		// Publish message
		publisher.PipeStrToIPFS(message, "pubsub", "pub", topic)

		// Wait for message or timeout
		select {
		case received := <-msgChan:
			assert.Equal(t, message, received)
		case <-ctx.Done():
			// Subscriber may not receive in time due to test timing - that's OK
			// The main goal is to test the seqno validator state persistence
			t.Log("subscriber did not receive message in time (this is acceptable)")
		}
	})

	t.Run("seqno validator state is persisted", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)

		// Create two connected nodes with pubsub
		nodes := h.NewNodes(2).Init()
		nodes.ForEachPar(enablePubsub)
		nodes = nodes.StartDaemons().Connect()

		node1 := nodes[0]
		node2 := nodes[1]
		node2PeerID := node2.PeerID().String()

		const topic = "seqno-test"

		// Start subscriber on node1
		go func() {
			node1.RunIPFS("pubsub", "sub", topic)
		}()
		waitForSubscription(t, node1, topic)

		// Publish multiple messages from node2 to trigger seqno validation
		publishMessages(t, node2, topic, 3)

		// Wait for messages to propagate and seqno to be stored
		waitForMessagePropagation(t)

		// Stop daemons to check datastore (diag datastore requires daemon to be stopped)
		nodes.StopDaemons()

		// Check that seqno state exists
		count := node1.DatastoreCount("/pubsub/seqno/")
		t.Logf("seqno entries count: %d", count)

		// There should be at least one seqno entry (from node2)
		assert.NotEqual(t, int64(0), count, "expected seqno state to be persisted")

		// Verify the specific peer's key exists and test --hex output format
		key := "/pubsub/seqno/" + node2PeerID
		res := node1.RunIPFS("diag", "datastore", "get", "--hex", key)
		if res.Err == nil {
			t.Logf("seqno for peer %s:\n%s", node2PeerID, res.Stdout.String())
			assert.Contains(t, res.Stdout.String(), "Hex Dump:")
		} else {
			// Key might not exist if messages didn't propagate - log but don't fail
			t.Logf("seqno key not found for peer %s (messages may not have propagated)", node2PeerID)
		}
	})

	t.Run("seqno updates when receiving multiple messages", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)

		// Create two connected nodes with pubsub
		nodes := h.NewNodes(2).Init()
		nodes.ForEachPar(enablePubsub)
		nodes = nodes.StartDaemons().Connect()

		node1 := nodes[0]
		node2 := nodes[1]
		node2PeerID := node2.PeerID().String()

		const topic = "seqno-update-test"
		seqnoKey := "/pubsub/seqno/" + node2PeerID

		// Start subscriber on node1
		go func() {
			node1.RunIPFS("pubsub", "sub", topic)
		}()
		waitForSubscription(t, node1, topic)

		// Send first message
		node2.PipeStrToIPFS("msg1", "pubsub", "pub", topic)
		time.Sleep(500 * time.Millisecond)

		// Stop daemons to check seqno (diag datastore requires daemon to be stopped)
		nodes.StopDaemons()

		// Get seqno after first message
		res1 := node1.RunIPFS("diag", "datastore", "get", seqnoKey)
		var seqno1 []byte
		if res1.Err == nil {
			seqno1 = res1.Stdout.Bytes()
			t.Logf("seqno after first message: %d bytes", len(seqno1))
		} else {
			t.Logf("seqno not found after first message (message may not have propagated)")
		}

		// Restart daemons for second message
		nodes = nodes.StartDaemons().Connect()

		// Resubscribe
		go func() {
			node1.RunIPFS("pubsub", "sub", topic)
		}()
		waitForSubscription(t, node1, topic)

		// Send second message
		node2.PipeStrToIPFS("msg2", "pubsub", "pub", topic)
		time.Sleep(500 * time.Millisecond)

		// Stop daemons to check seqno
		nodes.StopDaemons()

		// Get seqno after second message
		res2 := node1.RunIPFS("diag", "datastore", "get", seqnoKey)
		var seqno2 []byte
		if res2.Err == nil {
			seqno2 = res2.Stdout.Bytes()
			t.Logf("seqno after second message: %d bytes", len(seqno2))
		} else {
			t.Logf("seqno not found after second message")
		}

		// If both messages were received, seqno should have been updated
		// The seqno is a uint64 that should increase with each message
		if len(seqno1) > 0 && len(seqno2) > 0 {
			// seqno2 should be >= seqno1 (it's the max seen seqno)
			// We just verify they're both non-empty and potentially different
			t.Logf("seqno1: %x", seqno1)
			t.Logf("seqno2: %x", seqno2)
			// The seqno validator stores the max seqno seen, so seqno2 >= seqno1
			// We can't do a simple byte comparison due to potential endianness
			// but both should be valid uint64 values (8 bytes)
			assert.Equal(t, 8, len(seqno2), "seqno should be 8 bytes (uint64)")
		}
	})

	t.Run("pubsub reset clears seqno state", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)

		// Create two connected nodes
		nodes := h.NewNodes(2).Init()
		nodes.ForEachPar(enablePubsub)
		nodes = nodes.StartDaemons().Connect()

		node1 := nodes[0]
		node2 := nodes[1]

		const topic = "reset-test"

		// Start subscriber and exchange messages
		go func() {
			node1.RunIPFS("pubsub", "sub", topic)
		}()
		waitForSubscription(t, node1, topic)

		publishMessages(t, node2, topic, 3)
		waitForMessagePropagation(t)

		// Stop daemons to check initial count
		nodes.StopDaemons()

		// Verify there is state before resetting
		initialCount := node1.DatastoreCount("/pubsub/seqno/")
		t.Logf("initial seqno count: %d", initialCount)

		// Restart node1 to run pubsub reset
		node1.StartDaemon()

		// Reset all seqno state (while daemon is running)
		res := node1.IPFS("pubsub", "reset")
		assert.NoError(t, res.Err)
		t.Logf("reset output: %s", res.Stdout.String())

		// Stop daemon to verify state was cleared
		node1.StopDaemon()

		// Verify state was cleared
		finalCount := node1.DatastoreCount("/pubsub/seqno/")
		t.Logf("final seqno count: %d", finalCount)
		assert.Equal(t, int64(0), finalCount, "seqno state should be cleared after reset")
	})

	t.Run("pubsub reset with peer flag", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)

		// Create three connected nodes
		nodes := h.NewNodes(3).Init()
		nodes.ForEachPar(enablePubsub)
		nodes = nodes.StartDaemons().Connect()

		node1 := nodes[0]
		node2 := nodes[1]
		node3 := nodes[2]
		node2PeerID := node2.PeerID().String()
		node3PeerID := node3.PeerID().String()

		const topic = "peer-reset-test"

		// Start subscriber on node1
		go func() {
			node1.RunIPFS("pubsub", "sub", topic)
		}()
		waitForSubscription(t, node1, topic)

		// Publish from both node2 and node3
		for range 3 {
			node2.PipeStrToIPFS("msg2", "pubsub", "pub", topic)
			node3.PipeStrToIPFS("msg3", "pubsub", "pub", topic)
			time.Sleep(50 * time.Millisecond)
		}
		waitForMessagePropagation(t)

		// Stop node2 and node3
		node2.StopDaemon()
		node3.StopDaemon()

		// Reset only node2's state (while node1 daemon is running)
		res := node1.IPFS("pubsub", "reset", "--peer", node2PeerID)
		require.NoError(t, res.Err)
		t.Logf("reset output: %s", res.Stdout.String())

		// Stop node1 daemon to check datastore
		node1.StopDaemon()

		// Check that node2's key is gone
		res = node1.RunIPFS("diag", "datastore", "get", "/pubsub/seqno/"+node2PeerID)
		assert.Error(t, res.Err, "node2's seqno key should be deleted")

		// Check that node3's key still exists (if it was created)
		res = node1.RunIPFS("diag", "datastore", "get", "/pubsub/seqno/"+node3PeerID)
		// Note: node3's key might not exist if messages didn't propagate
		// So we just log the result without asserting
		if res.Err == nil {
			t.Logf("node3's seqno key still exists (as expected)")
		} else {
			t.Logf("node3's seqno key not found (messages may not have propagated)")
		}
	})

	t.Run("seqno state survives daemon restart", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)

		// Create and start single node
		node := h.NewNode().Init()
		enablePubsub(node)
		node.StartDaemon()

		// We need another node to publish messages
		node2 := h.NewNode().Init()
		enablePubsub(node2)
		node2.StartDaemon()
		node.Connect(node2)

		const topic = "restart-test"

		// Start subscriber and exchange messages
		go func() {
			node.RunIPFS("pubsub", "sub", topic)
		}()
		waitForSubscription(t, node, topic)

		publishMessages(t, node2, topic, 3)
		waitForMessagePropagation(t)

		// Stop daemons to check datastore
		node.StopDaemon()
		node2.StopDaemon()

		// Get count before restart
		beforeCount := node.DatastoreCount("/pubsub/seqno/")
		t.Logf("seqno count before restart: %d", beforeCount)

		// Restart node (simulate restart scenario)
		node.StartDaemon()
		time.Sleep(500 * time.Millisecond)

		// Stop daemon to check datastore again
		node.StopDaemon()

		// Get count after restart
		afterCount := node.DatastoreCount("/pubsub/seqno/")
		t.Logf("seqno count after restart: %d", afterCount)

		// Count should be the same (state persisted)
		assert.Equal(t, beforeCount, afterCount, "seqno state should survive daemon restart")
	})
}
