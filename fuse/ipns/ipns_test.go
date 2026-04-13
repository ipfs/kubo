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
	"testing"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/stretchr/testify/require"

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

	root, err := CreateRoot(nd.Context(), coreAPI, map[string]iface.Key{"local": key}, "", "", nd.Repo.Path(), cfg, config.Import{})
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

// TestNamespaceRootMode verifies that the /ipns root has execute-only
// mode (not listable, only traversable).
func TestNamespaceRootMode(t *testing.T) {
	_, mnt := setupIpnsTest(t, nil)

	info, err := os.Stat(mnt.Dir)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o111), info.Mode().Perm())
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

	// The in-memory test repo returns "" for Path(), so point RepoPath
	// at a real directory to exercise the syscall path.
	repoDir := t.TempDir()
	mnt.Root.RepoPath = repoDir

	fusetest.AssertStatfsNonZero(t, mnt.Dir)
}
