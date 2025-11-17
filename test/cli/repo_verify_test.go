package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Well-known block file names in flatfs blockstore that should not be corrupted during testing.
// Flatfs stores each block as a separate .data file on disk.
const (
	// emptyFileFlatfsFilename is the flatfs filename for an empty UnixFS file block
	emptyFileFlatfsFilename = "CIQL7TG2PB52XIZLLHDYIUFMHUQLMMZWBNBZSLDXFCPZ5VDNQQ2WDZQ"
	// emptyDirFlatfsFilename is the flatfs filename for an empty UnixFS directory block.
	// This block has special handling and may be served from memory even when corrupted on disk.
	emptyDirFlatfsFilename = "CIQFTFEEHEDF6KLBT32BFAGLXEZL4UWFNWM4LFTLMXQBCERZ6CMLX3Y"
)

// getEligibleFlatfsBlockFiles returns flatfs block files (*.data) that are safe to corrupt in tests.
// Filters out well-known blocks (empty file/dir) that cause test flakiness.
//
// Note: This helper is specific to the flatfs blockstore implementation where each block
// is stored as a separate file on disk under blocks/*/*.data.
func getEligibleFlatfsBlockFiles(t *testing.T, node *harness.Node) []string {
	blockFiles, err := filepath.Glob(filepath.Join(node.Dir, "blocks", "*", "*.data"))
	require.NoError(t, err)
	require.NotEmpty(t, blockFiles, "no flatfs block files found")

	var eligible []string
	for _, f := range blockFiles {
		name := filepath.Base(f)
		if !strings.Contains(name, emptyFileFlatfsFilename) &&
			!strings.Contains(name, emptyDirFlatfsFilename) {
			eligible = append(eligible, f)
		}
	}
	return eligible
}

// corruptRandomBlock corrupts a random block file in the flatfs blockstore.
// Returns the path to the corrupted file.
func corruptRandomBlock(t *testing.T, node *harness.Node) string {
	eligible := getEligibleFlatfsBlockFiles(t, node)
	require.NotEmpty(t, eligible, "no eligible blocks to corrupt")

	toCorrupt := eligible[0]
	err := os.WriteFile(toCorrupt, []byte("corrupted data"), 0644)
	require.NoError(t, err)

	return toCorrupt
}

// corruptMultipleBlocks corrupts multiple block files in the flatfs blockstore.
// Returns the paths to the corrupted files.
func corruptMultipleBlocks(t *testing.T, node *harness.Node, count int) []string {
	eligible := getEligibleFlatfsBlockFiles(t, node)
	require.GreaterOrEqual(t, len(eligible), count, "not enough eligible blocks to corrupt")

	var corrupted []string
	for i := 0; i < count && i < len(eligible); i++ {
		err := os.WriteFile(eligible[i], []byte(fmt.Sprintf("corrupted data %d", i)), 0644)
		require.NoError(t, err)
		corrupted = append(corrupted, eligible[i])
	}

	return corrupted
}

