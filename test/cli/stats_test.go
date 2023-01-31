package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ipfs/kubo/test/cli/harness"
)

func TestStats(t *testing.T) {
	t.Parallel()

	t.Run("stats dht", func(t *testing.T) {
		t.Parallel()
		nodes := harness.NewT(t).NewNodes(2).Init().StartDaemons().Connect()
		node1 := nodes[0]

		res := node1.IPFS("stats", "dht")
		assert.NoError(t, res.Err)
		assert.Equal(t, 0, len(res.Stderr.Lines()))
		assert.NotEqual(t, 0, len(res.Stdout.Lines()))
	})
}
