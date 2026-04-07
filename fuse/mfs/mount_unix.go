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

// mutableCacheTime is how long the kernel caches Lookup and Getattr
// results for writable mounts. 1 second matches the standard default
// used by go-fuse's loopback, gocryptfs, and rclone. This avoids
// re-traversing every path component on each syscall while keeping
// the staleness window short enough for interactive use.
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
			AllowOther:   cfg.Mounts.FuseAllowOther.WithDefault(config.DefaultFuseAllowOther),
			FsName:       "mfs",
			MaxReadAhead: fusemnt.MaxReadAhead,
			Debug:        os.Getenv("IPFS_FUSE_DEBUG") != "",
		},
	}
	return fusemnt.NewMount(root, mountpoint, opts)
}
