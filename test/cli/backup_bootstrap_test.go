package cli

import (
	"fmt"
	"testing"
	"time"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
)

func TestBackupBootstrapPeers(t *testing.T) {
	nodes := harness.NewT(t).NewNodes(3).Init()
	nodes.ForEachPar(func(n *harness.Node) {
		n.UpdateConfig(func(cfg *config.Config) {
			cfg.Bootstrap = []string{}
			cfg.Addresses.Swarm = []string{fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", harness.NewRandPort())}
			cfg.Discovery.MDNS.Enabled = false
			cfg.Internal.BackupBootstrapInterval = config.NewOptionalDuration(250 * time.Millisecond)
		})
	})

	// Start all nodes and ensure they all have no peers.
	nodes.StartDaemons()
	nodes.ForEachPar(func(n *harness.Node) {
		assert.Len(t, n.Peers(), 0)
	})

	// Connect nodes 0 and 1, ensure they know each other.
	nodes[0].Connect(nodes[1])
	assert.Len(t, nodes[0].Peers(), 1)
	assert.Len(t, nodes[1].Peers(), 1)
	assert.Len(t, nodes[2].Peers(), 0)

	// Wait a bit to ensure that 0 and 1 saved their temporary bootstrap backups.
	time.Sleep(time.Millisecond * 500)
	nodes.StopDaemons()

	// Start 1 and 2. 2 does not know anyone yet.
	nodes[1].StartDaemon()
	nodes[2].StartDaemon()
	assert.Len(t, nodes[1].Peers(), 0)
	assert.Len(t, nodes[2].Peers(), 0)

	// Connect 1 and 2, ensure they know each other.
	nodes[1].Connect(nodes[2])
	assert.Len(t, nodes[1].Peers(), 1)
	assert.Len(t, nodes[2].Peers(), 1)

	// Start 0, wait a bit. Should connect to 1, and then discover 2 via the
	// backup bootstrap peers.
	nodes[0].StartDaemon()
	time.Sleep(time.Millisecond * 500)

	// Check if they're all connected.
	assert.Len(t, nodes[0].Peers(), 2)
	assert.Len(t, nodes[1].Peers(), 2)
	assert.Len(t, nodes[2].Peers(), 2)
}
