package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLsHuman --human/-H
func TestLsHuman(t *testing.T) {
	t.Parallel()

	t.Run("human default without flag shows numeric size", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		testDir := filepath.Join(node.Dir, "testdata")
		require.NoError(t, os.MkdirAll(testDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(testDir, "big.bin"), make([]byte, 2_000_000), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(testDir, "small.txt"), []byte("hello"), 0644))

		addRes := node.IPFS("add", "-r", "-Q", testDir)
		dirCid := addRes.Stdout.Trimmed()

		lsRes := node.IPFS("ls", dirCid)
		output := lsRes.Stdout.String()

		assert.Regexp(t, `\b2000000\b`, output, "default should show numeric 2000000")
		assert.Regexp(t, `\b5\b`, output, "default should show numeric 5")
	})

	t.Run("human shows SI units", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		testDir := filepath.Join(node.Dir, "testdata")
		require.NoError(t, os.MkdirAll(testDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(testDir, "big.bin"), make([]byte, 2_000_000), 0644))

		addRes := node.IPFS("add", "-r", "-Q", testDir)
		dirCid := addRes.Stdout.Trimmed()

		lsRes := node.IPFS("ls", "--human", dirCid)
		output := lsRes.Stdout.String()

		assert.Contains(t, output, "2.0 MB", "--human should show 2.0 MB for 2_000_000 bytes")
	})

	t.Run("human with short flag -H", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		testDir := filepath.Join(node.Dir, "testdata")
		require.NoError(t, os.MkdirAll(testDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(testDir, "big.bin"), make([]byte, 2_000_000), 0644))

		addRes := node.IPFS("add", "-r", "-Q", testDir)
		dirCid := addRes.Stdout.Trimmed()

		lsRes := node.IPFS("ls", "-H", dirCid)
		output := lsRes.Stdout.String()

		assert.Contains(t, output, "2.0 MB")
	})

	t.Run("human with size=false does not error", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		testDir := filepath.Join(node.Dir, "testdata")
		require.NoError(t, os.MkdirAll(testDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(testDir, "file.txt"), []byte("data"), 0644))

		addRes := node.IPFS("add", "-r", "-Q", testDir)
		dirCid := addRes.Stdout.Trimmed()

		lsRes := node.IPFS("ls", "--human", "--size=false", dirCid)
		assert.Equal(t, 0, lsRes.ExitCode())
		output := strings.TrimSpace(lsRes.Stdout.String())

		// --size=false drops the size column, so --human has nothing to format
		// and the line is just <cid> <name>.
		assert.Len(t, strings.Fields(output), 2, "expected only hash and name, got %q", output)
	})

	t.Run("human with --long", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		testDir := filepath.Join(node.Dir, "testdata")
		require.NoError(t, os.MkdirAll(testDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(testDir, "big.bin"), make([]byte, 2_000_000), 0644))

		addRes := node.IPFS("add", "-r", "-Q", testDir)
		dirCid := addRes.Stdout.Trimmed()

		lsRes := node.IPFS("ls", "--human", "--long", dirCid)
		output := lsRes.Stdout.String()

		assert.Contains(t, output, "2.0 MB")
	})

	t.Run("human does not change directory display", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		testDir := filepath.Join(node.Dir, "testdata")
		subDir := filepath.Join(testDir, "subdir")
		require.NoError(t, os.MkdirAll(subDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(subDir, "file.txt"), []byte("hi"), 0644))

		addRes := node.IPFS("add", "-r", "-Q", testDir)
		dirCid := addRes.Stdout.Trimmed()

		lsRes := node.IPFS("ls", "--human", dirCid)
		output := lsRes.Stdout.String()

		// Directories should still show "-" for size, not "0 B"
		assert.Contains(t, output, "- subdir/")
	})
}

