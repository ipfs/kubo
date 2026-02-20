package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ft "github.com/ipfs/boxo/ipld/unixfs"
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
		defer node.StopDaemon()

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
		defer node.StopDaemon()

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
		defer node.StopDaemon()

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
		defer node.StopDaemon()

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
		defer node.StopDaemon()

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
		defer node.StopDaemon()

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
		defer node.StopDaemon()

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
		defer node.StopDaemon()

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
		defer node.StopDaemon()

		// Perform 256 operations with --flush=false (should succeed)
		for i := range 256 {
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
		defer node.StopDaemon()

		// Perform 5 operations (should succeed)
		for i := range 5 {
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
		defer node.StopDaemon()

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
		defer node.StopDaemon()

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
		defer node.StopDaemon()

		// Should be able to do many operations without error
		for i := range 300 {
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
		defer node.StopDaemon()

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

func TestFilesChroot(t *testing.T) {
	t.Parallel()

	// Known CIDs for testing
	emptyDirCid := "QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn"

	t.Run("requires --confirm flag", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		// Don't start daemon - chroot runs offline

		res := node.RunIPFS("files", "chroot")
		require.NotNil(t, res.ExitErr)
		assert.NotEqual(t, 0, res.ExitErr.ExitCode())
		assert.Contains(t, res.Stderr.String(), "pass --confirm to proceed")
	})

	t.Run("resets to empty directory", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()

		// Start daemon to create MFS state
		node.StartDaemon()
		node.IPFS("files", "mkdir", "/testdir")
		node.StopDaemon()

		// Reset MFS to empty - should exit 0
		res := node.RunIPFS("files", "chroot", "--confirm")
		assert.Nil(t, res.ExitErr, "expected exit code 0")
		assert.Contains(t, res.Stdout.String(), emptyDirCid)

		// Verify daemon starts and MFS is empty
		node.StartDaemon()
		defer node.StopDaemon()
		lsRes := node.IPFS("files", "ls", "/")
		assert.Empty(t, lsRes.Stdout.Trimmed())
	})

	t.Run("replaces with valid directory CID", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()

		// Start daemon to add content
		node.StartDaemon()
		node.IPFS("files", "mkdir", "/mydir")
		// Create a temp file for content
		tempFile := filepath.Join(node.Dir, "testfile.txt")
		require.NoError(t, os.WriteFile(tempFile, []byte("hello"), 0644))
		node.IPFS("files", "write", "--create", "/mydir/file.txt", tempFile)
		statRes := node.IPFS("files", "stat", "--hash", "/mydir")
		dirCid := statRes.Stdout.Trimmed()
		node.StopDaemon()

		// Reset to empty first
		node.IPFS("files", "chroot", "--confirm")

		// Set root to the saved directory - should exit 0
		res := node.RunIPFS("files", "chroot", "--confirm", dirCid)
		assert.Nil(t, res.ExitErr, "expected exit code 0")
		assert.Contains(t, res.Stdout.String(), dirCid)

		// Verify content
		node.StartDaemon()
		defer node.StopDaemon()
		readRes := node.IPFS("files", "read", "/file.txt")
		assert.Equal(t, "hello", readRes.Stdout.Trimmed())
	})

	t.Run("fails with non-existent CID", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()

		res := node.RunIPFS("files", "chroot", "--confirm", "bafybeibdxtd5thfoitjmnfhxhywokebwdmwnuqgkzjjdjhwjz7qh77777a")
		require.NotNil(t, res.ExitErr)
		assert.NotEqual(t, 0, res.ExitErr.ExitCode())
		assert.Contains(t, res.Stderr.String(), "does not exist locally")
	})

	t.Run("fails with file CID", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()

		// Add a file to get a file CID
		node.StartDaemon()
		fileCid := node.IPFSAddStr("hello world")
		node.StopDaemon()

		// Try to set file as root - should fail with non-zero exit
		res := node.RunIPFS("files", "chroot", "--confirm", fileCid)
		require.NotNil(t, res.ExitErr)
		assert.NotEqual(t, 0, res.ExitErr.ExitCode())
		assert.Contains(t, res.Stderr.String(), "must be a directory")
	})

	t.Run("fails while daemon is running", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		res := node.RunIPFS("files", "chroot", "--confirm")
		require.NotNil(t, res.ExitErr)
		assert.NotEqual(t, 0, res.ExitErr.ExitCode())
		assert.Contains(t, res.Stderr.String(), "opening repo")
	})
}

