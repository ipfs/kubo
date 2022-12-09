package cli

import (
	"fmt"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
)

func TestPing(t *testing.T) {
	t.Parallel()

	h := harness.NewT(t)
	h.Cluster.Init(2)
	h.Cluster.Run()
	node1 := h.Cluster.Nodes[0]
	node2 := h.Cluster.Nodes[1]

	t.Run("other", func(t *testing.T) {
		node1.IPFS("ping", "-n", "2", "--", node2.PeerID().String())
		node2.IPFS("ping", "-n", "2", "--", node1.PeerID().String())
	})

	t.Run("ping unreachable peer", func(t *testing.T) {
		badPeer := "QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJx"
		res := node1.RunIPFS("ping", "-n", "2", "--", badPeer)
		assert.Contains(t, res.Stdout.String(), fmt.Sprintf("Looking up peer %s", badPeer))
		assert.Contains(t, res.Stderr.String(), "Error: ping failed")
	})

	t.Run("self", func(t *testing.T) {
		res := node1.RunIPFS("ping", "-n", "2", "--", node1.PeerID().String())
		assert.Equal(t, 1, res.Cmd.ProcessState.ExitCode())
		assert.Contains(t, res.Stderr.String(), "can't ping self")

		res = node2.RunIPFS("ping", "-n", "2", "--", node2.PeerID().String())
		assert.Equal(t, 1, res.Cmd.ProcessState.ExitCode())
		assert.Contains(t, res.Stderr.String(), "can't ping self")
	})

	t.Run("0", func(t *testing.T) {
		res := node1.RunIPFS("ping", "-n", "0", "--", node2.PeerID().String())
		assert.Equal(t, 1, res.Cmd.ProcessState.ExitCode())
		assert.Contains(t, res.Stderr.String(), "ping count must be greater than 0")
	})

	t.Run("offline", func(t *testing.T) {
		node2.Stop()
		res := node1.RunIPFS("ping", "-n", "2", "--", node2.PeerID().String())
		assert.Equal(t, 1, res.Cmd.ProcessState.ExitCode())
		assert.Contains(t, res.Stderr.String(), "ping failed")
	})
}
