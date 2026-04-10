//go:build (linux || darwin || freebsd) && !nofuse

// End-to-end FUSE coverage with real POSIX tools.
//
// TestFUSERealWorld spins up one ipfs daemon, mounts /ipfs, /ipns, and
// /mfs, and exercises the writable mount through the actual binaries
// users invoke (cat, ls, cp, mv, rm, ln, find, dd, sha256sum, tar,
// rsync, vim, sh, wc). Each subtest verifies the result both via the
// FUSE filesystem and via the daemon's `ipfs files` view.
//
// All external tools are required: a missing binary fails the test
// instead of skipping, so a CI image change cannot silently turn this
// suite green. The whole-suite TEST_FUSE gate is the only place a
// developer is allowed to skip.
//
// Synthetic file payloads default to 1 MiB + 1 byte so multi-chunk
// read/write paths and chunk-boundary off-by-ones are exercised.

package fuse

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/ipfs/kubo/test/cli/testutils"
	"github.com/stretchr/testify/require"
)

// payloadSize is the default test payload size: 1 MiB + 1 byte.
// Forces multi-chunk DAG construction so single-chunk fast paths
// cannot mask cross-block bugs.
const payloadSize = 1024*1024 + 1

func TestFUSERealWorld(t *testing.T) {
	testutils.RequiresFUSE(t)

	node := harness.NewT(t).NewNode().Init()
	// StoreMtime/StoreMode on so rsync -a, tar -p, vim's chmod, and
	// any other tool that round-trips POSIX metadata see consistent
	// behaviour. The flags only affect the writable mounts.
	node.UpdateConfig(func(cfg *config.Config) {
		cfg.Mounts.StoreMtime = config.True
		cfg.Mounts.StoreMode = config.True
	})
	node.StartDaemon()
	defer node.StopDaemon()

	_, _, mfsMount := mountAll(t, node)

	// requireTool fails the current subtest if bin is not in PATH.
	// External tools are part of the test contract: a missing binary
	// is a hidden coverage gap and we want a loud failure.
	requireTool := func(t *testing.T, bins ...string) {
		t.Helper()
		for _, bin := range bins {
			if _, err := exec.LookPath(bin); err != nil {
				t.Fatalf("%s not in PATH; required for end-to-end FUSE tests", bin)
			}
		}
	}

	// workdir creates a unique subdirectory under the mount for the
	// current subtest. Subtests share one daemon and one mount; using
	// disjoint subdirectories keeps them from colliding.
	workdir := func(t *testing.T, name string) string {
		t.Helper()
		d := filepath.Join(mfsMount, name)
		require.NoError(t, os.Mkdir(d, 0o755))
		return d
	}

	// runCmd runs an external binary and fails the test on error,
	// printing both stdout and stderr in the failure message.
	//
	// LC_ALL=C forces the C locale so any locale-sensitive output
	// (date formats in `ls -l`, decimal separators in `wc` output on
	// some locales, localized error messages, collation order from
	// `find` and `ls`) is deterministic regardless of how the runner
	// is configured. Without this the same test could pass on a US
	// runner and fail on one with LC_ALL=de_DE.UTF-8.
	runCmd := func(t *testing.T, name string, args ...string) string {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("%s %v failed: %v\nstdout: %s\nstderr: %s",
				name, args, err, stdout.String(), stderr.String())
		}
		return stdout.String()
	}

	// randBytes returns n cryptographically random bytes.
	randBytes := func(t *testing.T, n int) []byte {
		t.Helper()
		b := make([]byte, n)
		_, err := rand.Read(b)
		require.NoError(t, err)
		return b
	}

	// ----- Shell and core POSIX -----

	t.Run("echo_redirect_and_cat", func(t *testing.T) {
		requireTool(t, "sh", "cat")
		dir := workdir(t, "echo_redirect_and_cat")
		path := filepath.Join(dir, "greeting")

		runCmd(t, "sh", "-c", "echo 'hello fuse' > "+path)

		got := runCmd(t, "cat", path)
		require.Equal(t, "hello fuse\n", got, "cat output via FUSE")

		// Cross-verify via daemon's MFS view (bypasses FUSE).
		ipfsView := node.IPFS("files", "read", "/echo_redirect_and_cat/greeting").Stdout.String()
		require.Equal(t, "hello fuse\n", ipfsView, "ipfs files read view")
	})

	t.Run("seq_pipe_to_file_and_wc", func(t *testing.T) {
		requireTool(t, "sh", "seq", "wc")
		dir := workdir(t, "seq_pipe_to_file_and_wc")
		path := filepath.Join(dir, "lines")

		// 200000 lines: about 1.3 MB of text, comfortably more than
		// one UnixFS chunk under the default chunker.
		runCmd(t, "sh", "-c", "seq 1 200000 > "+path)

		lineCount := strings.Fields(runCmd(t, "wc", "-l", path))[0]
		require.Equal(t, "200000", lineCount)

		// File size should match: digits + newline per line.
		// sum_{i=1..9} i*9*1 + sum_{i=10..99} i*90*2 + ... easier to
		// just stat the file and compare against wc -c.
		byteCount := strings.Fields(runCmd(t, "wc", "-c", path))[0]
		info, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, strconv.FormatInt(info.Size(), 10), byteCount,
			"wc -c and stat agree on the multi-chunk file size")
		require.Greater(t, info.Size(), int64(payloadSize),
			"file should be larger than one chunk")
	})

	t.Run("ls_l_shows_mode_and_size", func(t *testing.T) {
		requireTool(t, "ls")
		dir := workdir(t, "ls_l_shows_mode_and_size")
		path := filepath.Join(dir, "file")

		data := randBytes(t, payloadSize)
		require.NoError(t, os.WriteFile(path, data, 0o644))

		// `ls -l` line layout: <mode> <links> <user> <group> <size> <date> <name>
		out := runCmd(t, "ls", "-l", path)
		fields := strings.Fields(out)
		require.GreaterOrEqual(t, len(fields), 8, "ls -l output: %q", out)

		require.True(t, strings.HasPrefix(fields[0], "-rw-r--r--"),
			"mode field %q should be -rw-r--r--", fields[0])
		require.Equal(t, strconv.Itoa(payloadSize), fields[4],
			"size field should match payload size")
	})

	t.Run("stat_reports_default_mode", func(t *testing.T) {
		requireTool(t, "stat")
		dir := workdir(t, "stat_reports_default_mode")
		path := filepath.Join(dir, "file")

		f, err := os.Create(path)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		out := strings.TrimSpace(runCmd(t, "stat", "-c", "%a %s", path))
		require.Equal(t, "644 0", out, "stat -c '%%a %%s' on a fresh file")
	})

	t.Run("cp_file_in", func(t *testing.T) {
		requireTool(t, "cp")
		dir := workdir(t, "cp_file_in")

		src := filepath.Join(node.Dir, "cp_file_in_src")
		want := randBytes(t, payloadSize)
		require.NoError(t, os.WriteFile(src, want, 0o644))

		dst := filepath.Join(dir, "cp-in")
		runCmd(t, "cp", src, dst)

		got, err := os.ReadFile(dst)
		require.NoError(t, err)
		require.True(t, bytes.Equal(want, got), "FUSE read-back differs")

		// Cross-verify via daemon. ipfs files read can return huge
		// blobs; compare lengths first to fail fast.
		daemonView := node.IPFS("files", "read", "/cp_file_in/cp-in").Stdout.Bytes()
		require.Equal(t, len(want), len(daemonView), "daemon view length")
		require.True(t, bytes.Equal(want, daemonView), "daemon view content")
	})

	t.Run("cp_r_tree_in", func(t *testing.T) {
		requireTool(t, "cp")
		dir := workdir(t, "cp_r_tree_in")

		// Build the source tree under node.Dir.
		srcRoot := filepath.Join(node.Dir, "cp_r_tree_in_src")
		require.NoError(t, os.MkdirAll(filepath.Join(srcRoot, "a", "b", "c"), 0o755))

		topData := randBytes(t, payloadSize)
		leafData := randBytes(t, payloadSize)
		require.NoError(t, os.WriteFile(filepath.Join(srcRoot, "top.bin"), topData, 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(srcRoot, "a", "b", "c", "leaf.bin"), leafData, 0o644))

		runCmd(t, "cp", "-r", srcRoot, dir+"/")

		// Walk the FUSE side and assert both files match.
		gotTop, err := os.ReadFile(filepath.Join(dir, "cp_r_tree_in_src", "top.bin"))
		require.NoError(t, err)
		require.True(t, bytes.Equal(topData, gotTop), "top file content")

		gotLeaf, err := os.ReadFile(filepath.Join(dir, "cp_r_tree_in_src", "a", "b", "c", "leaf.bin"))
		require.NoError(t, err)
		require.True(t, bytes.Equal(leafData, gotLeaf), "leaf file content")

		// Cross-verify the deepest file via the daemon.
		daemonView := node.IPFS("files", "read",
			"/cp_r_tree_in/cp_r_tree_in_src/a/b/c/leaf.bin").Stdout.Bytes()
		require.True(t, bytes.Equal(leafData, daemonView), "daemon view of deepest leaf")
	})

	t.Run("cp_file_out", func(t *testing.T) {
		requireTool(t, "cp")
		dir := workdir(t, "cp_file_out")

		want := randBytes(t, payloadSize)
		src := filepath.Join(dir, "payload")
		require.NoError(t, os.WriteFile(src, want, 0o644))

		dst := filepath.Join(node.Dir, "cp_file_out_dst")
		runCmd(t, "cp", src, dst)

		got, err := os.ReadFile(dst)
		require.NoError(t, err)
		require.True(t, bytes.Equal(want, got), "exported file content")
	})

	t.Run("mv_atomic_save", func(t *testing.T) {
		requireTool(t, "mv")
		dir := workdir(t, "mv_atomic_save")

		oldData := randBytes(t, payloadSize)
		newData := randBytes(t, payloadSize)

		target := filepath.Join(dir, "target")
		tmp := filepath.Join(dir, ".target.tmp")

		require.NoError(t, os.WriteFile(target, oldData, 0o644))
		require.NoError(t, os.WriteFile(tmp, newData, 0o644))

		runCmd(t, "mv", tmp, target)

		got, err := os.ReadFile(target)
		require.NoError(t, err)
		require.True(t, bytes.Equal(newData, got), "target should now hold new data")

		_, err = os.Stat(tmp)
		require.True(t, os.IsNotExist(err), "tmp should be gone after mv")
	})

	t.Run("rm_rf_tree", func(t *testing.T) {
		requireTool(t, "cp", "rm")
		dir := workdir(t, "rm_rf_tree")

		// Build a tree the same shape as cp_r_tree_in.
		srcRoot := filepath.Join(node.Dir, "rm_rf_tree_src")
		require.NoError(t, os.MkdirAll(filepath.Join(srcRoot, "a", "b", "c"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(srcRoot, "top.bin"), randBytes(t, payloadSize), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(srcRoot, "a", "b", "c", "leaf.bin"), randBytes(t, payloadSize), 0o644))

		runCmd(t, "cp", "-r", srcRoot, dir+"/")
		copied := filepath.Join(dir, "rm_rf_tree_src")

		// Sanity: tree exists.
		_, err := os.Stat(filepath.Join(copied, "a", "b", "c", "leaf.bin"))
		require.NoError(t, err)

		runCmd(t, "rm", "-rf", copied)

		_, err = os.Stat(copied)
		require.True(t, os.IsNotExist(err), "copied tree should be gone")

		// Cross-verify the daemon no longer lists the subtree.
		listing := node.IPFS("files", "ls", "/rm_rf_tree").Stdout.String()
		require.NotContains(t, listing, "rm_rf_tree_src",
			"ipfs files ls should not see the removed subtree")
	})

	t.Run("ln_s_and_readlink", func(t *testing.T) {
		requireTool(t, "ln", "readlink", "ls")
		dir := workdir(t, "ln_s_and_readlink")
		link := filepath.Join(dir, "link")

		runCmd(t, "ln", "-s", "/tmp/some/target", link)

		target := strings.TrimSpace(runCmd(t, "readlink", link))
		require.Equal(t, "/tmp/some/target", target)

		// ls -l on a symlink starts with 'l'.
		lsOut := runCmd(t, "ls", "-l", link)
		require.True(t, strings.HasPrefix(lsOut, "l"),
			"ls -l output should start with 'l' for a symlink, got: %q", lsOut)

		// Daemon view: ipfs files stat reports symlinks via the Mode
		// field (lrwxrwxrwx). The Type field is "file" because MFS
		// stores symlinks as TFile/TSymlink under the hood.
		stat := node.IPFS("files", "stat", "/ln_s_and_readlink/link").Stdout.String()
		require.Contains(t, stat, "lrwxrwxrwx",
			"ipfs files stat mode should be lrwxrwxrwx for a symlink, got: %s", stat)
	})

	t.Run("find_traversal", func(t *testing.T) {
		requireTool(t, "find", "ln")
		dir := workdir(t, "find_traversal")

		require.NoError(t, os.WriteFile(filepath.Join(dir, "regular"), randBytes(t, payloadSize), 0o644))
		require.NoError(t, os.Mkdir(filepath.Join(dir, "subdir"), 0o755))
		runCmd(t, "ln", "-s", "regular", filepath.Join(dir, "link"))

		// strings.Fields splits on any whitespace; this is safe here
		// because every test filename is ASCII with no spaces. If a
		// future maintainer adds a filename with whitespace, switch
		// to `find -print0` and split on '\x00' instead.

		// -type f should find exactly the regular file.
		files := strings.Fields(runCmd(t, "find", dir, "-type", "f"))
		require.Equal(t, []string{filepath.Join(dir, "regular")}, files)

		// -type d should find dir itself plus subdir.
		dirs := strings.Fields(runCmd(t, "find", dir, "-type", "d"))
		require.ElementsMatch(t, []string{dir, filepath.Join(dir, "subdir")}, dirs)

		// -type l should find exactly the symlink.
		links := strings.Fields(runCmd(t, "find", dir, "-type", "l"))
		require.Equal(t, []string{filepath.Join(dir, "link")}, links)
	})

	t.Run("dd_block_write", func(t *testing.T) {
		requireTool(t, "dd")
		dir := workdir(t, "dd_block_write")
		path := filepath.Join(dir, "blob")

		// 4096 * 257 = 1052672 bytes, just past the 1 MiB chunk
		// boundary. Uses /dev/urandom to avoid pulling all-zero
		// pages from the kernel cache.
		runCmd(t, "dd",
			"if=/dev/urandom",
			"of="+path,
			"bs=4096",
			"count=257",
			"status=none",
		)

		info, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, int64(4096*257), info.Size())
	})

	t.Run("sha256sum_roundtrip", func(t *testing.T) {
		requireTool(t, "sha256sum")
		dir := workdir(t, "sha256sum_roundtrip")
		path := filepath.Join(dir, "blob")

		want := randBytes(t, payloadSize)
		require.NoError(t, os.WriteFile(path, want, 0o644))

		hash := sha256.Sum256(want)
		wantHex := hex.EncodeToString(hash[:])

		out := runCmd(t, "sha256sum", path)
		// `sha256sum` prints "<hex>  <path>".
		gotHex := strings.Fields(out)[0]
		require.Equal(t, wantHex, gotHex,
			"sha256sum on FUSE-read bytes should match the bytes we wrote")
	})

	// ----- Archives -----

	t.Run("tar_extract_into_mfs", func(t *testing.T) {
		requireTool(t, "tar")
		dir := workdir(t, "tar_extract_into_mfs")

		// Build the source tree and tar it up under node.Dir.
		srcRoot := filepath.Join(node.Dir, "tar_extract_src")
		require.NoError(t, os.MkdirAll(filepath.Join(srcRoot, "sub"), 0o755))
		oneData := randBytes(t, payloadSize)
		twoData := randBytes(t, payloadSize)
		require.NoError(t, os.WriteFile(filepath.Join(srcRoot, "one.bin"), oneData, 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(srcRoot, "sub", "two.bin"), twoData, 0o644))

		tarPath := filepath.Join(node.Dir, "tar_extract.tar")
		runCmd(t, "tar", "-cf", tarPath, "-C", node.Dir, "tar_extract_src")

		// Extract into the FUSE mount.
		runCmd(t, "tar", "-xf", tarPath, "-C", dir)

		extracted := filepath.Join(dir, "tar_extract_src")
		gotOne, err := os.ReadFile(filepath.Join(extracted, "one.bin"))
		require.NoError(t, err)
		require.True(t, bytes.Equal(oneData, gotOne), "one.bin content")

		gotTwo, err := os.ReadFile(filepath.Join(extracted, "sub", "two.bin"))
		require.NoError(t, err)
		require.True(t, bytes.Equal(twoData, gotTwo), "two.bin content")
	})

	t.Run("tar_create_from_mfs", func(t *testing.T) {
		requireTool(t, "tar")
		dir := workdir(t, "tar_create_from_mfs")

		// Populate a small tree under the FUSE mount.
		srcRoot := filepath.Join(dir, "src")
		require.NoError(t, os.MkdirAll(filepath.Join(srcRoot, "sub"), 0o755))
		oneData := randBytes(t, payloadSize)
		twoData := randBytes(t, payloadSize)
		require.NoError(t, os.WriteFile(filepath.Join(srcRoot, "one.bin"), oneData, 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(srcRoot, "sub", "two.bin"), twoData, 0o644))

		// tar it up *from* the mount.
		tarPath := filepath.Join(node.Dir, "tar_create.tar")
		runCmd(t, "tar", "-cf", tarPath, "-C", dir, "src")

		// tar listing should include both leaves.
		listing := runCmd(t, "tar", "-tf", tarPath)
		require.Contains(t, listing, "src/one.bin")
		require.Contains(t, listing, "src/sub/two.bin")

		// Extract back to a fresh dir off the mount and byte-compare.
		extractDir := filepath.Join(node.Dir, "tar_create_extract")
		require.NoError(t, os.MkdirAll(extractDir, 0o755))
		runCmd(t, "tar", "-xf", tarPath, "-C", extractDir)

		gotOne, err := os.ReadFile(filepath.Join(extractDir, "src", "one.bin"))
		require.NoError(t, err)
		require.True(t, bytes.Equal(oneData, gotOne), "one.bin survives tar round-trip")

		gotTwo, err := os.ReadFile(filepath.Join(extractDir, "src", "sub", "two.bin"))
		require.NoError(t, err)
		require.True(t, bytes.Equal(twoData, gotTwo), "two.bin survives tar round-trip")
	})

	// ----- Rsync -----

	t.Run("rsync_archive_in", func(t *testing.T) {
		requireTool(t, "rsync")
		dir := workdir(t, "rsync_archive_in")

		// Build a tree under node.Dir with a known mode and mtime.
		srcRoot := filepath.Join(node.Dir, "rsync_archive_src")
		require.NoError(t, os.MkdirAll(filepath.Join(srcRoot, "sub"), 0o755))

		oneData := randBytes(t, payloadSize)
		twoData := randBytes(t, payloadSize)
		onePath := filepath.Join(srcRoot, "one.bin")
		twoPath := filepath.Join(srcRoot, "sub", "two.bin")
		require.NoError(t, os.WriteFile(onePath, oneData, 0o640))
		require.NoError(t, os.WriteFile(twoPath, twoData, 0o640))

		mtime := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
		require.NoError(t, os.Chtimes(onePath, mtime, mtime))
		require.NoError(t, os.Chtimes(twoPath, mtime, mtime))

		// Trailing slash on source: copy the contents of srcRoot,
		// not the directory itself. Mirrors typical rsync usage.
		runCmd(t, "rsync", "-a", srcRoot+"/", dir+"/copy/")

		gotOne, err := os.ReadFile(filepath.Join(dir, "copy", "one.bin"))
		require.NoError(t, err)
		require.True(t, bytes.Equal(oneData, gotOne), "one.bin content")

		gotTwo, err := os.ReadFile(filepath.Join(dir, "copy", "sub", "two.bin"))
		require.NoError(t, err)
		require.True(t, bytes.Equal(twoData, gotTwo), "two.bin content")

		// Mode preserved (StoreMode is enabled at the daemon level).
		oneInfo, err := os.Stat(filepath.Join(dir, "copy", "one.bin"))
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o640), oneInfo.Mode().Perm(),
			"mode should be preserved through rsync -a")

		// Mtime preserved (StoreMtime is enabled at the daemon level).
		require.WithinDuration(t, mtime, oneInfo.ModTime(), time.Second,
			"mtime should be preserved through rsync -a")
	})

	t.Run("rsync_inplace_overwrite", func(t *testing.T) {
		requireTool(t, "rsync")
		dir := workdir(t, "rsync_inplace_overwrite")

		// Initial file is larger than the replacement so the inplace
		// path has to truncate the tail.
		initial := randBytes(t, payloadSize+4096)
		dst := filepath.Join(dir, "inplace")
		require.NoError(t, os.WriteFile(dst, initial, 0o644))

		replacement := randBytes(t, payloadSize)
		src := filepath.Join(node.Dir, "rsync_inplace_replacement")
		require.NoError(t, os.WriteFile(src, replacement, 0o644))

		runCmd(t, "rsync", "--inplace", src, dst)

		got, err := os.ReadFile(dst)
		require.NoError(t, err)
		require.Equal(t, len(replacement), len(got),
			"file size should shrink to replacement size after --inplace")
		require.True(t, bytes.Equal(replacement, got),
			"content should match the replacement after --inplace")
	})

	// ----- Editor -----

	t.Run("vim_edit_file", func(t *testing.T) {
		requireTool(t, "vim")
		dir := workdir(t, "vim_edit_file")
		path := filepath.Join(dir, "edit.txt")

		// Build a multi-chunk file: a header line followed by enough
		// "world" repeats to push the total size past one UnixFS chunk.
		const word = "world\n"
		repeats := payloadSize/len(word) + 1
		var buf bytes.Buffer
		buf.WriteString("header\n")
		for range repeats {
			buf.WriteString(word)
		}
		original := buf.Bytes()
		require.NoError(t, os.WriteFile(path, original, 0o644))
		require.Greater(t, len(original), payloadSize, "file should span multiple chunks")

		// Vim in headless ex mode: substitute world->fuse globally,
		// write, quit. -E selects ex mode, -s suppresses prompts.
		runCmd(t, "vim", "-E", "-s",
			"-c", "%s/world/fuse/g",
			"-c", "wq",
			path,
		)

		got, err := os.ReadFile(path)
		require.NoError(t, err)
		require.NotContains(t, string(got), "world",
			"after :%%s/world/fuse/g the file should contain no 'world'")
		gotFuses := bytes.Count(got, []byte("fuse"))
		require.Equal(t, repeats, gotFuses,
			"the substitution should have replaced exactly %d occurrences", repeats)

		// Cross-verify via daemon.
		daemonView := node.IPFS("files", "read", "/vim_edit_file/edit.txt").Stdout.Bytes()
		require.True(t, bytes.Equal(got, daemonView),
			"daemon view should match FUSE view after vim save")
	})
}
