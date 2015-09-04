// +build linux darwin freebsd
// +build !nofuse

package ipns

import (
	core "github.com/ipfs/go-ipfs/core"
	mount "github.com/ipfs/go-ipfs/fuse/mount"
	ipnsfs "github.com/ipfs/go-ipfs/ipnsfs"
)

// Mount mounts ipns at a given location, and returns a mount.Mount instance.
func Mount(ipfs *core.IpfsNode, ipnsmp, ipfsmp string) (mount.Mount, error) {
	cfg, err := ipfs.Repo.Config()
	if err != nil {
		return nil, err
	}

	allow_other := cfg.Mounts.FuseAllowOther

	if ipfs.IpnsFs == nil {
		fs, err := ipnsfs.NewFilesystem(ipfs.Context(), ipfs.DAG, ipfs.Namesys, ipfs.Pinning, ipfs.PrivateKey)
		if err != nil {
			return nil, err
		}
		ipfs.IpnsFs = fs
	}

	fsys, err := NewFileSystem(ipfs, ipfs.PrivateKey, ipfsmp, ipnsmp)
	if err != nil {
		return nil, err
	}

	return mount.NewMount(ipfs.Process(), fsys, ipnsmp, allow_other)
}
