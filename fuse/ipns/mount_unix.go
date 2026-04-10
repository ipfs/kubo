// Mount/unmount helpers for the /ipns FUSE mount. go-fuse only builds on linux, darwin, and freebsd.
//go:build (linux || darwin || freebsd) && !nofuse

package ipns

import (
	"os"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/ipfs/kubo/config"
	core "github.com/ipfs/kubo/core"
	coreapi "github.com/ipfs/kubo/core/coreapi"
	iface "github.com/ipfs/kubo/core/coreiface"
	fusemnt "github.com/ipfs/kubo/fuse/mount"
)

// How long the kernel caches Lookup and Getattr results. 1 second
// matches the go-fuse default and what gocryptfs/rclone use.
// TODO: for resolved IPNS names, use the record's cache TTL (capped
// at Ipns.MaxCacheTTL) instead of a fixed 1 second.
// var (not const) because fs.Options needs a *time.Duration.
var mutableCacheTime = time.Second

// Mount mounts ipns at a given location, and returns a mount.Mount instance.
func Mount(ipfs *core.IpfsNode, ipnsmp, ipfsmp string) (fusemnt.Mount, error) {
	coreAPI, err := coreapi.NewCoreAPI(ipfs)
	if err != nil {
		return nil, err
	}

	cfg, err := ipfs.Repo.Config()
	if err != nil {
		return nil, err
	}

	mfsOpts, err := cfg.Import.MFSRootOptions()
	if err != nil {
		return nil, err
	}

	key, err := coreAPI.Key().Self(ipfs.Context())
	if err != nil {
		return nil, err
	}

	root, err := CreateRoot(ipfs.Context(), coreAPI, map[string]iface.Key{"local": key}, ipfsmp, ipnsmp, ipfs.Repo.Path(), cfg.Mounts, mfsOpts...)
	if err != nil {
		return nil, err
	}

	opts := &fs.Options{
		NullPermissions: true,
		UID:             uint32(os.Getuid()),
		GID:             uint32(os.Getgid()),
		EntryTimeout:    &mutableCacheTime,
		AttrTimeout:     &mutableCacheTime,
		MountOptions: fuse.MountOptions{
			AllowOther:        cfg.Mounts.FuseAllowOther.WithDefault(config.DefaultFuseAllowOther),
			FsName:            "ipns",
			MaxReadAhead:      fusemnt.MaxReadAhead,
			Debug:             os.Getenv("IPFS_FUSE_DEBUG") != "",
			ExtraCapabilities: fusemnt.WritableMountCapabilities,
		},
	}

	m, err := fusemnt.NewMount(root, ipnsmp, opts)
	if err != nil {
		_ = root.Close()
		return nil, err
	}

	return &ipnsMount{Mount: m, root: root}, nil
}

// ipnsMount wraps mount.Mount to call Root.Close() on unmount,
// which flushes and publishes all MFS roots.
type ipnsMount struct {
	fusemnt.Mount
	root *Root
}

func (m *ipnsMount) Unmount() error {
	err := m.Mount.Unmount()
	_ = m.root.Close()
	return err
}
