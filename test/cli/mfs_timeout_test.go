package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/require"
)

// lazyFileWithMissingBlocks creates a multi-block file, references it into MFS
// with `files cp`, then removes its leaf blocks while the daemon is stopped so
// that a restarted daemon must fetch content that is gone and cannot be found.
// It returns the MFS path; the daemon is running again on return.
func lazyFileWithMissingBlocks(t *testing.T, node *harness.Node) string {
	t.Helper()
	content := strings.Repeat("z", 1<<20) // 1 MiB
	// Force 256KiB chunks so the file spans several blocks; the default 1 MiB
	// chunker would produce a single leaf with no children to remove.
	cid := node.PipeStrToIPFS(content, "add", "-q", "--pin=false", "--chunker=size-262144").Stdout.Trimmed()
	leaves := strings.Fields(node.IPFS("refs", cid).Stdout.String())
	require.NotEmpty(t, leaves, "the test file must have leaf blocks to remove")
	node.IPFS("files", "cp", "/ipfs/"+cid, "/lazy")

	// Remove the leaves offline so the restarted daemon cannot serve them from
	// an in-memory cache; only the file's local root node is left in MFS.
	node.StopDaemon()
	for _, leaf := range leaves {
		node.IPFS("block", "rm", "--force", leaf)
	}
	node.StartDaemon()
	return "/lazy"
}

// TestMFSTimeoutOnMissingBlock verifies that an `ipfs files` operation waiting
// on a block that is missing and unreachable ends with an error when the client
// sets --timeout, instead of hanging forever.
func TestMFSTimeoutOnMissingBlock(t *testing.T) {
	t.Parallel()

	t.Run("read honors --timeout", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		path := lazyFileWithMissingBlocks(t, node)

		start := time.Now()
		res := node.RunIPFS("files", "read", "--timeout=5s", path)
		require.NotEqual(t, 0, res.ExitCode(), "read of unreachable content should fail, not succeed")
		require.Less(t, time.Since(start), 60*time.Second, "read should stop on --timeout, not hang")
	})

	t.Run("write honors --timeout", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		path := lazyFileWithMissingBlocks(t, node)

		// Writing into existing content read-modify-writes the target block,
		// which is now missing.
		start := time.Now()
		res := node.RunPipeToIPFS(strings.NewReader("hello"),
			"files", "write", "--offset=500000", "--timeout=5s", path)
		require.NotEqual(t, 0, res.ExitCode(), "write over unreachable content should fail, not succeed")
		require.Less(t, time.Since(start), 60*time.Second, "write should stop on --timeout, not hang")
	})
}
