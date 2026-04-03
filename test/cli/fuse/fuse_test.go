package fuse

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/ipfs/kubo/test/cli/testutils"
	"github.com/stretchr/testify/require"
)

func TestFUSE(t *testing.T) {
	testutils.RequiresFUSE(t)
	t.Parallel()

	t.Run("mount and unmount work correctly", func(t *testing.T) {
		t.Parallel()

		node := harness.NewT(t).NewNode().Init()
		node.StartDaemon()

		ipfsMount, ipnsMount, mfsMount := mountAll(t, node)

		// Test basic MFS functionality via FUSE mount
		testFile := filepath.Join(mfsMount, "testfile")
		testContent := "hello fuse world"

		err := os.WriteFile(testFile, []byte(testContent), 0644)
		require.NoError(t, err)

		// Verify file appears in MFS via IPFS commands
		result := node.IPFS("files", "ls", "/")
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

		// Stop daemon, which should trigger automatic unmount
		node.StopDaemon()

		// Verify directories can now be removed (indicating successful unmount)
		require.NoError(t, os.Remove(ipfsMount))
		require.NoError(t, os.Remove(ipnsMount))
		require.NoError(t, os.Remove(mfsMount))
	})

	t.Run("explicit unmount works", func(t *testing.T) {
		t.Parallel()

		node := harness.NewT(t).NewNode().Init()
		node.StartDaemon()

		ipfsMount, ipnsMount, mfsMount := mountAll(t, node)

		doUnmount(t, ipfsMount, true)
		doUnmount(t, ipnsMount, true)
		doUnmount(t, mfsMount, true)

		// Verify directories can be removed after explicit unmount
		require.NoError(t, os.Remove(ipfsMount))
		require.NoError(t, os.Remove(ipnsMount))
		require.NoError(t, os.Remove(mfsMount))

		node.StopDaemon()
	})

	t.Run("mount fails when dirs missing", func(t *testing.T) {
		t.Parallel()

		node := harness.NewT(t).NewNode().Init()
		node.StartDaemon()

		res := node.RunIPFS("mount", "-f=not_ipfs", "-n=not_ipns", "-m=not_mfs")
		require.Error(t, res.Err)
		require.Empty(t, res.Stdout.String())
		stderr := res.Stderr.String()
		require.True(t,
			strings.Contains(stderr, "not_ipfs") ||
				strings.Contains(stderr, "not_ipns") ||
				strings.Contains(stderr, "not_mfs"),
			"error should mention missing mount dir, got: %s", stderr)

		node.StopDaemon()
	})

	t.Run("IPNS local symlink", func(t *testing.T) {
		t.Parallel()

		node := harness.NewT(t).NewNode().Init()
		node.StartDaemon()

		_, ipnsMount, _ := mountAll(t, node)

		target, err := os.Readlink(filepath.Join(ipnsMount, "local"))
		require.NoError(t, err)
		require.Equal(t, node.PeerID().String(), filepath.Base(target))

		node.StopDaemon()
	})

	t.Run("IPNS name resolution via NS map", func(t *testing.T) {
		t.Parallel()

		node := harness.NewT(t).NewNode().Init()

		// Add content offline (before daemon starts)
		expectedFile := filepath.Join(node.Dir, "expected")
		require.NoError(t, os.WriteFile(expectedFile, []byte("ipfs"), 0644))
		wrappedCID := node.IPFS("add", "--cid-version", "1", "-Q", "-w", expectedFile).Stdout.Trimmed()

		// Set IPFS_NS_MAP so the daemon resolves welcome.example.com
		node.Runner.Env["IPFS_NS_MAP"] = "welcome.example.com:/ipfs/" + wrappedCID

		node.StartDaemon()
		_, ipnsMount, _ := mountAll(t, node)

		// Read the file through IPNS FUSE mount using the DNS name
		content, err := os.ReadFile(filepath.Join(ipnsMount, "welcome.example.com", "expected"))
		require.NoError(t, err)
		require.Equal(t, "ipfs", string(content))

		node.StopDaemon()
	})

	t.Run("MFS file and dir creation", func(t *testing.T) {
		t.Parallel()

		node := harness.NewT(t).NewNode().Init()
		node.StartDaemon()

		_, _, mfsMount := mountAll(t, node)

		// Create file via FUSE
		require.NoError(t, os.WriteFile(filepath.Join(mfsMount, "testfile"), []byte("content"), 0644))
		result := node.IPFS("files", "ls", "/")
		require.Contains(t, result.Stdout.String(), "testfile")

		// Create dir via FUSE
		require.NoError(t, os.Mkdir(filepath.Join(mfsMount, "testdir"), 0755))
		result = node.IPFS("files", "ls", "/")
		require.Contains(t, result.Stdout.String(), "testdir")

		node.StopDaemon()
	})

	t.Run("MFS xattr", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS != "linux" {
			t.Skip("xattr requires Linux")
		}

		node := harness.NewT(t).NewNode().Init()
		node.StartDaemon()

		_, _, mfsMount := mountAll(t, node)

		testFile := filepath.Join(mfsMount, "testfile")
		require.NoError(t, os.WriteFile(testFile, []byte("content"), 0644))

		cid, err := getXattr(testFile, "ipfs_cid")
		require.NoError(t, err)
		require.NotEmpty(t, cid)

		node.StopDaemon()
	})

	t.Run("files write then read via FUSE", func(t *testing.T) {
		t.Parallel()

		node := harness.NewT(t).NewNode().Init()
		node.StartDaemon()

		_, _, mfsMount := mountAll(t, node)

		// Write via ipfs files write -e, read back via FUSE
		node.PipeStrToIPFS("content3", "files", "write", "-e", "/testfile3")

		got, err := os.ReadFile(filepath.Join(mfsMount, "testfile3"))
		require.NoError(t, err)
		require.Equal(t, "content3", string(got))

		node.StopDaemon()
	})

	t.Run("add --to-files then read via FUSE", func(t *testing.T) {
		t.Parallel()

		node := harness.NewT(t).NewNode().Init()
		node.StartDaemon()

		_, _, mfsMount := mountAll(t, node)

		// Create a temp file to add
		tmpFile := filepath.Join(node.Dir, "testfile2")
		require.NoError(t, os.WriteFile(tmpFile, []byte("content"), 0644))

		node.IPFS("add", "--to-files", "/testfile2", tmpFile)

		got, err := os.ReadFile(filepath.Join(mfsMount, "testfile2"))
		require.NoError(t, err)
		require.Equal(t, "content", string(got))

		node.StopDaemon()
	})

	t.Run("file removal via FUSE", func(t *testing.T) {
		t.Parallel()

		node := harness.NewT(t).NewNode().Init()
		node.StartDaemon()

		_, _, mfsMount := mountAll(t, node)

		testFile := filepath.Join(mfsMount, "testfile")
		require.NoError(t, os.WriteFile(testFile, []byte("content"), 0644))

		result := node.IPFS("files", "ls", "/")
		require.Contains(t, result.Stdout.String(), "testfile")

		require.NoError(t, os.Remove(testFile))

		result = node.IPFS("files", "ls", "/")
		require.NotContains(t, result.Stdout.String(), "testfile")

		node.StopDaemon()
	})

	t.Run("nested dirs via FUSE", func(t *testing.T) {
		t.Parallel()

		node := harness.NewT(t).NewNode().Init()
		node.StartDaemon()

		_, _, mfsMount := mountAll(t, node)

		nested := filepath.Join(mfsMount, "foo", "bar", "baz", "qux")
		require.NoError(t, os.MkdirAll(nested, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(nested, "quux"), []byte("content"), 0644))

		result := node.IPFS("files", "stat", "/foo/bar/baz/qux/quux")
		require.NoError(t, result.Err)

		node.StopDaemon()
	})

	t.Run("publish blocked while IPNS mounted", func(t *testing.T) {
		t.Parallel()

		node := harness.NewT(t).NewNode().Init()
		node.StartDaemon()

		// Add content and publish before mount
		hash := node.PipeStrToIPFS("hello warld", "add", "-Q", "-w", "--stdin-name", "file").Stdout.Trimmed()
		node.IPFS("name", "publish", hash)

		// Mount all
		_, ipnsMount, _ := mountAll(t, node)

		// Publish should fail while IPNS is mounted
		res := node.RunIPFS("name", "publish", hash)
		require.Error(t, res.Err)
		require.Contains(t, res.Stderr.String(), "cannot manually publish while IPNS is mounted")

		// Unmount IPNS out-of-band
		doUnmount(t, ipnsMount, true)

		// Publish should work again
		node.IPFS("name", "publish", hash)

		node.StopDaemon()
	})

	t.Run("sharded directory read via FUSE", func(t *testing.T) {
		t.Parallel()

		node := harness.NewT(t).NewNode().Init()

		// Force sharding with 1B threshold
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Import.UnixFSHAMTDirectorySizeThreshold = *config.NewOptionalBytes("1B")
		})

		node.StartDaemon()
		ipfsMount, _, _ := mountAll(t, node)

		// Create test data directory
		testdataDir := filepath.Join(node.Dir, "testdata")
		require.NoError(t, os.MkdirAll(filepath.Join(testdataDir, "subdir"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(testdataDir, "a"), []byte("a\n"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(testdataDir, "subdir", "b"), []byte("b\n"), 0644))

		// Add sharded directory
		hash := node.IPFS("add", "-r", "-Q", testdataDir).Stdout.Trimmed()

		// Read files via FUSE /ipfs mount
		contentA, err := os.ReadFile(filepath.Join(ipfsMount, hash, "a"))
		require.NoError(t, err)
		require.Equal(t, "a\n", string(contentA))

		contentB, err := os.ReadFile(filepath.Join(ipfsMount, hash, "subdir", "b"))
		require.NoError(t, err)
		require.Equal(t, "b\n", string(contentB))

		// List directories via FUSE
		entries, err := os.ReadDir(filepath.Join(ipfsMount, hash))
		require.NoError(t, err)
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		sort.Strings(names)
		require.Equal(t, []string{"a", "subdir"}, names)

		subEntries, err := os.ReadDir(filepath.Join(ipfsMount, hash, "subdir"))
		require.NoError(t, err)
		require.Len(t, subEntries, 1)
		require.Equal(t, "b", subEntries[0].Name())

		node.StopDaemon()
	})
}

