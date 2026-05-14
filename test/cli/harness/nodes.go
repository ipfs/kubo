package harness

import (
	"sync"

	. "github.com/ipfs/kubo/test/cli/testutils"
	"github.com/multiformats/go-multiaddr"
)

// Nodes is a collection of Kubo nodes along with operations on groups of nodes.
type Nodes []*Node

func (n Nodes) Init(args ...string) Nodes {
	ForEachPar(n, func(node *Node) { node.Init(args...) })
	return n
}

func (n Nodes) ForEachPar(f func(*Node)) {
	var wg sync.WaitGroup
	for _, node := range n {
		wg.Add(1)
		node := node
		go func() {
			defer wg.Done()
			f(node)
		}()
	}
	wg.Wait()
}

func (n Nodes) Connect() Nodes {
	for i, node := range n {
		for j, otherNode := range n {
			if i == j {
				continue
			}
			// Do not connect in parallel, because that can cause TLS handshake problems on some platforms.
			node.Connect(otherNode)
		}
	}
	for _, node := range n {
		firstPeer := node.Peers()[0]
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