// TestFilesMFSImportConfig tests that MFS operations respect Import.* configuration settings.
// These tests verify that `ipfs files` commands use the same import settings as `ipfs add`.
func TestFilesMFSImportConfig(t *testing.T) {
	t.Parallel()

	t.Run("files write respects Import.CidVersion=1", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Import.CidVersion = *config.NewOptionalInteger(1)
		})
		node.StartDaemon()
		defer node.StopDaemon()

		// Write file via MFS
		tempFile := filepath.Join(node.Dir, "test.txt")
		require.NoError(t, os.WriteFile(tempFile, []byte("hello"), 0644))
		node.IPFS("files", "write", "--create", "/test.txt", tempFile)

		// Get CID of written file
		cidStr := node.IPFS("files", "stat", "--hash", "/test.txt").Stdout.Trimmed()

		// Verify CIDv1 format (base32, starts with "b")
		require.True(t, strings.HasPrefix(cidStr, "b"), "expected CIDv1 (starts with b), got: %s", cidStr)
	})

	t.Run("files write respects Import.UnixFSRawLeaves=true", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Import.CidVersion = *config.NewOptionalInteger(1)
			cfg.Import.UnixFSRawLeaves = config.True
		})
		node.StartDaemon()
		defer node.StopDaemon()

		tempFile := filepath.Join(node.Dir, "test.txt")
		require.NoError(t, os.WriteFile(tempFile, []byte("hello world"), 0644))
		node.IPFS("files", "write", "--create", "/test.txt", tempFile)

		cidStr := node.IPFS("files", "stat", "--hash", "/test.txt").Stdout.Trimmed()
		codec := node.IPFS("cid", "format", "-f", "%c", cidStr).Stdout.Trimmed()
		require.Equal(t, "raw", codec, "expected raw codec for small file with raw leaves")
	})

	// This test verifies CID parity for single-block files only.
	// Multi-block files will have different CIDs because MFS uses trickle DAG layout
	// while 'ipfs add' uses balanced DAG layout. See "files write vs add for multi-block" test.
	t.Run("single-block file: files write produces same CID as ipfs add", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Import.CidVersion = *config.NewOptionalInteger(1)
			cfg.Import.UnixFSRawLeaves = config.True
		})
		node.StartDaemon()
		defer node.StopDaemon()

		tempFile := filepath.Join(node.Dir, "test.txt")
		require.NoError(t, os.WriteFile(tempFile, []byte("hello world"), 0644))
		node.IPFS("files", "write", "--create", "/test.txt", tempFile)

		mfsCid := node.IPFS("files", "stat", "--hash", "/test.txt").Stdout.Trimmed()
		addCid := node.IPFSAddStr("hello world")
		require.Equal(t, addCid, mfsCid, "MFS write should produce same CID as ipfs add for single-block files")
	})

	t.Run("files mkdir respects Import.CidVersion=1", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Import.CidVersion = *config.NewOptionalInteger(1)
		})
		node.StartDaemon()
		defer node.StopDaemon()

		node.IPFS("files", "mkdir", "/testdir")
		cidStr := node.IPFS("files", "stat", "--hash", "/testdir").Stdout.Trimmed()

		// Verify CIDv1 format
		require.True(t, strings.HasPrefix(cidStr, "b"), "expected CIDv1 (starts with b), got: %s", cidStr)
	})

	t.Run("MFS subdirectory becomes HAMT when exceeding threshold", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			// Use small threshold for faster testing
			cfg.Import.UnixFSHAMTDirectorySizeThreshold = *config.NewOptionalBytes("1KiB")
			cfg.Import.UnixFSHAMTDirectorySizeEstimation = *config.NewOptionalString("block")
		})
		node.StartDaemon()
		defer node.StopDaemon()

		node.IPFS("files", "mkdir", "/bigdir")

		content := "x"
		tempFile := filepath.Join(node.Dir, "content.txt")
		require.NoError(t, os.WriteFile(tempFile, []byte(content), 0644))

		// Add enough files to exceed 1KiB threshold
		for i := range 25 {
			node.IPFS("files", "write", "--create", fmt.Sprintf("/bigdir/file%02d", i), tempFile)
		}

		cidStr := node.IPFS("files", "stat", "--hash", "/bigdir").Stdout.Trimmed()
		fsType, err := node.UnixFSDataType(cidStr)
		require.NoError(t, err)
		require.Equal(t, ft.THAMTShard, fsType, "expected HAMT directory")
	})

	t.Run("MFS root directory becomes HAMT when exceeding threshold", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Import.UnixFSHAMTDirectorySizeThreshold = *config.NewOptionalBytes("1KiB")
			cfg.Import.UnixFSHAMTDirectorySizeEstimation = *config.NewOptionalString("block")
		})
		node.StartDaemon()
		defer node.StopDaemon()

		content := "x"
		tempFile := filepath.Join(node.Dir, "content.txt")
		require.NoError(t, os.WriteFile(tempFile, []byte(content), 0644))

		// Add files directly to root /
		for i := range 25 {
			node.IPFS("files", "write", "--create", fmt.Sprintf("/file%02d", i), tempFile)
		}

		cidStr := node.IPFS("files", "stat", "--hash", "/").Stdout.Trimmed()
		fsType, err := node.UnixFSDataType(cidStr)
		require.NoError(t, err)
		require.Equal(t, ft.THAMTShard, fsType, "expected MFS root to become HAMT")
	})

	t.Run("MFS directory reverts from HAMT to basic when items removed", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Import.UnixFSHAMTDirectorySizeThreshold = *config.NewOptionalBytes("1KiB")
			cfg.Import.UnixFSHAMTDirectorySizeEstimation = *config.NewOptionalString("block")
		})
		node.StartDaemon()
		defer node.StopDaemon()

		node.IPFS("files", "mkdir", "/testdir")

		content := "x"
		tempFile := filepath.Join(node.Dir, "content.txt")
		require.NoError(t, os.WriteFile(tempFile, []byte(content), 0644))

		// Add files to exceed threshold
		for i := range 25 {
			node.IPFS("files", "write", "--create", fmt.Sprintf("/testdir/file%02d", i), tempFile)
		}

		// Verify it became HAMT
		cidStr := node.IPFS("files", "stat", "--hash", "/testdir").Stdout.Trimmed()
		fsType, err := node.UnixFSDataType(cidStr)
		require.NoError(t, err)
		require.Equal(t, ft.THAMTShard, fsType, "should be HAMT after adding many files")

		// Remove files to get back below threshold
		for i := range 20 {
			node.IPFS("files", "rm", fmt.Sprintf("/testdir/file%02d", i))
		}

		// Verify it reverted to basic directory
		cidStr = node.IPFS("files", "stat", "--hash", "/testdir").Stdout.Trimmed()
		fsType, err = node.UnixFSDataType(cidStr)
		require.NoError(t, err)
		require.Equal(t, ft.TDirectory, fsType, "should revert to basic directory after removing files")
	})

	// Note: 'files write' produces DIFFERENT CIDs than 'ipfs add' for multi-block files because
	// MFS uses trickle DAG layout while 'ipfs add' uses balanced DAG layout.
	// Single-block files produce the same CID (tested above in "single-block file: files write...").
	// For multi-block CID compatibility with 'ipfs add', use 'ipfs add --to-files' instead.

	t.Run("files cp preserves original CID", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Import.CidVersion = *config.NewOptionalInteger(1)
			cfg.Import.UnixFSRawLeaves = config.True
		})
		node.StartDaemon()
		defer node.StopDaemon()

		// Add file via ipfs add
		originalCid := node.IPFSAddStr("hello world")

		// Copy to MFS
		node.IPFS("files", "cp", fmt.Sprintf("/ipfs/%s", originalCid), "/copied.txt")

		// Verify CID is preserved
		mfsCid := node.IPFS("files", "stat", "--hash", "/copied.txt").Stdout.Trimmed()
		require.Equal(t, originalCid, mfsCid, "files cp should preserve original CID")
	})

	t.Run("add --to-files respects Import config", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Import.CidVersion = *config.NewOptionalInteger(1)
			cfg.Import.UnixFSRawLeaves = config.True
		})
		node.StartDaemon()
		defer node.StopDaemon()

		// Create temp file
		tempFile := filepath.Join(node.Dir, "test.txt")
		require.NoError(t, os.WriteFile(tempFile, []byte("hello world"), 0644))

		// Add with --to-files
		addCid := node.IPFS("add", "-Q", "--to-files=/added.txt", tempFile).Stdout.Trimmed()

		// Verify MFS file has same CID
		mfsCid := node.IPFS("files", "stat", "--hash", "/added.txt").Stdout.Trimmed()
		require.Equal(t, addCid, mfsCid)

		// Should be CIDv1 raw leaf
		codec := node.IPFS("cid", "format", "-f", "%c", mfsCid).Stdout.Trimmed()
		require.Equal(t, "raw", codec)
	})

	t.Run("files mkdir respects Import.UnixFSDirectoryMaxLinks", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Import.CidVersion = *config.NewOptionalInteger(1)
			// Set low link threshold to trigger HAMT sharding at 5 links
			cfg.Import.UnixFSDirectoryMaxLinks = *config.NewOptionalInteger(5)
			// Also need size estimation enabled for switching to work
			cfg.Import.UnixFSHAMTDirectorySizeEstimation = *config.NewOptionalString("block")
		})
		node.StartDaemon()
		defer node.StopDaemon()

		// Create directory with 6 files (exceeds max 5 links)
		node.IPFS("files", "mkdir", "/testdir")

		content := "x"
		tempFile := filepath.Join(node.Dir, "content.txt")
		require.NoError(t, os.WriteFile(tempFile, []byte(content), 0644))

		for i := range 6 {
			node.IPFS("files", "write", "--create", fmt.Sprintf("/testdir/file%d.txt", i), tempFile)
		}

		// Verify directory became HAMT sharded
		cidStr := node.IPFS("files", "stat", "--hash", "/testdir").Stdout.Trimmed()
		fsType, err := node.UnixFSDataType(cidStr)
		require.NoError(t, err)
		require.Equal(t, ft.THAMTShard, fsType, "expected HAMT directory after exceeding UnixFSDirectoryMaxLinks")
	})

	t.Run("files write respects Import.UnixFSChunker", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Import.CidVersion = *config.NewOptionalInteger(1)
			cfg.Import.UnixFSRawLeaves = config.True
			cfg.Import.UnixFSChunker = *config.NewOptionalString("size-1024") // 1KB chunks
		})
		node.StartDaemon()
		defer node.StopDaemon()

		// Create file larger than chunk size (3KB)
		data := make([]byte, 3*1024)
		for i := range data {
			data[i] = byte(i % 256)
		}
		tempFile := filepath.Join(node.Dir, "large.bin")
		require.NoError(t, os.WriteFile(tempFile, data, 0644))

		node.IPFS("files", "write", "--create", "/large.bin", tempFile)

		// Verify chunking: 3KB file with 1KB chunks should have multiple child blocks
		cidStr := node.IPFS("files", "stat", "--hash", "/large.bin").Stdout.Trimmed()
		dagStatJSON := node.IPFS("dag", "stat", "--enc=json", cidStr).Stdout.Trimmed()
		var dagStat struct {
			UniqueBlocks int `json:"UniqueBlocks"`
		}
		require.NoError(t, json.Unmarshal([]byte(dagStatJSON), &dagStat))
		// With 1KB chunks on a 3KB file, we expect 4 blocks (3 leaf + 1 root)
		assert.Greater(t, dagStat.UniqueBlocks, 1, "expected more than 1 block with 1KB chunker on 3KB file")
	})

	t.Run("files write with custom chunker produces same CID as ipfs add --trickle", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Import.CidVersion = *config.NewOptionalInteger(1)
			cfg.Import.UnixFSRawLeaves = config.True
			cfg.Import.UnixFSChunker = *config.NewOptionalString("size-512")
		})
		node.StartDaemon()
		defer node.StopDaemon()

		// Create test data (2KB to get multiple chunks)
		data := make([]byte, 2048)
		for i := range data {
			data[i] = byte(i % 256)
		}
		tempFile := filepath.Join(node.Dir, "test.bin")
		require.NoError(t, os.WriteFile(tempFile, data, 0644))

		// Add via MFS
		node.IPFS("files", "write", "--create", "/test.bin", tempFile)
		mfsCid := node.IPFS("files", "stat", "--hash", "/test.bin").Stdout.Trimmed()

		// Add via ipfs add with same chunker and trickle (MFS always uses trickle)
		addCid := node.IPFS("add", "-Q", "--chunker=size-512", "--trickle", tempFile).Stdout.Trimmed()

		// CIDs should match when using same chunker + trickle layout
		require.Equal(t, addCid, mfsCid, "MFS and add --trickle should produce same CID with matching chunker")
	})

	t.Run("files mkdir respects Import.UnixFSHAMTDirectoryMaxFanout", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			// Use non-default fanout of 64 (default is 256)
			cfg.Import.UnixFSHAMTDirectoryMaxFanout = *config.NewOptionalInteger(64)
			// Set low link threshold to trigger HAMT at 5 links
			cfg.Import.UnixFSDirectoryMaxLinks = *config.NewOptionalInteger(5)
			cfg.Import.UnixFSHAMTDirectorySizeEstimation = *config.NewOptionalString("disabled")
		})
		node.StartDaemon()
		defer node.StopDaemon()

		node.IPFS("files", "mkdir", "/testdir")

		content := "x"
		tempFile := filepath.Join(node.Dir, "content.txt")
		require.NoError(t, os.WriteFile(tempFile, []byte(content), 0644))

		// Add 6 files (exceeds MaxLinks=5) to trigger HAMT
		for i := range 6 {
			node.IPFS("files", "write", "--create", fmt.Sprintf("/testdir/file%d.txt", i), tempFile)
		}

		// Verify directory became HAMT
		cidStr := node.IPFS("files", "stat", "--hash", "/testdir").Stdout.Trimmed()
		fsType, err := node.UnixFSDataType(cidStr)
		require.NoError(t, err)
		require.Equal(t, ft.THAMTShard, fsType, "expected HAMT directory")

		// Verify the HAMT uses the custom fanout (64) by inspecting the UnixFS Data field.
		fanout, err := node.UnixFSHAMTFanout(cidStr)
		require.NoError(t, err)
		require.Equal(t, uint64(64), fanout, "expected HAMT fanout 64")
	})

	t.Run("files mkdir respects Import.UnixFSHAMTDirectorySizeThreshold", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			// Use very small threshold (100 bytes) to trigger HAMT quickly
			cfg.Import.UnixFSHAMTDirectorySizeThreshold = *config.NewOptionalBytes("100B")
			cfg.Import.UnixFSHAMTDirectorySizeEstimation = *config.NewOptionalString("block")
		})
		node.StartDaemon()
		defer node.StopDaemon()

		node.IPFS("files", "mkdir", "/testdir")

		content := "test content"
		tempFile := filepath.Join(node.Dir, "content.txt")
		require.NoError(t, os.WriteFile(tempFile, []byte(content), 0644))

		// Add 3 files - each link adds ~40-50 bytes, so 3 should exceed 100B threshold
		for i := range 3 {
			node.IPFS("files", "write", "--create", fmt.Sprintf("/testdir/file%d.txt", i), tempFile)
		}

		// Verify directory became HAMT due to size threshold
		cidStr := node.IPFS("files", "stat", "--hash", "/testdir").Stdout.Trimmed()
		fsType, err := node.UnixFSDataType(cidStr)
		require.NoError(t, err)
		require.Equal(t, ft.THAMTShard, fsType, "expected HAMT directory after exceeding size threshold")
	})

	t.Run("config change takes effect after daemon restart", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()

		// Start with high threshold (won't trigger HAMT)
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Import.UnixFSHAMTDirectorySizeThreshold = *config.NewOptionalBytes("256KiB")
			cfg.Import.UnixFSHAMTDirectorySizeEstimation = *config.NewOptionalString("block")
		})
		node.StartDaemon()

		// Create directory with some files
		node.IPFS("files", "mkdir", "/testdir")
		content := "test"
		tempFile := filepath.Join(node.Dir, "content.txt")
		require.NoError(t, os.WriteFile(tempFile, []byte(content), 0644))
		for i := range 3 {
			node.IPFS("files", "write", "--create", fmt.Sprintf("/testdir/file%d.txt", i), tempFile)
		}

		// Verify it's still a basic directory (threshold not exceeded)
		cidStr := node.IPFS("files", "stat", "--hash", "/testdir").Stdout.Trimmed()
		fsType, err := node.UnixFSDataType(cidStr)
		require.NoError(t, err)
		require.Equal(t, ft.TDirectory, fsType, "should be basic directory with high threshold")

		// Stop daemon
		node.StopDaemon()

		// Change config to use very low threshold
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Import.UnixFSHAMTDirectorySizeThreshold = *config.NewOptionalBytes("100B")
		})

		// Restart daemon
		node.StartDaemon()
		defer node.StopDaemon()

		// Add one more file - this should trigger HAMT conversion with new threshold
		node.IPFS("files", "write", "--create", "/testdir/file3.txt", tempFile)

		// Verify it became HAMT (new threshold applied)
		cidStr = node.IPFS("files", "stat", "--hash", "/testdir").Stdout.Trimmed()
		fsType, err = node.UnixFSDataType(cidStr)
		require.NoError(t, err)
		require.Equal(t, ft.THAMTShard, fsType, "should be HAMT after daemon restart with lower threshold")
	})
}
