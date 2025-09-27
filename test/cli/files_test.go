package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ipfs/kubo/config"
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

func TestFilesRm(t *testing.T) {
	t.Parallel()

	t.Run("files rm with --flush=false returns error", func(t *testing.T) {
		// Test that files rm rejects --flush=false so user does not assume disabling flush works
		// (rm ignored it before, better to explicitly error)
		// See https://github.com/ipfs/kubo/issues/10842
		t.Parallel()

		node := harness.NewT(t).NewNode().Init().StartDaemon()

		// Create a file to remove
		node.IPFS("files", "mkdir", "/test-dir")

		// Try to remove with --flush=false, should error
		res := node.RunIPFS("files", "rm", "-r", "--flush=false", "/test-dir")
		assert.NotEqual(t, 0, res.ExitErr.ExitCode())
		assert.Contains(t, res.Stderr.String(), "files rm always flushes for safety")
		assert.Contains(t, res.Stderr.String(), "cannot be set to false")

		// Verify the directory still exists (wasn't removed due to error)
		lsRes := node.IPFS("files", "ls", "/")
		assert.Contains(t, lsRes.Stdout.String(), "test-dir")
	})

	t.Run("files rm with --flush=true works", func(t *testing.T) {
		t.Parallel()

		node := harness.NewT(t).NewNode().Init().StartDaemon()

		// Create a file to remove
		node.IPFS("files", "mkdir", "/test-dir")

		// Remove with explicit --flush=true, should work
		res := node.IPFS("files", "rm", "-r", "--flush=true", "/test-dir")
		assert.NoError(t, res.Err)

		// Verify the directory was removed
		lsRes := node.IPFS("files", "ls", "/")
		assert.NotContains(t, lsRes.Stdout.String(), "test-dir")
	})

	t.Run("files rm without flush flag works (default behavior)", func(t *testing.T) {
		t.Parallel()

		node := harness.NewT(t).NewNode().Init().StartDaemon()

		// Create a file to remove
		node.IPFS("files", "mkdir", "/test-dir")

		// Remove without flush flag (should use default which is true)
		res := node.IPFS("files", "rm", "-r", "/test-dir")
		assert.NoError(t, res.Err)

		// Verify the directory was removed
		lsRes := node.IPFS("files", "ls", "/")
		assert.NotContains(t, lsRes.Stdout.String(), "test-dir")
	})
}