// TestLsSortSize --sort-size/-S
func TestLsSortSize(t *testing.T) {
	t.Parallel()

	t.Run("sort-size orders by size descending", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		// Names are deliberately out of step with sizes. Name each file after
		// its size instead and both orders coincide, so the test would pass
		// even if --sort-size did nothing at all.
		testDir := filepath.Join(node.Dir, "testdata")
		require.NoError(t, os.MkdirAll(testDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(testDir, "a-small.bin"), make([]byte, 100), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(testDir, "m-large.bin"), make([]byte, 100_000), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(testDir, "z-medium.bin"), make([]byte, 500), 0644))

		addRes := node.IPFS("add", "-r", "-Q", testDir)
		dirCid := addRes.Stdout.Trimmed()

		// Default: sorted by name
		lsDefault := node.IPFS("ls", dirCid)
		defaultLines := strings.Split(strings.TrimSpace(lsDefault.Stdout.String()), "\n")
		require.Len(t, defaultLines, 3)
		assert.Contains(t, defaultLines[0], "a-small.bin")
		assert.Contains(t, defaultLines[1], "m-large.bin")
		assert.Contains(t, defaultLines[2], "z-medium.bin")

		// With --sort-size: sorted by size descending
		lsRes := node.IPFS("ls", "--sort-size", dirCid)
		lines := strings.Split(strings.TrimSpace(lsRes.Stdout.String()), "\n")
		require.Len(t, lines, 3)
		assert.Contains(t, lines[0], "m-large.bin", "largest file first")
		assert.Contains(t, lines[1], "z-medium.bin", "medium file second")
		assert.Contains(t, lines[2], "a-small.bin", "smallest file last")
	})

	t.Run("sort-size with short flag -S", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		// a- sorts before z- by name, so only a real size sort puts z-large first
		testDir := filepath.Join(node.Dir, "testdata")
		require.NoError(t, os.MkdirAll(testDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(testDir, "a-small.bin"), make([]byte, 100), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(testDir, "z-large.bin"), make([]byte, 100_000), 0644))

		addRes := node.IPFS("add", "-r", "-Q", testDir)
		dirCid := addRes.Stdout.Trimmed()

		lsRes := node.IPFS("ls", "-S", dirCid)
		lines := strings.Split(strings.TrimSpace(lsRes.Stdout.String()), "\n")
		require.Len(t, lines, 2)
		assert.Contains(t, lines[0], "z-large.bin", "largest file first with -S")
		assert.Contains(t, lines[1], "a-small.bin", "smallest file last with -S")
	})

	t.Run("sort-size ranks directories as size 0", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		// a-subdir holds content, but UnixFS gives directories no file size, so
		// it sorts as 0. That means it does not land strictly last: it ties with
		// the empty file and the tie breaks by name.
		testDir := filepath.Join(node.Dir, "testdata")
		subDir := filepath.Join(testDir, "a-subdir")
		require.NoError(t, os.MkdirAll(subDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(subDir, "file.txt"), make([]byte, 5_000), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(testDir, "m-empty.bin"), []byte{}, 0644))
		require.NoError(t, os.WriteFile(filepath.Join(testDir, "z-file.bin"), make([]byte, 100), 0644))

		addRes := node.IPFS("add", "-r", "-Q", testDir)
		dirCid := addRes.Stdout.Trimmed()

		lsRes := node.IPFS("ls", "--sort-size", dirCid)
		lines := strings.Split(strings.TrimSpace(lsRes.Stdout.String()), "\n")
		require.Len(t, lines, 3)

		assert.Contains(t, lines[0], "z-file.bin", "only non-empty file goes first")
		assert.Contains(t, lines[1], "a-subdir/", "directory ties at 0 and wins the name tiebreak")
		assert.Contains(t, lines[2], "m-empty.bin", "empty file ties with the directory")
	})

	t.Run("sort-size with stream is an error", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		testDir := filepath.Join(node.Dir, "testdata")
		require.NoError(t, os.MkdirAll(testDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(testDir, "file.txt"), []byte("data"), 0644))

		addRes := node.IPFS("add", "-r", "-Q", testDir)
		dirCid := addRes.Stdout.Trimmed()

		lsRes := node.RunIPFS("ls", "--sort-size", "--stream", dirCid)
		assert.NotEqual(t, 0, lsRes.ExitCode(), "--sort-size + --stream should fail")
		assert.Contains(t, lsRes.Stderr.String(), "cannot use --sort-size with --stream")
	})

	t.Run("sort-size with size=false is an error", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		testDir := filepath.Join(node.Dir, "testdata")
		require.NoError(t, os.MkdirAll(testDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(testDir, "file.txt"), []byte("data"), 0644))

		addRes := node.IPFS("add", "-r", "-Q", testDir)
		dirCid := addRes.Stdout.Trimmed()

		lsRes := node.RunIPFS("ls", "--sort-size", "--size=false", dirCid)
		assert.NotEqual(t, 0, lsRes.ExitCode(), "--sort-size + --size=false should fail")
		assert.Contains(t, lsRes.Stderr.String(), "cannot use --sort-size with --size=false")
	})

	t.Run("sort-size tie-breaker by name for equal sizes", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		testDir := filepath.Join(node.Dir, "testdata")
		require.NoError(t, os.MkdirAll(testDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(testDir, "b_file.bin"), make([]byte, 100), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(testDir, "a_file.bin"), make([]byte, 100), 0644))

		addRes := node.IPFS("add", "-r", "-Q", testDir)
		dirCid := addRes.Stdout.Trimmed()

		lsRes := node.IPFS("ls", "--sort-size", dirCid)
		lines := strings.Split(strings.TrimSpace(lsRes.Stdout.String()), "\n")
		require.Len(t, lines, 2)

		// Equal sizes: alphabetical order
		assert.Contains(t, lines[0], "a_file.bin")
		assert.Contains(t, lines[1], "b_file.bin")
	})
}
