// +build !nofuse

package mount

import (
	"fmt"
	"time"

	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/bazil.org/fuse"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/bazil.org/fuse/fs"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/goprocess"
)

// mount implements go-ipfs/fuse/mount
type mount struct {
	mpoint   string
	filesys  fs.FS
	fuseConn *fuse.Conn
	// closeErr error

	proc goprocess.Process
}

// Mount mounts a fuse fs.FS at a given location, and returns a Mount instance.
// parent is a ContextGroup to bind the mount's ContextGroup to.
func NewMount(p goprocess.Process, fsys fs.FS, mountpoint string, allow_other bool) (Mount, error) {
	var conn *fuse.Conn
	var err error

	if allow_other {
		conn, err = fuse.Mount(mountpoint, fuse.AllowOther())
	} else {
		conn, err = fuse.Mount(mountpoint)
	}

	if err != nil {
		return nil, err
	}

	m := &mount{
		mpoint:   mountpoint,
		fuseConn: conn,
		filesys:  fsys,
		proc:     goprocess.WithParent(p), // link it to parent.
	}
	m.proc.SetTeardown(m.unmount)

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
		if err != nil {
			errs <- err
		}
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
	log.Warningf("fuse unmount err: %s", err)

	// try closing the fuseConn
	err = m.fuseConn.Close()
	if err == nil {
		return nil
	}
	if err != nil {
		log.Warningf("fuse conn error: %s", err)
	}

	// try mount.ForceUnmountManyTimes
	if err := ForceUnmountManyTimes(m, 10); err != nil {
		return err
	}

	log.Infof("Seemingly unmounted %s", m.MountPoint())
	return nil
}

func (m *mount) Process() goprocess.Process {
	return m.proc
}

func (m *mount) MountPoint() string {
	return m.mpoint
}

func (m *mount) Unmount() error {
	// call Process Close(), which calls unmount() exactly once.
	return m.proc.Close()
}
