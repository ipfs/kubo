package harness

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/ipfs/kubo/config"
)

type Peering struct {
	From int
	To   int
}

func NewRandPort() int {
	n := rand.Int()
	return 3000 + (n % 1000)
}

func CreatePeerNodes(t *testing.T, n int, peerings []Peering) (*Harness, Nodes) {
	h := NewT(t)
	nodes := h.NewNodes(n).Init()
	nodes.ForEachPar(func(node *Node) {
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Routing.Type = config.NewOptionalString("none")
			cfg.Addresses.Swarm = []string{fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", NewRandPort())}
		})

	})

	for _, peering := range peerings {
		nodes[peering.From].PeerWith(nodes[peering.To])
	}

	return h, nodes
}
