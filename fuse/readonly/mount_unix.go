// +build linux darwin freebsd

package readonly

import (
	core "github.com/jbenet/go-ipfs/core"
	mount "github.com/jbenet/go-ipfs/fuse/mount"
)

// Mount mounts ipfs at a given location, and returns a mount.Mount instance.
func Mount(ipfs *core.IpfsNode, mountpoint string) (mount.Mount, error) {
	fsys := NewFileSystem(ipfs)
	return mount.NewMount(ipfs, fsys, mountpoint)
}
