package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/ipfs/kubo/test/cli/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMultipleNodes(t *testing.T) {
	t.Parallel()

	checkSingleFile := func(nodes harness.Nodes) {
		s := testutils.RandomStr(100)
		hash := nodes[0].IPFSAddStr(s)
		nodes.ForEachPar(func(n *harness.Node) {
			val := n.IPFS("cat", hash).Stdout.Trimmed()
			assert.Equal(t, s, val)
		})
	}
	checkRandomDir := func(nodes harness.Nodes) {
		randDir := filepath.Join(nodes[0].Dir, "foobar")
		require.NoError(t, os.Mkdir(randDir, 0777))
		rf := testutils.NewRandFiles()
		rf.FanoutDirs = 3
		rf.FanoutFiles = 6
		require.NoError(t, rf.WriteRandomFiles(randDir, 4))

		hash := nodes[2].IPFS("add", "-r", "-Q", randDir).Stdout.Trimmed()
		nodes.ForEachPar(func(n *harness.Node) {
			res := n.RunIPFS("refs", "-r", hash)
			assert.Equal(t, 0, res.ExitCode())
		})
	}

	t.Run("vanilla nodes", func(t *testing.T) {
		t.Parallel()
		nodes := harness.NewT(t).NewNodes(5).Init().StartDaemons().Connect()
		checkSingleFile(nodes)
		checkRandomDir(nodes)
	})

	t.Run("nodes with yamux disabled", func(t *testing.T) {
		t.Parallel()
		nodes := harness.NewT(t).NewNodes(5).Init()
		nodes.ForEachPar(func(n *harness.Node) {
			n.UpdateConfig(func(cfg *config.Config) {
				cfg.Swarm.Transports.Multiplexers.Yamux = config.Disabled
			})
		})
		nodes.StartDaemons().Connect()
		checkSingleFile(nodes)
		checkRandomDir(nodes)
	})
}
