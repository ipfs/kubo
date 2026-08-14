//go:build (linux || darwin || freebsd) && !nofuse

// Unit tests for the /ipns FUSE mount.
// Generic writable operations are exercised by the shared suite in
// fusetest.RunWritableSuite. This file contains the mount factory
// and IPNS-specific tests only.

package ipns

import (
	"bytes"
	"context"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/stretchr/testify/require"

	"github.com/ipfs/boxo/mfs"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	coreapi "github.com/ipfs/kubo/core/coreapi"
	iface "github.com/ipfs/kubo/core/coreiface"
	"github.com/ipfs/kubo/fuse/fusetest"
	fusemnt "github.com/ipfs/kubo/fuse/mount"
	"github.com/ipfs/kubo/fuse/writable"
)

type mountWrap struct {
	Dir    string
	Root   *Root
	server *fuse.Server
	closed bool
}

func (m *mountWrap) Close() {
	if m.closed {
		return
	}
	m.closed = true
	if m.server != nil {
		_ = m.server.Unmount()
	}
	_ = m.Root.Close()
}

// fakeMount is a minimal mount.Mount that reports itself as active.
// This simulates the real daemon path where node.Mounts.Ipns is set
// after the FUSE filesystem is mounted, ensuring that checkPublishAllowed
// is actually exercised during tests (see issue #2168).
type fakeMount struct{}

func (fakeMount) MountPoint() string { return "/fake/ipns" }
func (fakeMount) Unmount() error     { return nil }
func (fakeMount) IsActive() bool     { return true }

func setupIpnsTest(t *testing.T, nd *core.IpfsNode, cfgs ...config.Mounts) (*core.IpfsNode, *mountWrap) {
	t.Helper()
	fusetest.SkipUnlessFUSE(t)

	var cfg config.Mounts
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	}

	var err error
	if nd == nil {
		nd, err = core.NewNode(context.Background(), &core.BuildCfg{})
		require.NoError(t, err)

		err = InitializeKeyspace(nd, nd.PrivateKey)
		require.NoError(t, err)
	}

	coreAPI, err := coreapi.NewCoreAPI(nd)
	require.NoError(t, err)

	key, err := coreAPI.Key().Self(nd.Context())
	require.NoError(t, err)

	// Settle the repo path before the mount starts serving. Statfs reads it
	// from a FUSE handler goroutine, so a test that assigns it afterwards
	// races the server. The in-memory test repo reports no path, and Statfs
	// needs a real directory to stat.
	repoPath := nd.Repo.Path()
	if repoPath == "" {
		repoPath = t.TempDir()
	}

	root, err := CreateRoot(nd.Context(), coreAPI, nd.Blockstore, map[string]iface.Key{"local": key}, "", "", repoPath, cfg, config.Import{})
	require.NoError(t, err)

	mntDir := t.TempDir()
	server, err := fs.Mount(mntDir, root, &fs.Options{
		NullPermissions: true,
		UID:             uint32(os.Getuid()),
		GID:             uint32(os.Getgid()),
		EntryTimeout:    &mutableCacheTime,
		AttrTimeout:     &mutableCacheTime,
		MountOptions: fuse.MountOptions{
			FsName:            "kubo-test",
			MaxReadAhead:      fusemnt.MaxReadAhead,
			ExtraCapabilities: fusemnt.WritableMountCapabilities,
		},
	})
	fusetest.MountError(t, err)

	mnt := &mountWrap{Dir: mntDir, Root: root, server: server}
	t.Cleanup(mnt.Close)

	nd.Mounts.Ipns = fakeMount{}
	return nd, mnt
}

// newIpnsMount is the factory for the shared writable suite. It creates
// an IPNS mount and returns the writable /local directory path.
func newIpnsMount(t *testing.T, cfg writable.Config) string {
	t.Helper()
	mountsCfg := config.Mounts{}
	if cfg.StoreMtime {
		mountsCfg.StoreMtime = config.True
	}
	if cfg.StoreMode {
		mountsCfg.StoreMode = config.True
	}
	_, mnt := setupIpnsTest(t, nil, mountsCfg)
	return mnt.Dir + "/local"
}

func TestWritableSuite(t *testing.T) {
	fusetest.RunWritableSuite(t, newIpnsMount)
}

// TestIpnsLocalLink verifies that /ipns/local is a symlink to the
// node's own peer ID directory.
func TestIpnsLocalLink(t *testing.T) {
	nd, mnt := setupIpnsTest(t, nil)

	target, err := os.Readlink(mnt.Dir + "/local")
	require.NoError(t, err)
	require.Equal(t, nd.Identity.String(), target)
}

