package cli

import (
	"bytes"
	"testing"

	"github.com/ipfs/go-test/random"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
)

func TestDHTAutoclient(t *testing.T) {
	t.Parallel()
	nodes := harness.NewT(t).NewNodes(10).Init()
	harness.Nodes(nodes[8:]).ForEachPar(func(node *harness.Node) {
		node.IPFS("config", "Routing.Type", "autoclient")
	})
	nodes.StartDaemons().Connect()

	t.Run("file added on node in client mode is retrievable from node in client mode", func(t *testing.T) {
		t.Parallel()
		randomBytes := random.Bytes(1000)
		randomBytes = append(randomBytes, '\r')
		hash := nodes[8].IPFSAdd(bytes.NewReader(randomBytes))

		res := nodes[9].IPFS("cat", hash)
		assert.Equal(t, randomBytes, []byte(res.Stdout.Trimmed()))
	})

	t.Run("file added on node in server mode is retrievable from all nodes", func(t *testing.T) {
		t.Parallel()
		randomBytes := random.Bytes(1000)
		hash := nodes[0].IPFSAdd(bytes.NewReader(randomBytes))

		for i := 0; i < 10; i++ {
			res := nodes[i].IPFS("cat", hash)
			assert.Equal(t, randomBytes, []byte(res.Stdout.Trimmed()))
		}
	})
}
