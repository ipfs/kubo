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
	root := NewFileSystem(ipfs, mountsCfg)
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
		root := NewFileSystem(ipfs, config.Mounts{})
		mntDir := testMount(t, root)

		f, err := os.Create(mntDir + "/testpersistence")
		require.NoError(t, err)
		_, err = f.Write(content)
		require.NoError(t, err)
		require.NoError(t, f.Close())
	})
	t.Run("read", func(t *testing.T) {
		root := NewFileSystem(ipfs, config.Mounts{})
		mntDir := testMount(t, root)

		got, err := os.ReadFile(mntDir + "/testpersistence")
		require.NoError(t, err)
		require.True(t, bytes.Equal(content, got))
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

	var got syscall.Statfs_t
	require.NoError(t, syscall.Statfs(mntDir, &got))

	var want syscall.Statfs_t
	require.NoError(t, syscall.Statfs(repoDir, &want))

	require.Equal(t, want.Blocks, got.Blocks, "total blocks should match the repo filesystem")
	require.Equal(t, want.Bfree, got.Bfree, "free blocks should match the repo filesystem")
}
