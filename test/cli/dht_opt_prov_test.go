package cli

import (
	"testing"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/ipfs/kubo/test/cli/testutils"
	"github.com/stretchr/testify/assert"
)

func TestDHTOptimisticProvide(t *testing.T) {
	t.Parallel()

	t.Run("optimistic provide smoke test", func(t *testing.T) {
		nodes := harness.NewT(t).NewNodes(2).Init()

		nodes[0].UpdateConfig(func(cfg *config.Config) {
			cfg.Experimental.OptimisticProvide = true
		})

		nodes.StartDaemons().Connect()

		hash := nodes[0].IPFSAddStr(testutils.RandomStr(100))
		nodes[0].IPFS("routing", "provide", hash)

		res := nodes[1].IPFS("routing", "findprovs", "--num-providers=1", hash)
		assert.Equal(t, nodes[0].PeerID().String(), res.Stdout.Trimmed())
	})
}