// TestRenameOntoNamespaceRoot moves a file from a key directory onto the
// /ipns root, which holds no files and cannot take it. The move has to fail
// with the file still where it was: the mount used to unlink the source
// before finding out it had nowhere to put it, and the file was gone.
func TestRenameOntoNamespaceRoot(t *testing.T) {
	nd, mnt := setupIpnsTest(t, nil)
	keyDir := mnt.Dir + "/" + nd.Identity.String()

	src := keyDir + "/keepme"
	content := []byte("still here")
	require.NoError(t, os.WriteFile(src, content, 0o644))

	require.Error(t, os.Rename(src, mnt.Dir+"/keepme"))

	// Ask MFS, not the mount. The kernel still has the entry cached, so a
	// read through the mount answers from the handle it already holds and
	// succeeds for a second either way, whether or not the file is still
	// in the tree.
	_, err := mfs.Lookup(mnt.Root.Roots[nd.Identity.String()], "/keepme")
	require.NoError(t, err, "the file must survive a rename that could not be carried out")

	got, err := os.ReadFile(src)
	require.NoError(t, err)
	require.Equal(t, content, got)
}

// TestNamespaceRootMode verifies that the /ipns root has execute-only
// mode (not listable, only traversable).
func TestNamespaceRootMode(t *testing.T) {
	_, mnt := setupIpnsTest(t, nil)

	info, err := os.Stat(mnt.Dir)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o111), info.Mode().Perm())
}

// TestKeyDirAttrs verifies that a key directory served by the /ipns root
// reports the same attributes as any other directory on a writable mount: a
// stable inode number of its own, a link count, and real permissions. The
// root answers these lookups itself rather than through writable.Dir, so the
// reply used to carry nothing but zeroes.
func TestKeyDirAttrs(t *testing.T) {
	nd, mnt := setupIpnsTest(t, nil)
	keyDir := mnt.Dir + "/" + nd.Identity.String()

	stat := func() (uint64, os.FileInfo) {
		t.Helper()
		info, err := os.Stat(keyDir)
		require.NoError(t, err)
		st, ok := info.Sys().(*syscall.Stat_t)
		require.True(t, ok)
		return st.Ino, info
	}

	ino, info := stat()
	require.NotZero(t, ino, "key directory should report an inode number")
	require.Less(t, ino, uint64(fusemnt.AutomaticIno),
		"inode number should come from the mount, not go-fuse's automatic range")
	require.True(t, info.IsDir())
	require.Equal(t, os.FileMode(0o755), info.Mode().Perm())
	require.EqualValues(t, fusemnt.Nlink, info.Sys().(*syscall.Stat_t).Nlink)

	// Outlast the mount's one second entry timeout so the kernel has to look
	// the entry up again.
	time.Sleep(1500 * time.Millisecond)

	again, _ := stat()
	require.Equal(t, ino, again, "inode number should survive a re-lookup")
}

// TestFilePersistence verifies that file data survives unmount and remount.
func TestFilePersistence(t *testing.T) {
	nd, mnt := setupIpnsTest(t, nil)

	data := fusetest.RandBytes(4000)
	require.NoError(t, os.WriteFile(mnt.Dir+"/local/persist", data, 0o644))
	mnt.Close()

	_, mnt = setupIpnsTest(t, nd)
	got, err := os.ReadFile(mnt.Dir + "/local/persist")
	require.NoError(t, err)
	require.True(t, bytes.Equal(data, got))
}

// TestMultipleDirs verifies nested directories persist across remount.
func TestMultipleDirs(t *testing.T) {
	nd, mnt := setupIpnsTest(t, nil)

	require.NoError(t, os.Mkdir(mnt.Dir+"/local/test1", 0o755))
	data1 := fusetest.WriteFileOrFail(t, 4000, mnt.Dir+"/local/test1/file1")
	require.NoError(t, os.Mkdir(mnt.Dir+"/local/test1/dir2", 0o755))
	data2 := fusetest.WriteFileOrFail(t, 5000, mnt.Dir+"/local/test1/dir2/file2")

	mnt.Close()
	_, mnt = setupIpnsTest(t, nd)

	fusetest.CheckExists(t, mnt.Dir+"/local/test1")
	fusetest.VerifyFile(t, mnt.Dir+"/local/test1/file1", data1)
	fusetest.VerifyFile(t, mnt.Dir+"/local/test1/dir2/file2", data2)
}

// TestStatfs verifies that statfs on the /ipns mount reports the disk
// space of the repo's backing filesystem. macOS Finder refuses to copy
// files onto a volume that reports zero free space.
func TestStatfs(t *testing.T) {
	_, mnt := setupIpnsTest(t, nil)

	fusetest.AssertStatfsNonZero(t, mnt.Dir)
}
