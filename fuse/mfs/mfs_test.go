//go:build (linux || darwin || freebsd) && !nofuse

// Unit tests for the /mfs FUSE mount.
// Generic writable operations are exercised by the shared suite in
// fusetest.RunWritableSuite. This file contains the mount factory
// and MFS-specific tests only.

package mfs

import (
	"bytes"
	"context"
	"crypto/rand"
	"os"
	"syscall"
	"testing"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/stretchr/testify/require"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/node"
	"github.com/ipfs/kubo/fuse/fusetest"
	fusemnt "github.com/ipfs/kubo/fuse/mount"
	"github.com/ipfs/kubo/fuse/writable"
)

func testMount(t *testing.T, root fs.InodeEmbedder) string {
	t.Helper()
	return fusetest.TestMount(t, root, &fs.Options{
		EntryTimeout: &mutableCacheTime,
		AttrTimeout:  &mutableCacheTime,
		MountOptions: fuse.MountOptions{
			MaxReadAhead:      fusemnt.MaxReadAhead,
			ExtraCapabilities: fusemnt.WritableMountCapabilities,
		},
	})
}

func mfsMount(t *testing.T, cfg writable.Config) string {
	t.Helper()
	ipfs, err := core.NewNode(context.Background(), &node.BuildCfg{})
	require.NoError(t, err)

	mountsCfg := config.Mounts{}
	if cfg.StoreMtime {
		mountsCfg.StoreMtime = config.True
	}
	if cfg.StoreMode {
		mountsCfg.StoreMode = config.True
	}
	root := NewFileSystem(ipfs, mountsCfg, config.Import{})
	return testMount(t, root)
}

func TestWritableSuite(t *testing.T) {
	fusetest.RunWritableSuite(t, mfsMount)
}

// TestPersistence verifies that file data survives unmount and remount
// on the same IpfsNode.
func TestPersistence(t *testing.T) {
	ipfs, err := core.NewNode(context.Background(), &node.BuildCfg{})
	require.NoError(t, err)

	content := make([]byte, 8196)
	_, err = rand.Read(content)
	require.NoError(t, err)

	t.Run("write", func(t *testing.T) {
		root := NewFileSystem(ipfs, config.Mounts{}, config.Import{})
		mntDir := testMount(t, root)

		f, err := os.Create(mntDir + "/testpersistence")
		require.NoError(t, err)
		_, err = f.Write(content)
		require.NoError(t, err)
		require.NoError(t, f.Close())
	})
	t.Run("read", func(t *testing.T) {
		root := NewFileSystem(ipfs, config.Mounts{}, config.Import{})
		mntDir := testMount(t, root)

		got, err := os.ReadFile(mntDir + "/testpersistence")
		require.NoError(t, err)
		require.True(t, bytes.Equal(content, got))
	})
}

// TestStatBlocks verifies that stat(2) on entries in /mfs populates
// st_blocks (used by du and ls -s) consistent with the file size, and
// that st_blksize advertises the chunker size MFS will use for writes
// so tools can align their I/O buffers.
func TestStatBlocks(t *testing.T) {
	const chunkerStr = "size-65536"
	const wantBlksize uint32 = 65536

	ipfs, err := core.NewNode(t.Context(), &node.BuildCfg{})
	require.NoError(t, err)

	kuboCfg := config.Import{UnixFSChunker: *config.NewOptionalString(chunkerStr)}
	root := NewFileSystem(ipfs, config.Mounts{}, kuboCfg)
	mntDir := testMount(t, root)

	t.Run("multi-block file", func(t *testing.T) {
		// >1 MiB ensures the UnixFS DAG has multiple leaves under the
		// configured 64 KiB chunker.
		content := make([]byte, 1024*1024+1)
		_, err := rand.Read(content)
		require.NoError(t, err)
		fpath := mntDir + "/big"
		require.NoError(t, os.WriteFile(fpath, content, 0o644))
		fusetest.AssertStatBlocks(t, fpath, wantBlksize)
	})

	t.Run("small single-chunk file", func(t *testing.T) {
		fpath := mntDir + "/small"
		require.NoError(t, os.WriteFile(fpath, []byte("hello"), 0o644))
		fusetest.AssertStatBlocks(t, fpath, wantBlksize)
	})

	t.Run("directory", func(t *testing.T) {
		dpath := mntDir + "/d"
		require.NoError(t, os.Mkdir(dpath, 0o755))
		info, err := os.Stat(dpath)
		require.NoError(t, err)
		st, ok := info.Sys().(*syscall.Stat_t)
		require.True(t, ok)
		require.EqualValues(t, 1, st.Blocks, "directory should report 1 nominal block")
		require.EqualValues(t, wantBlksize, st.Blksize)
	})

	t.Run("symlink", func(t *testing.T) {
		const target = "../some/target"
		lpath := mntDir + "/link"
		require.NoError(t, os.Symlink(target, lpath))
		info, err := os.Lstat(lpath)
		require.NoError(t, err)
		st, ok := info.Sys().(*syscall.Stat_t)
		require.True(t, ok)
		require.EqualValues(t, len(target), st.Size)
		require.EqualValues(t, 1, st.Blocks)
		require.EqualValues(t, wantBlksize, st.Blksize)
	})
}

// TestStatfs verifies that statfs on the /mfs mount reports the disk
// space of the repo's backing filesystem. macOS Finder refuses to copy
// files onto a volume that reports zero free space.
func TestStatfs(t *testing.T) {
	ipfs, err := core.NewNode(t.Context(), &node.BuildCfg{})
	require.NoError(t, err)

	// The default in-memory repo returns "" for Path(), so point
	// RepoPath at a real directory to exercise the syscall path.
	repoDir := t.TempDir()
	root := writable.NewDir(ipfs.FilesRoot.GetDirectory(), &writable.Config{
		DAG:      ipfs.DAG,
		RepoPath: repoDir,
	})
	mntDir := testMount(t, root)

	fusetest.AssertStatfsNonZero(t, mntDir)
}