// mountAll creates mount directories and mounts IPFS, IPNS, and MFS.
func mountAll(t *testing.T, node *harness.Node) (ipfsMount, ipnsMount, mfsMount string) {
	t.Helper()
	ipfsMount = filepath.Join(node.Dir, "ipfs")
	ipnsMount = filepath.Join(node.Dir, "ipns")
	mfsMount = filepath.Join(node.Dir, "mfs")

	require.NoError(t, os.MkdirAll(ipfsMount, 0755))
	require.NoError(t, os.MkdirAll(ipnsMount, 0755))
	require.NoError(t, os.MkdirAll(mfsMount, 0755))

	// Clean up any stale mounts (non-fatal)
	doUnmount(t, ipfsMount, false)
	doUnmount(t, ipnsMount, false)
	doUnmount(t, mfsMount, false)

	result := node.IPFS("mount", "-f", ipfsMount, "-n", ipnsMount, "-m", mfsMount)

	expectedOutput := "IPFS mounted at: " + ipfsMount + "\n" +
		"IPNS mounted at: " + ipnsMount + "\n" +
		"MFS mounted at: " + mfsMount + "\n"
	require.Equal(t, expectedOutput, result.Stdout.String())

	return
}

// doUnmount performs platform-specific unmount, similar to sharness do_umount.
// If failOnError is true, unmount errors cause test failure; otherwise errors are ignored.
func doUnmount(t *testing.T, mountPoint string, failOnError bool) {
	t.Helper()
	var cmd *exec.Cmd
	if runtime.GOOS == "linux" {
		cmd = exec.Command("fusermount", "-u", mountPoint)
	} else {
		cmd = exec.Command("umount", mountPoint)
	}

	err := cmd.Run()
	if err != nil && failOnError {
		t.Fatalf("failed to unmount %s: %v", mountPoint, err)
	}
}
