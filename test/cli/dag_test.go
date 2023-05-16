package cli

import (
	"os"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
)

const (
	fixtureFile = "./fixtures/TestDagStat.car"
	node1Cid    = "bafyreibmdfd7c5db4kls4ty57zljfhqv36gi43l6txl44pi423wwmeskwy"
	node2Cid    = "bafyreie3njilzdi4ixumru4nzgecsnjtu7fzfcwhg7e6s4s5i7cnbslvn4"
	fixtureCid  = "bafyreifrm6uf5o4dsaacuszf35zhibyojlqclabzrms7iak67pf62jygaq"
)

// The Fixture file represents a dag where 2 nodes of size = 46B each, have a common child of 53B
// when traversing the DAG from the root's children (node1 and node2) we count (46 + 53)x2 bytes (counting redundant bytes) = 198
// since both nodes share a common child of 53 bytes we actually had to read (46)x2 + 53 =  145 bytes
// we should get a dedup ratio of 198/145 that results in approximatelly 1.3655173

func TestDag(t *testing.T) {
	t.Parallel()
	t.Run("ipfs dag stat", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		// Import fixture
		r, err := os.Open(fixtureFile)
		assert.Nil(t, err)
		defer r.Close()
		err = node.IPFSDagImport(r, fixtureCid)
		assert.Nil(t, err)
		stat := node.RunIPFS("dag", "stat", "--progress=false", node1Cid, node2Cid)
		str := stat.Stdout.String()
		expected := "\nCID                                           \tBlocks         \tSize\nbafyreibmdfd7c5db4kls4ty57zljfhqv36gi43l6txl44pi423wwmeskwy\t2              \t53\nbafyreie3njilzdi4ixumru4nzgecsnjtu7fzfcwhg7e6s4s5i7cnbslvn4\t2              \t53\n\nSummary\nTotal Size: 145\nUnique Blocks: 3\nShared Size: 53\nRatio: 1.365517\n\n\n"
		assert.Equal(t, expected, str)
	})
}
