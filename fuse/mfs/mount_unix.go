//go:build (linux || darwin || freebsd || netbsd || openbsd) && !nofuse
// +build linux darwin freebsd netbsd openbsd
// +build !nofuse

package mfs

import (
	"context"
	"errors"

	core "github.com/ipfs/kubo/core"
	mount "github.com/ipfs/kubo/fuse/mount"
)

// Mount mounts MFS at a given location, and returns a mount.Mount instance.
func Mount(ipfs *core.IpfsNode, mountpoint string) (mount.Mount, error) {
	cfg, err := ipfs.Repo.Config()
	if err != nil {
		return nil, err
	}
	allowOther := cfg.Mounts.FuseAllowOther
	fsys := NewFileSystem(ipfs)
	mnt, err := mount.NewMount(fsys, mountpoint, allowOther)
	if err != nil {
		return nil, err
	}
	ipfs.WG().Add(1)
	context.AfterFunc(ipfs.Context(), func() {
		err := mnt.Unmount()
		if err != nil && !errors.Is(err, mount.ErrNotMounted) {
			log.Errorw("failed to unmount", "err", err)
		}
		ipfs.WG().Done()
	})
	return mnt, nil
}
