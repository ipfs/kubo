package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilesCp(t *testing.T) {
	t.Parallel()

	t.Run("files cp with valid UnixFS succeeds", func(t *testing.T) {
		t.Parallel()

		node := harness.NewT(t).NewNode().Init().StartDaemon()

		// Create simple text file
		data := "testing files cp command"
		cid := node.IPFSAddStr(data)

		// Copy form IPFS => MFS
		res := node.IPFS("files", "cp", fmt.Sprintf("/ipfs/%s", cid), "/valid-file")
		assert.NoError(t, res.Err)

		// verification
		catRes := node.IPFS("files", "read", "/valid-file")
		assert.Equal(t, data, catRes.Stdout.Trimmed())
	})

	t.Run("files cp with unsupported DAG node type fails", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()

		// MFS UnixFS is limited to dag-pb or raw, so we create a dag-cbor node to test this
		jsonData := `{"data": "not a UnixFS node"}`
		tempFile := filepath.Join(node.Dir, "test.json")
		err := os.WriteFile(tempFile, []byte(jsonData), 0644)
		require.NoError(t, err)
		cid := node.IPFS("dag", "put", "--input-codec=json", "--store-codec=dag-cbor", tempFile).Stdout.Trimmed()

		// copy without --force
		res := node.RunIPFS("files", "cp", fmt.Sprintf("/ipfs/%s", cid), "/invalid-file")
		assert.NotEqual(t, 0, res.ExitErr.ExitCode())
		assert.Contains(t, res.Stderr.String(), "Error: cp: source must be a valid UnixFS (dag-pb or raw codec)")
	})

	t.Run("files cp with invalid UnixFS data structure fails", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()

		// Create an invalid proto file
		data := []byte{0xDE, 0xAD, 0xBE, 0xEF} // Invalid protobuf data
		tempFile := filepath.Join(node.Dir, "invalid-proto.bin")
		err := os.WriteFile(tempFile, data, 0644)
		require.NoError(t, err)

		res := node.IPFS("block", "put", "--format=raw", tempFile)
		require.NoError(t, res.Err)

		// we manually changed codec from raw to dag-pb to test "bad dag-pb" scenario
		cid := "bafybeic7pdbte5heh6u54vszezob3el6exadoiw4wc4ne7ny2x7kvajzkm"

		// should fail because node cannot be read as a valid dag-pb
		cpResNoForce := node.RunIPFS("files", "cp", fmt.Sprintf("/ipfs/%s", cid), "/invalid-proto")
		assert.NotEqual(t, 0, cpResNoForce.ExitErr.ExitCode())
		assert.Contains(t, cpResNoForce.Stderr.String(), "Error")
	})

	t.Run("files cp with raw node succeeds", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()

		// Create a raw node
		data := "raw data"
		tempFile := filepath.Join(node.Dir, "raw.bin")
		err := os.WriteFile(tempFile, []byte(data), 0644)
		require.NoError(t, err)

		res := node.IPFS("block", "put", "--format=raw", tempFile)
		require.NoError(t, res.Err)
		cid := res.Stdout.Trimmed()

		// Copy from IPFS to MFS (raw nodes should work without --force)
		cpRes := node.IPFS("files", "cp", fmt.Sprintf("/ipfs/%s", cid), "/raw-file")
		assert.NoError(t, cpRes.Err)

		// Verify the file was copied correctly
		catRes := node.IPFS("files", "read", "/raw-file")
		assert.Equal(t, data, catRes.Stdout.Trimmed())
	})

	t.Run("files cp creates intermediate directories with -p", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()

		// Create a simple text file and add it to IPFS
		data := "hello parent directories"
		tempFile := filepath.Join(node.Dir, "parent-test.txt")
		err := os.WriteFile(tempFile, []byte(data), 0644)
		require.NoError(t, err)

		cid := node.IPFS("add", "-Q", tempFile).Stdout.Trimmed()

		// Copy from IPFS to MFS with parent flag
		res := node.IPFS("files", "cp", "-p", fmt.Sprintf("/ipfs/%s", cid), "/parent/dir/file")
		assert.NoError(t, res.Err)

		// Verify the file and directories were created
		lsRes := node.IPFS("files", "ls", "/parent/dir")
		assert.Contains(t, lsRes.Stdout.String(), "file")

		catRes := node.IPFS("files", "read", "/parent/dir/file")
		assert.Equal(t, data, catRes.Stdout.Trimmed())
	})
}
