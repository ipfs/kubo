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

// Ports handed out by NewRandPort come from this range. It sits below the
// ephemeral range every platform we test on uses (Linux starts at 32768, macOS
// and Windows at 49152), so the kernel never assigns one of these as the source
// port of an outgoing connection. Without that, a port we probed and released
// can be taken by any of the many connections the daemons under test open,
// which used to surface as "bind: address already in use".
const (
	reservedRangeStart = 10000
	reservedRangeEnd   = 32000
)

var (
	allocatedPorts = make(map[int]struct{})
	portMutex      sync.Mutex
)

// NewTCPListener binds a listener on a free loopback port and returns it along
// with the port number. Prefer this over NewRandPort whenever the test itself
// is the one listening: the socket stays bound the whole time, so nothing can
// take the port between picking it and using it.
func NewTCPListener(t *testing.T) (net.Listener, int) {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocating a loopback listener: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port

	portMutex.Lock()
	allocatedPorts[port] = struct{}{}
	portMutex.Unlock()

	t.Cleanup(func() { _ = l.Close() })
	return l, port
}

// NewRandPort reserves a port for something else to bind later, typically an
// ipfs daemon we spawn. Because the port is only probed and then released,
// there is an unavoidable window where another process could take it; ports are
// picked from below the ephemeral range so that at least outgoing connections
// on a busy machine cannot land on one.
//
// When the test binds the port itself, use NewTCPListener instead.
func NewRandPort() int {
	portMutex.Lock()
	defer portMutex.Unlock()

	for range 100 {
		port := reservedRangeStart + rand.Intn(reservedRangeEnd-reservedRangeStart)
		if _, used := allocatedPorts[port]; used {
			continue
		}

		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			continue // in use by something outside this test binary
		}
		l.Close()

		allocatedPorts[port] = struct{}{}
		return port
	}

	// Every candidate was either taken or already handed out. Give up on
	// probing and return anything we have not used yet.
	for range 1000 {
		port := reservedRangeStart + rand.Intn(reservedRangeEnd-reservedRangeStart)
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
