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

	t.Run("files cp with valid UnixFS", func(t *testing.T) {
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

	t.Run("files cp with invalid DAG node fails without force", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()

		// create a dag-cbor node
		jsonData := `{"data": "not a UnixFS node"}`
		tempFile := filepath.Join(node.Dir, "test.json")
		err := os.WriteFile(tempFile, []byte(jsonData), 0644)
		require.NoError(t, err)
		cid := node.IPFS("dag", "put", "--input-codec=json", "--store-codec=dag-cbor", tempFile).Stdout.Trimmed()

		// copy without --force
		res := node.RunIPFS("files", "cp", fmt.Sprintf("/ipfs/%s", cid), "/invalid-file")
		assert.NotEqual(t, 0, res.ExitErr.ExitCode())
		assert.Contains(t, res.Stderr.String(), "source must be a UnixFS")
	})

	t.Run("files cp with invalid DAG node succeeds with force", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()

		// create dag-bor node
		jsonData := `{"data": "not a UnixFS node"}`
		tempFile := filepath.Join(node.Dir, "test.json")
		err := os.WriteFile(tempFile, []byte(jsonData), 0644)
		require.NoError(t, err)
		cid := node.IPFS("dag", "put", "--input-codec=json", "--store-codec=dag-cbor", tempFile).Stdout.Trimmed()

		// copy with --force
		resWithForce := node.RunIPFS("files", "cp", "--force", fmt.Sprintf("/ipfs/%s", cid), "/forced-file")
		assert.NotEqual(t, 0, resWithForce.ExitErr.ExitCode())

		// Verification
		// Should NOT contain the validation error
		assert.NotContains(t, resWithForce.Stderr.String(), "source must be a valid UnixFS")

		// But should contain flush error instead
		assert.Contains(t, resWithForce.Stderr.String(), "cannot flush the created file")
	})
}
