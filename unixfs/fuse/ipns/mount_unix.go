// +build linux darwin freebsd

package ipns

import (
	core "github.com/jbenet/go-ipfs/core"
	mount "github.com/jbenet/go-ipfs/unixfs/fuse/mount"
)

// Mount mounts ipns at a given location, and returns a mount.Mount instance.
func Mount(ipfs *core.IpfsNode, ipnsmp, ipfsmp string) (mount.Mount, error) {
	fsys, err := NewFileSystem(ipfs, ipfs.PrivateKey, ipfsmp)
	if err != nil {
		return nil, err
	}

	return mount.NewMount(ipfs, fsys, ipnsmp)
}
