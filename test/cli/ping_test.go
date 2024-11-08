package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
)

func TestPing(t *testing.T) {
	t.Parallel()

	t.Run("other", func(t *testing.T) {
		t.Parallel()
		nodes := harness.NewT(t).NewNodes(2).Init().StartDaemons().Connect()
		node1 := nodes[0]
		node2 := nodes[1]

		node1.IPFS("ping", "-n", "2", "--", node2.PeerID().String())
		node2.IPFS("ping", "-n", "2", "--", node1.PeerID().String())
	})

	t.Run("ping unreachable peer", func(t *testing.T) {
		t.Parallel()
		nodes := harness.NewT(t).NewNodes(2).Init().StartDaemons().Connect()
		node1 := nodes[0]

		badPeer := "QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJx"
		res := node1.RunIPFS("ping", "-n", "2", "--", badPeer)
		assert.Contains(t, res.Stdout.String(), fmt.Sprintf("Looking up peer %s", badPeer))
		msg := res.Stderr.String()
		assert.Truef(t, strings.HasPrefix(msg, "Error:"), "should fail got this instead: %q", msg)
	})

	t.Run("self", func(t *testing.T) {
		t.Parallel()
		nodes := harness.NewT(t).NewNodes(2).Init().StartDaemons()
		node1 := nodes[0]
		node2 := nodes[1]

		res := node1.RunIPFS("ping", "-n", "2", "--", node1.PeerID().String())
		assert.Equal(t, 1, res.Cmd.ProcessState.ExitCode())
		assert.Contains(t, res.Stderr.String(), "can't ping self")

		res = node2.RunIPFS("ping", "-n", "2", "--", node2.PeerID().String())
		assert.Equal(t, 1, res.Cmd.ProcessState.ExitCode())
		assert.Contains(t, res.Stderr.String(), "can't ping self")
	})

	t.Run("0", func(t *testing.T) {
		t.Parallel()
		nodes := harness.NewT(t).NewNodes(2).Init().StartDaemons().Connect()
		node1 := nodes[0]
		node2 := nodes[1]

		res := node1.RunIPFS("ping", "-n", "0", "--", node2.PeerID().String())
		assert.Equal(t, 1, res.Cmd.ProcessState.ExitCode())
		assert.Contains(t, res.Stderr.String(), "ping count must be greater than 0")
	})

	t.Run("offline", func(t *testing.T) {
		t.Parallel()
		nodes := harness.NewT(t).NewNodes(2).Init().StartDaemons().Connect()
		node1 := nodes[0]
		node2 := nodes[1]

		node2.StopDaemon()

		res := node1.RunIPFS("ping", "-n", "2", "--", node2.PeerID().String())
		assert.Equal(t, 1, res.Cmd.ProcessState.ExitCode())
		assert.Contains(t, res.Stderr.String(), "ping failed")
	})
}
