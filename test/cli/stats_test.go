package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ipfs/kubo/test/cli/harness"
)

func TestStats(t *testing.T) {
	t.Parallel()

	t.Run("stats bw rejects non-positive intervals", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		for _, interval := range []string{"0s", "-1s"} {
			res := node.RunIPFS("stats", "bw", "--poll", "--interval="+interval)
			assert.Equal(t, 1, res.ExitCode(), "interval %s should be rejected", interval)
			assert.Contains(t, res.Stderr.Trimmed(), "interval must be greater than zero")
		}
	})

	t.Run("stats dht", func(t *testing.T) {
		t.Parallel()
		nodes := harness.NewT(t).NewNodes(2).Init().StartDaemons().Connect()
		defer nodes.StopDaemons()
		node1 := nodes[0]

		res := node1.IPFS("stats", "dht")
		assert.NoError(t, res.Err)
		assert.Equal(t, 0, len(res.Stderr.Lines()))
		assert.NotEqual(t, 0, len(res.Stdout.Lines()))
	})
}
