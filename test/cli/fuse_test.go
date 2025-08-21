package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/ipfs/kubo/test/cli/testutils"
	"github.com/stretchr/testify/require"
)

func TestFUSE(t *testing.T) {
	testutils.RequiresFUSE(t)
	t.Parallel()

	t.Run("mount and unmount work correctly", func(t *testing.T) {
		t.Parallel()

		// Create a node and start daemon
		node := harness.NewT(t).NewNode().Init()
		node.StartDaemon()

		// Create mount directories in the node's working directory
		nodeDir := node.Dir
		ipfsMount := filepath.Join(nodeDir, "ipfs")
		ipnsMount := filepath.Join(nodeDir, "ipns")
		mfsMount := filepath.Join(nodeDir, "mfs")

		err := os.MkdirAll(ipfsMount, 0755)
		require.NoError(t, err)
		err = os.MkdirAll(ipnsMount, 0755)
		require.NoError(t, err)
		err = os.MkdirAll(mfsMount, 0755)
		require.NoError(t, err)

		// Ensure any existing mounts are cleaned up first
		failOnError := false // mount points might not exist from previous runs
		doUnmount(t, ipfsMount, failOnError)
		doUnmount(t, ipnsMount, failOnError)
		doUnmount(t, mfsMount, failOnError)

		// Test mount operation
		result := node.IPFS("mount", "-f", ipfsMount, "-n", ipnsMount, "-m", mfsMount)

		// Verify mount output
		expectedOutput := "IPFS mounted at: " + ipfsMount + "\n" +
			"IPNS mounted at: " + ipnsMount + "\n" +
			"MFS mounted at: " + mfsMount + "\n"
		require.Equal(t, expectedOutput, result.Stdout.String())

		// Test basic MFS functionality via FUSE mount
		testFile := filepath.Join(mfsMount, "testfile")
		testContent := "hello fuse world"

		// Create file via FUSE mount
		err = os.WriteFile(testFile, []byte(testContent), 0644)
		require.NoError(t, err)

		// Verify file appears in MFS via IPFS commands
		result = node.IPFS("files", "ls", "/")
		require.Contains(t, result.Stdout.String(), "testfile")

		// Read content back via MFS FUSE mount
		readContent, err := os.ReadFile(testFile)
		require.NoError(t, err)
		require.Equal(t, testContent, string(readContent))

		// Get the CID of the MFS file
		result = node.IPFS("files", "stat", "/testfile", "--format=<hash>")
		fileCID := strings.TrimSpace(result.Stdout.String())
		require.NotEmpty(t, fileCID, "should have a CID for the MFS file")

		// Read the same content via IPFS FUSE mount using the CID
		ipfsFile := filepath.Join(ipfsMount, fileCID)
		ipfsContent, err := os.ReadFile(ipfsFile)
		require.NoError(t, err)
		require.Equal(t, testContent, string(ipfsContent), "content should match between MFS and IPFS mounts")

		// Verify both FUSE mounts return identical data
		require.Equal(t, readContent, ipfsContent, "MFS and IPFS FUSE mounts should return identical data")

		// Test that mount directories cannot be removed while mounted
		err = os.Remove(ipfsMount)
		require.Error(t, err, "should not be able to remove mounted directory")

		// Stop daemon - this should trigger automatic unmount via context cancellation
		node.StopDaemon()

		// Daemon shutdown should handle unmount synchronously via context.AfterFunc

		// Verify directories can now be removed (indicating successful unmount)
		err = os.Remove(ipfsMount)
		require.NoError(t, err, "should be able to remove directory after unmount")
		err = os.Remove(ipnsMount)
		require.NoError(t, err, "should be able to remove directory after unmount")
		err = os.Remove(mfsMount)
		require.NoError(t, err, "should be able to remove directory after unmount")
	})

	t.Run("explicit unmount works", func(t *testing.T) {
		t.Parallel()

		node := harness.NewT(t).NewNode().Init()
		node.StartDaemon()

		// Create mount directories
		nodeDir := node.Dir
		ipfsMount := filepath.Join(nodeDir, "ipfs")
		ipnsMount := filepath.Join(nodeDir, "ipns")
		mfsMount := filepath.Join(nodeDir, "mfs")

		err := os.MkdirAll(ipfsMount, 0755)
		require.NoError(t, err)
		err = os.MkdirAll(ipnsMount, 0755)
		require.NoError(t, err)
		err = os.MkdirAll(mfsMount, 0755)
		require.NoError(t, err)

		// Clean up any existing mounts
		failOnError := false // mount points might not exist from previous runs
		doUnmount(t, ipfsMount, failOnError)
		doUnmount(t, ipnsMount, failOnError)
		doUnmount(t, mfsMount, failOnError)

		// Mount
		node.IPFS("mount", "-f", ipfsMount, "-n", ipnsMount, "-m", mfsMount)

		// Explicit unmount via platform-specific command
		failOnError = true // test that explicit unmount works correctly
		doUnmount(t, ipfsMount, failOnError)
		doUnmount(t, ipnsMount, failOnError)
		doUnmount(t, mfsMount, failOnError)

		// Verify directories can be removed after explicit unmount
		err = os.Remove(ipfsMount)
		require.NoError(t, err)
		err = os.Remove(ipnsMount)
		require.NoError(t, err)
		err = os.Remove(mfsMount)
		require.NoError(t, err)

		node.StopDaemon()
	})
}

// doUnmount performs platform-specific unmount, similar to sharness do_umount
// failOnError: if true, unmount errors cause test failure; if false, errors are ignored (useful for cleanup)
func doUnmount(t *testing.T, mountPoint string, failOnError bool) {
	t.Helper()
	var cmd *exec.Cmd
	if runtime.GOOS == "linux" {
		// fusermount -u: unmount filesystem (strict - fails if busy)
		cmd = exec.Command("fusermount", "-u", mountPoint)
	} else {
		cmd = exec.Command("umount", mountPoint)
	}

	err := cmd.Run()
	if err != nil && failOnError {
		t.Fatalf("failed to unmount %s: %v", mountPoint, err)
	}
}
