package harness

import (
	"fmt"
	"math/rand"
	"net"
	"sync"
	"testing"

	"github.com/ipfs/kubo/config"
)

type Peering struct {
	From int
	To   int
}

var (
	allocatedPorts = make(map[int]struct{})
	portMutex      sync.Mutex
)

func NewRandPort() int {
	portMutex.Lock()
	defer portMutex.Unlock()

	for i := 0; i < 100; i++ {
		l, err := net.Listen("tcp", "localhost:0")
		if err != nil {
			continue
		}
		port := l.Addr().(*net.TCPAddr).Port
		l.Close()

		if _, used := allocatedPorts[port]; !used {
			allocatedPorts[port] = struct{}{}
			return port
		}
	}

	// Fallback to random port if we can't get a unique one from the OS
	for i := 0; i < 1000; i++ {
		port := 30000 + rand.Intn(10000)
		if _, used := allocatedPorts[port]; !used {
			allocatedPorts[port] = struct{}{}
			return port
		}
	}

	panic("failed to allocate unique port after 1100 attempts")
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
