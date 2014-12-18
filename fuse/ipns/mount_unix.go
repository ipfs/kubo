package ipns

import (
	"fmt"
	"os/exec"
	"runtime"
	"time"

	fuse "github.com/jbenet/go-ipfs/Godeps/_workspace/src/bazil.org/fuse"
	fs "github.com/jbenet/go-ipfs/Godeps/_workspace/src/bazil.org/fuse/fs"

	core "github.com/jbenet/go-ipfs/core"
	mount "github.com/jbenet/go-ipfs/fuse/mount"
)

// Mount mounts an IpfsNode instance at a particular path. It
// serves until the process receives exit signals (to Unmount).
func Mount(ipfs *core.IpfsNode, fpath string, ipfspath string) (mount.Mount, error) {
	log.Infof("Mounting ipns at %s...", fpath)

	// setup the Mount abstraction.
	m := mount.New(ipfs.Context(), fpath)

	// go serve the mount
	m.Mount(func(m mount.Mount) error {
		return internalMount(ipfs, fpath, ipfspath)
	}, internalUnmount)

	select {
	case <-m.Closed():
		return nil, fmt.Errorf("failed to mount")
	case <-time.After(time.Second):
		// assume it worked...
	}

	// bind the mount (ContextGroup) to the node, so that when the node exits
	// the fsclosers are automatically closed.
	ipfs.AddChildGroup(m)
	return m, nil
}

// mount attempts to mount at the provided FUSE mount point
func internalMount(ipfs *core.IpfsNode, fpath string, ipfspath string) error {

	c, err := fuse.Mount(fpath)
	if err != nil {
		return err
	}
	defer c.Close()

	fsys, err := NewIpns(ipfs, ipfspath)
	if err != nil {
		return err
	}

	log.Infof("Mounted ipns at %s.", fpath)
	if err := fs.Serve(c, fsys); err != nil {
		return err
	}

	// check if the mount process has an error to report
	<-c.Ready
	if err := c.MountError; err != nil {
		return err
	}
	return nil
}

// unmount attempts to unmount the provided FUSE mount point, forcibly
// if necessary.
func internalUnmount(m mount.Mount) error {
	point := m.MountPoint()
	log.Infof("Unmounting ipns at %s...", point)

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("diskutil", "umount", "force", point)
	case "linux":
		cmd = exec.Command("fusermount", "-u", point)
	default:
		return fmt.Errorf("unmount: unimplemented")
	}

	errc := make(chan error, 1)
	go func() {
		if err := exec.Command("umount", point).Run(); err == nil {
			errc <- err
		}
		// retry to unmount with the fallback cmd
		errc <- cmd.Run()
	}()

	select {
	case <-time.After(1 * time.Second):
		return fmt.Errorf("umount timeout")
	case err := <-errc:
		return err
	}
}
