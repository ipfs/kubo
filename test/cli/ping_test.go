package cli

import (
	"fmt"
	"testing"

	"github.com/ipfs/go-ipfs/test/cli/harness"
	"github.com/stretchr/testify/assert"
)

func TestPing(t *testing.T) {
	t.Parallel()

	h := harness.NewForTest(t)
	h.Init()
	h.IPTB.Init(2)
	h.IPTB.StartupCluster()

	t.Run("test ping other", func(t *testing.T) {
		h.IPTB.MustRunIPFS(0, "ping", "-n", "2", "--", h.IPTB.PeerID(1))
		h.IPTB.MustRunIPFS(1, "ping", "-n", "2", "--", h.IPTB.PeerID(0))
	})

	t.Run("test ping unreachable peer", func(t *testing.T) {
		badPeer := "QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJx"
		res := h.IPTB.RunIPFS(0, "ping", "-n", "2", "--", badPeer)
		assert.Contains(t, res.Stdout.String(), fmt.Sprintf("Looking up peer %s", badPeer))
		assert.Contains(t, res.Stderr.String(), "Error: peer lookup failed: routing: not found")
	})

	t.Run("test ping self", func(t *testing.T) {
		res := h.IPTB.RunIPFS(0, "ping", "-n", "2", "--", h.IPTB.PeerID(0))
		assert.Equal(t, 1, res.Cmd.ProcessState.ExitCode())
		assert.Contains(t, res.Stderr.String(), "can't ping self")

		res = h.IPTB.RunIPFS(1, "ping", "-n", "2", "--", h.IPTB.PeerID(1))
		assert.Equal(t, 1, res.Cmd.ProcessState.ExitCode())
		assert.Contains(t, res.Stderr.String(), "can't ping self")
	})

	t.Run("test ping 0", func(t *testing.T) {
		res := h.IPTB.RunIPFS(0, "ping", "-n", "0", "--", h.IPTB.PeerID(1))
		assert.Equal(t, 1, res.Cmd.ProcessState.ExitCode())
		assert.Contains(t, res.Stderr.String(), "ping count must be greater than 0")
	})

	t.Run("test ping offline", func(t *testing.T) {
		h.IPTB.StopNode(1)
		res := h.IPTB.RunIPFS(0, "ping", "-n", "2", "--", h.IPTB.PeerID(1))
		assert.Equal(t, 1, res.Cmd.ProcessState.ExitCode())
		assert.Contains(t, res.Stderr.String(), "ping failed")
	})
}
