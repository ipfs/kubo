package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNativeIPFSURIs checks that commands accept native IPFS URIs (ipfs://cid,
// ipns://name, and the schemeless ipfs:cid / ipns:name forms) anywhere a content
// path or CID is accepted, so a value copied from a browser works as-is.
func TestNativeIPFSURIs(t *testing.T) {
	t.Parallel()

	node := harness.NewT(t).NewNode().Init()
	const content = "hello ipfs uri"
	fileCID := node.IPFSAddStr(content, "--cid-version=1")

	t.Run("cat accepts every form the same as a bare CID", func(t *testing.T) {
		for _, arg := range []string{
			fileCID,
			"/ipfs/" + fileCID,
			"ipfs://" + fileCID,
			"ipfs:" + fileCID,
		} {
			assert.Equalf(t, content, node.IPFS("cat", arg).Stdout.Trimmed(), "cat %q", arg)
		}
	})

	t.Run("block stat accepts a URI", func(t *testing.T) {
		assert.Contains(t, node.IPFS("block", "stat", "ipfs://"+fileCID).Stdout.String(), fileCID)
	})

	t.Run("ls and sub-paths work through a URI", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644))
		dirCID := node.IPFS("add", "-Q", "-r", "--cid-version=1", dir).Stdout.Trimmed()

		assert.Contains(t, node.IPFS("ls", "ipfs://"+dirCID).Stdout.String(), "a.txt")
		assert.Equal(t, "a", node.IPFS("cat", "ipfs://"+dirCID+"/a.txt").Stdout.String())
	})

	t.Run("name publish and resolve accept URIs", func(t *testing.T) {
		var published struct{ Name string }
		out := node.IPFS("name", "publish", "--allow-offline", "--enc=json", "ipfs://"+fileCID).Stdout.Bytes()
		require.NoError(t, json.Unmarshal(out, &published))

		want := "/ipfs/" + fileCID
		for _, arg := range []string{
			published.Name,
			"/ipns/" + published.Name,
			"ipns://" + published.Name,
			"ipns:" + published.Name,
		} {
			assert.Equalf(t, want, node.IPFS("name", "resolve", "--offline", arg).Stdout.Trimmed(), "name resolve %q", arg)
		}

		// A URI also resolves transitively through cat.
		assert.Equal(t, content, node.IPFS("cat", "--offline", "ipns://"+published.Name).Stdout.Trimmed())

		// name resolve stays IPNS-only: an ipfs:// URI is rejected, not echoed back.
		assert.NotEqual(t, 0, node.RunIPFS("name", "resolve", "--offline", "ipfs://"+fileCID).ExitCode())
	})

	t.Run("an invalid CID in a URI is rejected", func(t *testing.T) {
		res := node.RunIPFS("cat", "ipfs://not-a-cid")
		assert.NotEqual(t, 0, res.ExitCode())
	})
}
