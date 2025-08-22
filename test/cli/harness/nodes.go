package harness

import (
	"fmt"
	"sync"
	"time"

	. "github.com/ipfs/kubo/test/cli/testutils"
	"github.com/multiformats/go-multiaddr"
	"golang.org/x/sync/errgroup"
)

// Nodes is a collection of Kubo nodes along with operations on groups of nodes.
type Nodes []*Node

func (n Nodes) Init(args ...string) Nodes {
	ForEachPar(n, func(node *Node) { node.Init(args...) })
	return n
}

func (n Nodes) ForEachPar(f func(*Node)) {
	group := &errgroup.Group{}
	for _, node := range n {
		node := node
		group.Go(func() error {
			f(node)
			return nil
		})
	}
	err := group.Wait()
	if err != nil {
		panic(err)
	}
}

// Connect establishes connections between all nodes in the collection
func (n Nodes) Connect() Nodes {
	// Use a 10 second timeout - more generous than the original 1 second,
	// but reasonable for test environments
	const timeout = 10 * time.Second
	if len(n) < 2 {
		return n // Nothing to connect
	}

	// Use errgroup for better concurrent error handling
	group := &errgroup.Group{}

	// Track connection errors
	type connError struct {
		from, to int
		err      error
	}
	var mu sync.Mutex
	var errors []connError

	// Establish all connections concurrently
	for i, node := range n {
		for j, otherNode := range n {
			if i == j {
				continue
			}
			// Capture loop variables
			fromNode, toNode := node, otherNode
			fromIdx, toIdx := i, j

			group.Go(func() error {
				// Use ConnectAndWait for robust connection with timeout
				if err := fromNode.ConnectAndWait(toNode, timeout); err != nil {
					mu.Lock()
					errors = append(errors, connError{from: fromIdx, to: toIdx, err: err})
					mu.Unlock()
					// Don't return error - collect all failures first
				}
				return nil
			})
		}
	}

	// Wait for all connection attempts
	_ = group.Wait() // We handle errors separately

	// Report any connection failures
	if len(errors) > 0 {
		errMsg := fmt.Sprintf("failed to establish %d connections:\n", len(errors))
		for _, e := range errors {
			errMsg += fmt.Sprintf("  - node %d -> node %d: %v\n", e.from, e.to, e.err)
		}
		log.Panicf(errMsg)
	}

	// Verify all nodes have expected connections
	if err := n.verifyAllConnected(); err != nil {
		log.Panicf("connection verification failed: %v", err)
	}

	return n
}

// verifyAllConnected checks that all nodes are properly connected
func (n Nodes) verifyAllConnected() error {
	expectedPeers := len(n) - 1

	for _, node := range n {
		peers := node.Peers()

		if len(peers) < expectedPeers {
			return fmt.Errorf("node %d (peer %s) has only %d peers, expected at least %d",
				node.ID, node.PeerID(), len(peers), expectedPeers)
		}

		// Verify each peer has valid P2P protocol
		for i, peer := range peers {
			if _, err := peer.ValueForProtocol(multiaddr.P_P2P); err != nil {
				return fmt.Errorf("node %d peer %d has invalid protocol: %v",
					node.ID, i, err)
			}
		}
	}

	return nil
}

func (n Nodes) StartDaemons(args ...string) Nodes {
	ForEachPar(n, func(node *Node) { node.StartDaemon(args...) })
	return n
}

func (n Nodes) StopDaemons() Nodes {
	ForEachPar(n, func(node *Node) { node.StopDaemon() })
	return n
}