func TestRepoVerify(t *testing.T) {
	t.Run("healthy repo passes", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.IPFS("add", "-q", "--raw-leaves=false", "-r", node.IPFSBin)

		res := node.IPFS("repo", "verify")
		assert.Contains(t, res.Stdout.String(), "all blocks validated")
	})

	t.Run("detects corruption", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.IPFSAddStr("test content")

		corruptRandomBlock(t, node)

		res := node.RunIPFS("repo", "verify")
		assert.Equal(t, 1, res.ExitCode())
		assert.Contains(t, res.Stdout.String(), "was corrupt")
		assert.Contains(t, res.Stderr.String(), "1 blocks corrupt")
	})

	t.Run("drop removes corrupt blocks", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		cid := node.IPFSAddStr("test content")

		corruptRandomBlock(t, node)

		res := node.RunIPFS("repo", "verify", "--drop")
		assert.Equal(t, 0, res.ExitCode(), "should exit 0 when all corrupt blocks removed successfully")
		output := res.Stdout.String()
		assert.Contains(t, output, "1 blocks corrupt")
		assert.Contains(t, output, "1 removed")

		// Verify block is gone
		res = node.RunIPFS("block", "stat", cid)
		assert.NotEqual(t, 0, res.ExitCode())
	})

	t.Run("heal requires online mode", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.IPFSAddStr("test content")

		corruptRandomBlock(t, node)

		res := node.RunIPFS("repo", "verify", "--heal")
		assert.NotEqual(t, 0, res.ExitCode())
		assert.Contains(t, res.Stderr.String(), "online mode")
	})

	t.Run("heal repairs from network", func(t *testing.T) {
		t.Parallel()
		nodes := harness.NewT(t).NewNodes(2).Init()
		nodes.StartDaemons().Connect()
		defer nodes.StopDaemons()

		// Add content to node 0
		cid := nodes[0].IPFSAddStr("test content for healing")

		// Wait for it to appear on node 1
		nodes[1].IPFS("block", "get", cid)

		// Corrupt on node 1
		corruptRandomBlock(t, nodes[1])

		// Heal should restore from node 0
		res := nodes[1].RunIPFS("repo", "verify", "--heal")
		assert.Equal(t, 0, res.ExitCode(), "should exit 0 when all corrupt blocks healed successfully")
		output := res.Stdout.String()

		// Should report corruption and healing with specific counts
		assert.Contains(t, output, "1 blocks corrupt")
		assert.Contains(t, output, "1 removed")
		assert.Contains(t, output, "1 healed")

		// Verify block is restored
		nodes[1].IPFS("block", "stat", cid)
	})

	t.Run("healed blocks contain correct data", func(t *testing.T) {
		t.Parallel()
		nodes := harness.NewT(t).NewNodes(2).Init()
		nodes.StartDaemons().Connect()
		defer nodes.StopDaemons()

		// Add specific content to node 0
		testContent := "this is the exact content that should be healed correctly"
		cid := nodes[0].IPFSAddStr(testContent)

		// Fetch to node 1 and verify the content is correct initially
		nodes[1].IPFS("block", "get", cid)
		res := nodes[1].IPFS("cat", cid)
		assert.Equal(t, testContent, res.Stdout.String())

		// Corrupt on node 1
		corruptRandomBlock(t, nodes[1])

		// Heal the corruption
		res = nodes[1].RunIPFS("repo", "verify", "--heal")
		assert.Equal(t, 0, res.ExitCode(), "should exit 0 when all corrupt blocks healed successfully")
		output := res.Stdout.String()
		assert.Contains(t, output, "1 blocks corrupt")
		assert.Contains(t, output, "1 removed")
		assert.Contains(t, output, "1 healed")

		// Verify the healed content matches the original exactly
		res = nodes[1].IPFS("cat", cid)
		assert.Equal(t, testContent, res.Stdout.String(), "healed content should match original")

		// Also verify via block get that the raw block data is correct
		block0 := nodes[0].IPFS("block", "get", cid)
		block1 := nodes[1].IPFS("block", "get", cid)
		assert.Equal(t, block0.Stdout.String(), block1.Stdout.String(), "raw block data should match")
	})

	t.Run("multiple corrupt blocks", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()

		// Create 20 blocks
		for i := 0; i < 20; i++ {
			node.IPFSAddStr(strings.Repeat("test content ", i+1))
		}

		// Corrupt 5 blocks
		corruptMultipleBlocks(t, node, 5)

		// Verify detects all corruptions
		res := node.RunIPFS("repo", "verify")
		assert.Equal(t, 1, res.ExitCode())
		// Error summary is in stderr
		assert.Contains(t, res.Stderr.String(), "5 blocks corrupt")

		// Test with --drop
		res = node.RunIPFS("repo", "verify", "--drop")
		assert.Equal(t, 0, res.ExitCode(), "should exit 0 when all corrupt blocks removed successfully")
		assert.Contains(t, res.Stdout.String(), "5 blocks corrupt")
		assert.Contains(t, res.Stdout.String(), "5 removed")
	})

	t.Run("empty repository", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()

		// Verify empty repo passes
		res := node.IPFS("repo", "verify")
		assert.Equal(t, 0, res.ExitCode())
		assert.Contains(t, res.Stdout.String(), "all blocks validated")

		// Should work with --drop and --heal too
		res = node.IPFS("repo", "verify", "--drop")
		assert.Equal(t, 0, res.ExitCode())
		assert.Contains(t, res.Stdout.String(), "all blocks validated")
	})

	t.Run("partial heal success", func(t *testing.T) {
		t.Parallel()
		nodes := harness.NewT(t).NewNodes(2).Init()

		// Start both nodes and connect them
		nodes.StartDaemons().Connect()
		defer nodes.StopDaemons()

		// Add 5 blocks to node 0, pin them to keep available
		cid1 := nodes[0].IPFSAddStr("content available for healing 1")
		cid2 := nodes[0].IPFSAddStr("content available for healing 2")
		cid3 := nodes[0].IPFSAddStr("content available for healing 3")
		cid4 := nodes[0].IPFSAddStr("content available for healing 4")
		cid5 := nodes[0].IPFSAddStr("content available for healing 5")

		// Pin these on node 0 to ensure they stay available
		nodes[0].IPFS("pin", "add", cid1)
		nodes[0].IPFS("pin", "add", cid2)
		nodes[0].IPFS("pin", "add", cid3)
		nodes[0].IPFS("pin", "add", cid4)
		nodes[0].IPFS("pin", "add", cid5)

		// Node 1 fetches these blocks
		nodes[1].IPFS("block", "get", cid1)
		nodes[1].IPFS("block", "get", cid2)
		nodes[1].IPFS("block", "get", cid3)
		nodes[1].IPFS("block", "get", cid4)
		nodes[1].IPFS("block", "get", cid5)

		// Now remove some blocks from node 0 to simulate partial availability
		nodes[0].IPFS("pin", "rm", cid3)
		nodes[0].IPFS("pin", "rm", cid4)
		nodes[0].IPFS("pin", "rm", cid5)
		nodes[0].IPFS("repo", "gc")

		// Verify node 1 is still connected
		peers := nodes[1].IPFS("swarm", "peers")
		require.Contains(t, peers.Stdout.String(), nodes[0].PeerID().String())

		// Corrupt 5 blocks on node 1
		corruptMultipleBlocks(t, nodes[1], 5)

		// Heal should partially succeed (only cid1 and cid2 available from node 0)
		res := nodes[1].RunIPFS("repo", "verify", "--heal")
		assert.Equal(t, 1, res.ExitCode())

		// Should show mixed results with specific counts in stderr
		errOutput := res.Stderr.String()
		assert.Contains(t, errOutput, "5 blocks corrupt")
		assert.Contains(t, errOutput, "5 removed")
		// Only cid1 and cid2 are available for healing, cid3-5 were GC'd
		assert.Contains(t, errOutput, "2 healed")
		assert.Contains(t, errOutput, "3 failed to heal")
	})

	t.Run("heal with block not available on network", func(t *testing.T) {
		t.Parallel()
		nodes := harness.NewT(t).NewNodes(2).Init()

		// Start both nodes and connect
		nodes.StartDaemons().Connect()
		defer nodes.StopDaemons()

		// Add unique content only to node 1
		nodes[1].IPFSAddStr("unique content that exists nowhere else")

		// Ensure nodes are connected
		peers := nodes[1].IPFS("swarm", "peers")
		require.Contains(t, peers.Stdout.String(), nodes[0].PeerID().String())

		// Corrupt the block on node 1
		corruptRandomBlock(t, nodes[1])

		// Heal should fail - node 0 doesn't have this content
		res := nodes[1].RunIPFS("repo", "verify", "--heal")
		assert.Equal(t, 1, res.ExitCode())

		// Should report heal failure with specific counts in stderr
		errOutput := res.Stderr.String()
		assert.Contains(t, errOutput, "1 blocks corrupt")
		assert.Contains(t, errOutput, "1 removed")
		assert.Contains(t, errOutput, "1 failed to heal")
	})

	t.Run("large repository scale test", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()

		// Create 1000 small blocks
		for i := 0; i < 1000; i++ {
			node.IPFSAddStr(fmt.Sprintf("content-%d", i))
		}

		// Corrupt 10 blocks
		corruptMultipleBlocks(t, node, 10)

		// Verify handles large repos efficiently
		res := node.RunIPFS("repo", "verify")
		assert.Equal(t, 1, res.ExitCode())

		// Should report exactly 10 corrupt blocks in stderr
		assert.Contains(t, res.Stderr.String(), "10 blocks corrupt")

		// Test --drop at scale
		res = node.RunIPFS("repo", "verify", "--drop")
		assert.Equal(t, 0, res.ExitCode(), "should exit 0 when all corrupt blocks removed successfully")
		output := res.Stdout.String()
		assert.Contains(t, output, "10 blocks corrupt")
		assert.Contains(t, output, "10 removed")
	})

	t.Run("drop with partial removal failures", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()

		// Create several blocks
		for i := 0; i < 5; i++ {
			node.IPFSAddStr(fmt.Sprintf("content for removal test %d", i))
		}

		// Corrupt 3 blocks
		corruptedFiles := corruptMultipleBlocks(t, node, 3)
		require.Len(t, corruptedFiles, 3)

		// Make one of the corrupted files read-only to simulate removal failure
		err := os.Chmod(corruptedFiles[0], 0400) // read-only
		require.NoError(t, err)
		defer func() { _ = os.Chmod(corruptedFiles[0], 0644) }() // cleanup

		// Also make the directory read-only to prevent deletion
		blockDir := filepath.Dir(corruptedFiles[0])
		originalPerm, err := os.Stat(blockDir)
		require.NoError(t, err)
		err = os.Chmod(blockDir, 0500) // read+execute only, no write
		require.NoError(t, err)
		defer func() { _ = os.Chmod(blockDir, originalPerm.Mode()) }() // cleanup

		// Try to drop - should fail because at least one block can't be removed
		res := node.RunIPFS("repo", "verify", "--drop")
		assert.Equal(t, 1, res.ExitCode(), "should exit 1 when some blocks fail to remove")

		// Restore permissions for verification
		_ = os.Chmod(blockDir, originalPerm.Mode())
		_ = os.Chmod(corruptedFiles[0], 0644)

		// Should report both successes and failures with specific counts
		errOutput := res.Stderr.String()
		assert.Contains(t, errOutput, "3 blocks corrupt")
		assert.Contains(t, errOutput, "2 removed")
		assert.Contains(t, errOutput, "1 failed to remove")
	})
}
