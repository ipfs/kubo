package harness

import (
	"sync"

	"github.com/multiformats/go-multiaddr"
)

type Cluster struct {
	ClusterRoot string
	IPFSBin     string
	Runner      *Runner

	inited bool
	Nodes  []*Node
	mut    sync.Mutex
}

func (t *Cluster) Init(count int) {
	t.mut.Lock()
	defer t.mut.Unlock()
	if t.inited {
		panic("cannot init cluster until it is stopped first")
	}
	for n := 0; n < count; n++ {
		node := BuildNode(t.IPFSBin, t.ClusterRoot, n)
		node.Init()
		t.Nodes = append(t.Nodes, &node)
	}
	t.inited = true
}

func (t *Cluster) InitSingle() *Node {
	t.Init(1)
	return t.Nodes[0]
}

func (t *Cluster) Run(args ...string) {
	t.mut.Lock()
	defer t.mut.Unlock()
	if !t.inited {
		panic("cannot start an IPTB cluster before it's inited")
	}

	log.Debugf("starting cluster")
	for _, node := range t.Nodes {
		node.Start()
	}
	for i, node := range t.Nodes {
		for j, otherNode := range t.Nodes {
			if i == j {
				continue
			}
			node.Connect(otherNode)
		}
	}

	for _, node := range t.Nodes {
		firstPeer := node.Peers()[0]
		if _, err := firstPeer.ValueForProtocol(multiaddr.P_P2P); err != nil {
			log.Panicf("unexpected state for node %d with peer ID %s: %s", node.ID, node.PeerID(), err)
		}
	}
}

func (t *Cluster) Stop() {
	t.mut.Lock()
	defer t.mut.Unlock()
	if !t.inited {
		return
	}
	for _, node := range t.Nodes {
		node.Stop()
	}
}
