//go:build (linux || darwin || freebsd || netbsd || openbsd) && !nofuse

package mfs

import (
	"github.com/ipfs/kubo/config"
	core "github.com/ipfs/kubo/core"
	mount "github.com/ipfs/kubo/fuse/mount"
)

// Mount mounts MFS at a given location, and returns a mount.Mount instance.
func Mount(ipfs *core.IpfsNode, mountpoint string) (mount.Mount, error) {
	cfg, err := ipfs.Repo.Config()
	if err != nil {
		return nil, err
	}
	fsys := NewFileSystem(ipfs, cfg.Mounts)
	return mount.NewMount(fsys, mountpoint, cfg.Mounts.FuseAllowOther.WithDefault(config.DefaultFuseAllowOther))
}
