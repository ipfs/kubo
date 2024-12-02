package cli

import (
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/ipfs/kubo/test/cli/testutils"
	"github.com/stretchr/testify/assert"
)

const (
	fixtureFile    = "./fixtures/TestDagStat.car"
	textOutputPath = "./fixtures/TestDagStatExpectedOutput.txt"
	node1Cid       = "bafyreibmdfd7c5db4kls4ty57zljfhqv36gi43l6txl44pi423wwmeskwy"
	node2Cid       = "bafyreie3njilzdi4ixumru4nzgecsnjtu7fzfcwhg7e6s4s5i7cnbslvn4"
	fixtureCid     = "bafyreifrm6uf5o4dsaacuszf35zhibyojlqclabzrms7iak67pf62jygaq"
)

type DagStat struct {
	Cid       string `json:"Cid"`
	Size      int    `json:"Size"`
	NumBlocks int    `json:"NumBlocks"`
}

type Data struct {
	UniqueBlocks int       `json:"UniqueBlocks"`
	TotalSize    int       `json:"TotalSize"`
	SharedSize   int       `json:"SharedSize"`
	Ratio        float64   `json:"Ratio"`
	DagStats     []DagStat `json:"DagStats"`
}

// The Fixture file represents a dag where 2 nodes of size = 46B each, have a common child of 7B
// when traversing the DAG from the root's children (node1 and node2) we count (46 + 7)x2 bytes (counting redundant bytes) = 106
// since both nodes share a common child of 7 bytes we actually had to read (46)x2 + 7 =  99 bytes
// we should get a dedup ratio of 106/99 that results in approximately 1.0707071

func TestDag(t *testing.T) {
	t.Parallel()

	t.Run("ipfs dag stat --enc=json", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		// Import fixture
		r, err := os.Open(fixtureFile)
		assert.Nil(t, err)
		defer r.Close()
		err = node.IPFSDagImport(r, fixtureCid)
		assert.NoError(t, err)
		stat := node.RunIPFS("dag", "stat", "--progress=false", "--enc=json", node1Cid, node2Cid)
		var data Data
		err = json.Unmarshal(stat.Stdout.Bytes(), &data)
		assert.NoError(t, err)

		expectedUniqueBlocks := 3
		expectedSharedSize := 7
		expectedTotalSize := 99
		expectedRatio := float64(expectedSharedSize+expectedTotalSize) / float64(expectedTotalSize)
		expectedDagStatsLength := 2
		// Validate UniqueBlocks
		assert.Equal(t, expectedUniqueBlocks, data.UniqueBlocks)
		assert.Equal(t, expectedSharedSize, data.SharedSize)
		assert.Equal(t, expectedTotalSize, data.TotalSize)
		assert.Equal(t, testutils.FloatTruncate(expectedRatio, 4), testutils.FloatTruncate(data.Ratio, 4))

		// Validate DagStats
		assert.Equal(t, expectedDagStatsLength, len(data.DagStats))
		node1Output := data.DagStats[0]
		node2Output := data.DagStats[1]

		assert.Equal(t, node1Output.Cid, node1Cid)
		assert.Equal(t, node2Output.Cid, node2Cid)

		expectedNode1Size := (expectedTotalSize + expectedSharedSize) / 2
		expectedNode2Size := (expectedTotalSize + expectedSharedSize) / 2
		assert.Equal(t, expectedNode1Size, node1Output.Size)
		assert.Equal(t, expectedNode2Size, node2Output.Size)

		expectedNode1Blocks := 2
		expectedNode2Blocks := 2
		assert.Equal(t, expectedNode1Blocks, node1Output.NumBlocks)
		assert.Equal(t, expectedNode2Blocks, node2Output.NumBlocks)
	})

	t.Run("ipfs dag stat", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		r, err := os.Open(fixtureFile)
		assert.NoError(t, err)
		defer r.Close()
		f, err := os.Open(textOutputPath)
		assert.NoError(t, err)
		defer f.Close()
		content, err := io.ReadAll(f)
		assert.NoError(t, err)
		err = node.IPFSDagImport(r, fixtureCid)
		assert.NoError(t, err)
		stat := node.RunIPFS("dag", "stat", "--progress=false", node1Cid, node2Cid)
		assert.Equal(t, content, stat.Stdout.Bytes())
	})
}
