package harness

import (
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

func (n Nodes) Connect() Nodes {
	wg := sync.WaitGroup{}
	for i, node := range n {
		for j, otherNode := range n {
			if i == j {
				continue
			}
			node := node
			otherNode := otherNode
			wg.Add(1)
			go func() {
				defer wg.Done()
				node.Connect(otherNode)
			}()
		}
	}
	wg.Wait()
	
	// Wait for connections to be established with retry and timeout
	const maxRetries = 50
	const retryDelay = 100 * time.Millisecond
	
	for _, node := range n {
		// Retry until we have at least one peer or timeout
		var peers []multiaddr.Multiaddr
		for i := 0; i < maxRetries; i++ {
			peers = node.Peers()
			if len(peers) > 0 {
				break
			}
			time.Sleep(retryDelay)
		}
		
		if len(peers) == 0 {
			log.Panicf("node %d with peer ID %s has no peers after connection timeout", node.ID, node.PeerID())
		}
		
		firstPeer := peers[0]
		if _, err := firstPeer.ValueForProtocol(multiaddr.P_P2P); err != nil {
			log.Panicf("unexpected state for node %d with peer ID %s: %s", node.ID, node.PeerID(), err)
		}
	}
	return n
}

func (n Nodes) StartDaemons(args ...string) Nodes {
	ForEachPar(n, func(node *Node) { node.StartDaemon(args...) })
	return n
}

func (n Nodes) StopDaemons() Nodes {
	ForEachPar(n, func(node *Node) { node.StopDaemon() })
	return n
}
