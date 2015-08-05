// +build linux darwin freebsd
// +build !nofuse

package ipns

import (
	core "github.com/ipfs/go-ipfs/core"
	mount "github.com/ipfs/go-ipfs/fuse/mount"
	mfs "github.com/ipfs/go-ipfs/mfs"
)

// Mount mounts ipns at a given location, and returns a mount.Mount instance.
func Mount(ipfs *core.IpfsNode, ipnsmp, ipfsmp string) (mount.Mount, error) {
	cfg := ipfs.Repo.Config()
	allow_other := cfg.Mounts.FuseAllowOther

	if ipfs.Mfs == nil {
		fs, err := mfs.NewFilesystem(ipfs.Context(), ipfs.DAG, ipfs.Pinning)
		if err != nil {
			return nil, err
		}
		ipfs.Mfs = fs
	}

	fsys, err := NewFileSystem(ipfs, ipfs.PrivateKey, ipfsmp, ipnsmp)
	if err != nil {
		return nil, err
	}

	return mount.NewMount(ipfs.Process(), fsys, ipnsmp, allow_other)
}
