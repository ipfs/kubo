//go:build (linux || darwin || freebsd) && !nofuse

package readonly

import (
	"os"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/ipfs/kubo/config"
	core "github.com/ipfs/kubo/core"
	fusemnt "github.com/ipfs/kubo/fuse/mount"
)

// Mount mounts IPFS at a given location, and returns a mount.Mount instance.
func Mount(ipfs *core.IpfsNode, mountpoint string) (fusemnt.Mount, error) {
	cfg, err := ipfs.Repo.Config()
	if err != nil {
		return nil, err
	}
	root := NewRoot(ipfs)
	opts := &fs.Options{
		NullPermissions: true,
		UID:             uint32(os.Getuid()),
		GID:             uint32(os.Getgid()),
		AttrTimeout:     &immutableAttrCacheTime,
		EntryTimeout:    &immutableAttrCacheTime,
		MountOptions: fuse.MountOptions{
			AllowOther:   cfg.Mounts.FuseAllowOther.WithDefault(config.DefaultFuseAllowOther),
			FsName:       "ipfs",
			MaxReadAhead: fusemnt.MaxReadAhead,
			Debug:        os.Getenv("IPFS_FUSE_DEBUG") != "",
		},
	}
	return fusemnt.NewMount(root, mountpoint, opts)
}
