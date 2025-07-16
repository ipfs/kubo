//go:build (linux || darwin || freebsd || netbsd || openbsd) && !nofuse
// +build linux darwin freebsd netbsd openbsd
// +build !nofuse

package ipns

import (
	"context"

	core "github.com/ipfs/kubo/core"
	coreapi "github.com/ipfs/kubo/core/coreapi"
	mount "github.com/ipfs/kubo/fuse/mount"
)

// Mount mounts ipns at a given location, and returns a mount.Mount instance.
func Mount(ipfs *core.IpfsNode, ipnsmp, ipfsmp string) (mount.Mount, error) {
	coreAPI, err := coreapi.NewCoreAPI(ipfs)
	if err != nil {
		return nil, err
	}

	cfg, err := ipfs.Repo.Config()
	if err != nil {
		return nil, err
	}

	allowOther := cfg.Mounts.FuseAllowOther

	fsys, err := NewFileSystem(ipfs.Context(), coreAPI, ipfsmp, ipnsmp)
	if err != nil {
		return nil, err
	}

	mnt, err := mount.NewMount(fsys, ipnsmp, allowOther)
	if err != nil {
		return nil, err
	}
	context.AfterFunc(ipfs.Context(), func() {
		err := mnt.Unmount()
		if err != nil {
			log.Errorw("failed to unmount", "err", err)
		}
	})
	return mnt, nil
}