func TestFilesNoFlushLimit(t *testing.T) {
	t.Parallel()

	t.Run("reaches default limit of 256 operations", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()

		// Perform 256 operations with --flush=false (should succeed)
		for i := 0; i < 256; i++ {
			res := node.IPFS("files", "mkdir", "--flush=false", fmt.Sprintf("/dir%d", i))
			assert.NoError(t, res.Err, "operation %d should succeed", i+1)
		}

		// 257th operation should fail
		res := node.RunIPFS("files", "mkdir", "--flush=false", "/dir256")
		require.NotNil(t, res.ExitErr, "command should have failed")
		assert.NotEqual(t, 0, res.ExitErr.ExitCode())
		assert.Contains(t, res.Stderr.String(), "reached limit of 256 unflushed MFS operations")
		assert.Contains(t, res.Stderr.String(), "run 'ipfs files flush'")
		assert.Contains(t, res.Stderr.String(), "use --flush=true")
		assert.Contains(t, res.Stderr.String(), "increase Internal.MFSNoFlushLimit")
	})

	t.Run("custom limit via config", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()

		// Set custom limit to 5
		node.UpdateConfig(func(cfg *config.Config) {
			limit := config.NewOptionalInteger(5)
			cfg.Internal.MFSNoFlushLimit = limit
		})

		node.StartDaemon()

		// Perform 5 operations (should succeed)
		for i := 0; i < 5; i++ {
			res := node.IPFS("files", "mkdir", "--flush=false", fmt.Sprintf("/dir%d", i))
			assert.NoError(t, res.Err, "operation %d should succeed", i+1)
		}

		// 6th operation should fail
		res := node.RunIPFS("files", "mkdir", "--flush=false", "/dir5")
		require.NotNil(t, res.ExitErr, "command should have failed")
		assert.NotEqual(t, 0, res.ExitErr.ExitCode())
		assert.Contains(t, res.Stderr.String(), "reached limit of 5 unflushed MFS operations")
	})

	t.Run("flush=true resets counter", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()

		// Set limit to 3 for faster testing
		node.UpdateConfig(func(cfg *config.Config) {
			limit := config.NewOptionalInteger(3)
			cfg.Internal.MFSNoFlushLimit = limit
		})

		node.StartDaemon()

		// Do 2 operations with --flush=false
		node.IPFS("files", "mkdir", "--flush=false", "/dir1")
		node.IPFS("files", "mkdir", "--flush=false", "/dir2")

		// Operation with --flush=true should reset counter
		node.IPFS("files", "mkdir", "--flush=true", "/dir3")

		// Now we should be able to do 3 more operations with --flush=false
		for i := 4; i <= 6; i++ {
			res := node.IPFS("files", "mkdir", "--flush=false", fmt.Sprintf("/dir%d", i))
			assert.NoError(t, res.Err, "operation after flush should succeed")
		}

		// 4th operation after reset should fail
		res := node.RunIPFS("files", "mkdir", "--flush=false", "/dir7")
		require.NotNil(t, res.ExitErr, "command should have failed")
		assert.NotEqual(t, 0, res.ExitErr.ExitCode())
		assert.Contains(t, res.Stderr.String(), "reached limit of 3 unflushed MFS operations")
	})

	t.Run("explicit flush command resets counter", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()

		// Set limit to 3 for faster testing
		node.UpdateConfig(func(cfg *config.Config) {
			limit := config.NewOptionalInteger(3)
			cfg.Internal.MFSNoFlushLimit = limit
		})

		node.StartDaemon()

		// Do 2 operations with --flush=false
		node.IPFS("files", "mkdir", "--flush=false", "/dir1")
		node.IPFS("files", "mkdir", "--flush=false", "/dir2")

		// Explicit flush should reset counter
		node.IPFS("files", "flush")

		// Now we should be able to do 3 more operations
		for i := 3; i <= 5; i++ {
			res := node.IPFS("files", "mkdir", "--flush=false", fmt.Sprintf("/dir%d", i))
			assert.NoError(t, res.Err, "operation after flush should succeed")
		}

		// 4th operation should fail
		res := node.RunIPFS("files", "mkdir", "--flush=false", "/dir6")
		require.NotNil(t, res.ExitErr, "command should have failed")
		assert.NotEqual(t, 0, res.ExitErr.ExitCode())
		assert.Contains(t, res.Stderr.String(), "reached limit of 3 unflushed MFS operations")
	})

	t.Run("limit=0 disables the feature", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()

		// Set limit to 0 (disabled)
		node.UpdateConfig(func(cfg *config.Config) {
			limit := config.NewOptionalInteger(0)
			cfg.Internal.MFSNoFlushLimit = limit
		})

		node.StartDaemon()

		// Should be able to do many operations without error
		for i := 0; i < 300; i++ {
			res := node.IPFS("files", "mkdir", "--flush=false", fmt.Sprintf("/dir%d", i))
			assert.NoError(t, res.Err, "operation %d should succeed with limit disabled", i+1)
		}
	})

	t.Run("different MFS commands count towards limit", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()

		// Set limit to 5 for testing
		node.UpdateConfig(func(cfg *config.Config) {
			limit := config.NewOptionalInteger(5)
			cfg.Internal.MFSNoFlushLimit = limit
		})

		node.StartDaemon()

		// Mix of different MFS operations (5 operations to hit the limit)
		node.IPFS("files", "mkdir", "--flush=false", "/testdir")
		// Create a file first, then copy it
		testCid := node.IPFSAddStr("test content")
		node.IPFS("files", "cp", "--flush=false", fmt.Sprintf("/ipfs/%s", testCid), "/testfile")
		node.IPFS("files", "cp", "--flush=false", "/testfile", "/testfile2")
		node.IPFS("files", "mv", "--flush=false", "/testfile2", "/testfile3")
		node.IPFS("files", "mkdir", "--flush=false", "/anotherdir")

		// 6th operation should fail
		res := node.RunIPFS("files", "mkdir", "--flush=false", "/another")
		require.NotNil(t, res.ExitErr, "command should have failed")
		assert.NotEqual(t, 0, res.ExitErr.ExitCode())
		assert.Contains(t, res.Stderr.String(), "reached limit of 5 unflushed MFS operations")
	})
}
