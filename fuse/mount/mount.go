// package mount provides a simple abstraction around a mount point
package mount

import (
	"fmt"
	"os/exec"
	"runtime"
	"time"

	fuse "github.com/jbenet/go-ipfs/Godeps/_workspace/src/bazil.org/fuse"
	fs "github.com/jbenet/go-ipfs/Godeps/_workspace/src/bazil.org/fuse/fs"
	ctxgroup "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-ctxgroup"

	u "github.com/jbenet/go-ipfs/util"
)

var log = u.Logger("mount")

var MountTimeout = time.Second * 5

// Mount represents a filesystem mount
type Mount interface {
	// MountPoint is the path at which this mount is mounted
	MountPoint() string

	// Unmounts the mount
	Unmount() error

	// CtxGroup returns the mount's CtxGroup to be able to link it
	// to other processes. Unmount upon closing.
	CtxGroup() ctxgroup.ContextGroup
}

// mount implements go-ipfs/fuse/mount
type mount struct {
	mpoint   string
	filesys  fs.FS
	fuseConn *fuse.Conn
	// closeErr error

	cg ctxgroup.ContextGroup
}

// Mount mounts a fuse fs.FS at a given location, and returns a Mount instance.
// parent is a ContextGroup to bind the mount's ContextGroup to.
func NewMount(p ctxgroup.ContextGroup, fsys fs.FS, mountpoint string) (Mount, error) {
	conn, err := fuse.Mount(mountpoint)
	if err != nil {
		return nil, err
	}

	m := &mount{
		mpoint:   mountpoint,
		fuseConn: conn,
		filesys:  fsys,
		cg:       ctxgroup.WithParent(p), // link it to parent.
	}
	m.cg.SetTeardown(m.unmount)

	// launch the mounting process.
	if err := m.mount(); err != nil {
		m.Unmount() // just in case.
		return nil, err
	}

	return m, nil
}

func (m *mount) mount() error {
	log.Infof("Mounting %s", m.MountPoint())

	errs := make(chan error, 1)
	go func() {
		err := fs.Serve(m.fuseConn, m.filesys)
		log.Debugf("Mounting %s -- fs.Serve returned (%s)", err)
		errs <- err
		close(errs)
	}()

	// wait for the mount process to be done, or timed out.
	select {
	case <-time.After(MountTimeout):
		return fmt.Errorf("Mounting %s timed out.", m.MountPoint())
	case err := <-errs:
		return err
	case <-m.fuseConn.Ready:
	}

	// check if the mount process has an error to report
	if err := m.fuseConn.MountError; err != nil {
		return err
	}

	log.Infof("Mounted %s", m.MountPoint())
	return nil
}

// umount is called exactly once to unmount this service.
// note that closing the connection will not always unmount
// properly. If that happens, we bring out the big guns
// (mount.ForceUnmountManyTimes, exec unmount).
func (m *mount) unmount() error {
	log.Infof("Unmounting %s", m.MountPoint())

	// try unmounting with fuse lib
	err := fuse.Unmount(m.MountPoint())
	if err == nil {
		return nil
	}
	log.Error("fuse unmount err: %s", err)

	// try closing the fuseConn
	err = m.fuseConn.Close()
	if err == nil {
		return nil
	}
	if err != nil {
		log.Error("fuse conn error: %s", err)
	}

	// try mount.ForceUnmountManyTimes
	if err := ForceUnmountManyTimes(m, 10); err != nil {
		return err
	}

	log.Infof("Seemingly unmounted %s", m.MountPoint())
	return nil
}

func (m *mount) CtxGroup() ctxgroup.ContextGroup {
	return m.cg
}

func (m *mount) MountPoint() string {
	return m.mpoint
}

func (m *mount) Unmount() error {
	// call ContextCloser Close(), which calls unmount() exactly once.
	return m.cg.Close()
}

// ForceUnmount attempts to forcibly unmount a given mount.
// It does so by calling diskutil or fusermount directly.
func ForceUnmount(m Mount) error {
	point := m.MountPoint()
	log.Infof("Force-Unmounting %s...", point)

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
		defer close(errc)

		// try vanilla unmount first.
		if err := exec.Command("umount", point).Run(); err == nil {
			return
		}

		// retry to unmount with the fallback cmd
		errc <- cmd.Run()
	}()

	select {
	case <-time.After(2 * time.Second):
		return fmt.Errorf("umount timeout")
	case err := <-errc:
		return err
	}
}

// ForceUnmountManyTimes attempts to forcibly unmount a given mount,
// many times. It does so by calling diskutil or fusermount directly.
// Attempts a given number of times.
func ForceUnmountManyTimes(m Mount, attempts int) error {
	var err error
	for i := 0; i < attempts; i++ {
		err = ForceUnmount(m)
		if err == nil {
			return err
		}

		<-time.After(time.Millisecond * 500)
	}
	return fmt.Errorf("Unmount %s failed after 10 seconds of trying.", m.MountPoint())
}
