//go:build (linux || darwin || freebsd) && !nofuse

package mfs

import (
	"os"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/ipfs/kubo/config"
	core "github.com/ipfs/kubo/core"
	fusemnt "github.com/ipfs/kubo/fuse/mount"
)

// How long the kernel caches Lookup and Getattr results. 1 second
// matches the go-fuse default and what gocryptfs/rclone use.
// var (not const) because fs.Options needs a *time.Duration.
var mutableCacheTime = time.Second

// Mount mounts MFS at a given location, and returns a mount.Mount instance.
func Mount(ipfs *core.IpfsNode, mountpoint string) (fusemnt.Mount, error) {
	cfg, err := ipfs.Repo.Config()
	if err != nil {
		return nil, err
	}
	root := NewFileSystem(ipfs, cfg.Mounts)
	opts := &fs.Options{
		NullPermissions: true,
		UID:             uint32(os.Getuid()),
		GID:             uint32(os.Getgid()),
		EntryTimeout:    &mutableCacheTime,
		AttrTimeout:     &mutableCacheTime,
		MountOptions: fuse.MountOptions{
			AllowOther:        cfg.Mounts.FuseAllowOther.WithDefault(config.DefaultFuseAllowOther),
			FsName:            "mfs",
			MaxReadAhead:      fusemnt.MaxReadAhead,
			Debug:             os.Getenv("IPFS_FUSE_DEBUG") != "",
			ExtraCapabilities: fusemnt.WritableMountCapabilities,
		},
	}
	return fusemnt.NewMount(root, mountpoint, opts)
}
