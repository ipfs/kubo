package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLsLongFormat(t *testing.T) {
	t.Parallel()

	t.Run("long format shows mode and mtime when preserved", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		// Create a test directory structure with known permissions
		testDir := filepath.Join(node.Dir, "testdata")
		require.NoError(t, os.MkdirAll(testDir, 0755))

		// Create files with specific permissions
		file1 := filepath.Join(testDir, "readable.txt")
		require.NoError(t, os.WriteFile(file1, []byte("hello"), 0644))

		file2 := filepath.Join(testDir, "executable.sh")
		require.NoError(t, os.WriteFile(file2, []byte("#!/bin/sh\necho hi"), 0755))

		// Set a known mtime in the past (to get year format, avoiding flaky time-based tests)
		oldTime := time.Date(2020, time.June, 15, 10, 30, 0, 0, time.UTC)
		require.NoError(t, os.Chtimes(file1, oldTime, oldTime))
		require.NoError(t, os.Chtimes(file2, oldTime, oldTime))

		// Add with preserved mode and mtime
		addRes := node.IPFS("add", "-r", "--preserve-mode", "--preserve-mtime", "-Q", testDir)
		dirCid := addRes.Stdout.Trimmed()

		// Run ls with --long flag
		lsRes := node.IPFS("ls", "--long", dirCid)
		output := lsRes.Stdout.String()

		// Verify format: Mode Hash Size ModTime Name
		lines := strings.Split(strings.TrimSpace(output), "\n")
		require.Len(t, lines, 2, "expected 2 files in output")

		// Check executable.sh line (should be first alphabetically)
		assert.Contains(t, lines[0], "-rwxr-xr-x", "executable should have 755 permissions")
		assert.Contains(t, lines[0], "Jun 15  2020", "should show mtime with year format")
		assert.Contains(t, lines[0], "executable.sh", "should show filename")

		// Check readable.txt line
		assert.Contains(t, lines[1], "-rw-r--r--", "readable file should have 644 permissions")
		assert.Contains(t, lines[1], "Jun 15  2020", "should show mtime with year format")
		assert.Contains(t, lines[1], "readable.txt", "should show filename")
	})

	t.Run("long format shows dash for files without preserved mode or mtime", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		// Create and add a file without --preserve-mode or --preserve-mtime
		testFile := filepath.Join(node.Dir, "nopreserve.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0644))

		addRes := node.IPFS("add", "-Q", testFile)
		fileCid := addRes.Stdout.Trimmed()

		// Create a wrapper directory to list
		node.IPFS("files", "mkdir", "/testdir")
		node.IPFS("files", "cp", "/ipfs/"+fileCid, "/testdir/file.txt")
		statRes := node.IPFS("files", "stat", "--hash", "/testdir")
		dirCid := statRes.Stdout.Trimmed()

		// Run ls with --long flag
		lsRes := node.IPFS("ls", "--long", dirCid)
		output := lsRes.Stdout.String()

		// Files without preserved mode or mtime should show "-" for both columns
		// Format: "-" (mode) <CID> <size> "-" (mtime) <name>
		assert.Regexp(t, `^-\s+\S+\s+\d+\s+-\s+`, output, "missing mode and mtime should both show dash")
	})

	t.Run("long format with headers shows correct column order", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		// Create a simple test file
		testDir := filepath.Join(node.Dir, "headertest")
		require.NoError(t, os.MkdirAll(testDir, 0755))
		testFile := filepath.Join(testDir, "file.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("hello"), 0644))

		oldTime := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
		require.NoError(t, os.Chtimes(testFile, oldTime, oldTime))

		addRes := node.IPFS("add", "-r", "--preserve-mode", "--preserve-mtime", "-Q", testDir)
		dirCid := addRes.Stdout.Trimmed()

		// Run ls with --long and --headers (--size defaults to true)
		lsRes := node.IPFS("ls", "--long", "--headers", dirCid)
		output := lsRes.Stdout.String()
		lines := strings.Split(strings.TrimSpace(output), "\n")

		// First line should be headers in correct order: Mode Hash Size ModTime Name
		require.GreaterOrEqual(t, len(lines), 2)
		headerFields := strings.Fields(lines[0])
		require.Len(t, headerFields, 5, "header should have 5 columns")
		assert.Equal(t, "Mode", headerFields[0])
		assert.Equal(t, "Hash", headerFields[1])
		assert.Equal(t, "Size", headerFields[2])
		assert.Equal(t, "ModTime", headerFields[3])
		assert.Equal(t, "Name", headerFields[4])

		// Data line should have matching columns
		dataFields := strings.Fields(lines[1])
		require.GreaterOrEqual(t, len(dataFields), 5)
		assert.Regexp(t, `^-[rwx-]{9}$`, dataFields[0], "first field should be mode")
		assert.Regexp(t, `^Qm`, dataFields[1], "second field should be CID")
		assert.Regexp(t, `^\d+$`, dataFields[2], "third field should be size")
	})

	t.Run("long format with headers and size=false", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		testDir := filepath.Join(node.Dir, "headertest2")
		require.NoError(t, os.MkdirAll(testDir, 0755))
		testFile := filepath.Join(testDir, "file.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("hello"), 0644))

		oldTime := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
		require.NoError(t, os.Chtimes(testFile, oldTime, oldTime))

		addRes := node.IPFS("add", "-r", "--preserve-mode", "--preserve-mtime", "-Q", testDir)
		dirCid := addRes.Stdout.Trimmed()

		// Run ls with --long --headers --size=false
		lsRes := node.IPFS("ls", "--long", "--headers", "--size=false", dirCid)
		output := lsRes.Stdout.String()
		lines := strings.Split(strings.TrimSpace(output), "\n")

		// Header should be: Mode Hash ModTime Name (no Size)
		require.GreaterOrEqual(t, len(lines), 2)
		headerFields := strings.Fields(lines[0])
		require.Len(t, headerFields, 4, "header should have 4 columns without size")
		assert.Equal(t, "Mode", headerFields[0])
		assert.Equal(t, "Hash", headerFields[1])
		assert.Equal(t, "ModTime", headerFields[2])
		assert.Equal(t, "Name", headerFields[3])
	})

	t.Run("long format for directories shows trailing slash", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		// Create nested directory structure
		testDir := filepath.Join(node.Dir, "dirtest")
		subDir := filepath.Join(testDir, "subdir")
		require.NoError(t, os.MkdirAll(subDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(subDir, "file.txt"), []byte("hi"), 0644))

		addRes := node.IPFS("add", "-r", "--preserve-mode", "-Q", testDir)
		dirCid := addRes.Stdout.Trimmed()

		// Run ls with --long flag
		lsRes := node.IPFS("ls", "--long", dirCid)
		output := lsRes.Stdout.String()

		// Directory should end with /
		assert.Contains(t, output, "subdir/", "directory should have trailing slash")
		// Directory should show 'd' in mode
		assert.Contains(t, output, "drwxr-xr-x", "directory should show directory mode")
	})

	t.Run("long format without size flag", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		testDir := filepath.Join(node.Dir, "nosizetest")
		require.NoError(t, os.MkdirAll(testDir, 0755))
		testFile := filepath.Join(testDir, "file.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("hello world"), 0644))

		oldTime := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
		require.NoError(t, os.Chtimes(testFile, oldTime, oldTime))

		addRes := node.IPFS("add", "-r", "--preserve-mode", "--preserve-mtime", "-Q", testDir)
		dirCid := addRes.Stdout.Trimmed()

		// Run ls with --long but --size=false
		lsRes := node.IPFS("ls", "--long", "--size=false", dirCid)
		output := lsRes.Stdout.String()

		// Should still have mode and mtime, but format differs (no size column)
		assert.Contains(t, output, "-rw-r--r--")
		assert.Contains(t, output, "Jan 01  2020")
		assert.Contains(t, output, "file.txt")
	})

	t.Run("long format output is stable", func(t *testing.T) {
		// This test ensures the output format doesn't change due to refactors
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		testDir := filepath.Join(node.Dir, "stabletest")
		require.NoError(t, os.MkdirAll(testDir, 0755))
		testFile := filepath.Join(testDir, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("stable"), 0644))

		// Use a fixed time for reproducibility
		fixedTime := time.Date(2020, time.December, 25, 12, 0, 0, 0, time.UTC)
		require.NoError(t, os.Chtimes(testFile, fixedTime, fixedTime))

		addRes := node.IPFS("add", "-r", "--preserve-mode", "--preserve-mtime", "-Q", testDir)
		dirCid := addRes.Stdout.Trimmed()

		// The CID should be deterministic given same content, mode, and mtime
		// This is the expected CID for this specific test data
		lsRes := node.IPFS("ls", "--long", dirCid)
		output := strings.TrimSpace(lsRes.Stdout.String())

		// Verify the format: Mode<tab>Hash<tab>Size<tab>ModTime<tab>Name
		fields := strings.Fields(output)
		require.GreaterOrEqual(t, len(fields), 5, "output should have at least 5 fields")

		// Field 0: mode (10 chars, starts with - for regular file)
		assert.Regexp(t, `^-[rwx-]{9}$`, fields[0], "mode should be Unix permission format")

		// Field 1: CID (starts with Qm or bafy)
		assert.Regexp(t, `^(Qm|bafy)`, fields[1], "second field should be CID")

		// Field 2: size (numeric)
		assert.Regexp(t, `^\d+$`, fields[2], "third field should be numeric size")

		// Fields 3-4: date (e.g., "Dec 25  2020" or "Dec 25 12:00")
		// The date format is "Mon DD  YYYY" for old files
		assert.Equal(t, "Dec", fields[3])
		assert.Equal(t, "25", fields[4])

		// Last field: filename
		assert.Equal(t, "test.txt", fields[len(fields)-1])
	})
}
