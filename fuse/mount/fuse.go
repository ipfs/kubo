// +build !nofuse,!windows,!openbsd,!netbsd,!plan9

package mount

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"github.com/jbenet/goprocess"
)

var ErrNotMounted = errors.New("not mounted")

// mount implements go-ipfs/fuse/mount
type mount struct {
	mpoint   string
	filesys  fs.FS
	fuseConn *fuse.Conn

	active     bool
	activeLock *sync.RWMutex

	proc goprocess.Process
}

// Mount mounts a fuse fs.FS at a given location, and returns a Mount instance.
// parent is a ContextGroup to bind the mount's ContextGroup to.
func NewMount(p goprocess.Process, fsys fs.FS, mountpoint string, allow_other bool) (Mount, error) {
	var conn *fuse.Conn
	var err error

	var mountOpts = []fuse.MountOption{
		fuse.MaxReadahead(64 * 1024 * 1024),
		fuse.AsyncRead(),
	}

	if allow_other {
		mountOpts = append(mountOpts, fuse.AllowOther())
	}
	conn, err = fuse.Mount(mountpoint, mountOpts...)

	if err != nil {
		return nil, err
	}

	m := &mount{
		mpoint:     mountpoint,
		fuseConn:   conn,
		filesys:    fsys,
		active:     false,
		activeLock: &sync.RWMutex{},
		proc:       goprocess.WithParent(p), // link it to parent.
	}
	m.proc.SetTeardown(m.unmount)

	// launch the mounting process.
	if err := m.mount(); err != nil {
		_ = m.Unmount() // just in case.
		return nil, err
	}

	return m, nil
}

func (m *mount) mount() error {
	log.Infof("Mounting %s", m.MountPoint())

	errs := make(chan error, 1)
	go func() {
		// fs.Serve blocks until the filesystem is unmounted.
		err := fs.Serve(m.fuseConn, m.filesys)
		log.Debugf("%s is unmounted", m.MountPoint())
		if err != nil {
			log.Debugf("fs.Serve returned (%s)", err)
			errs <- err
		}
		m.setActive(false)
	}()

	// wait for the mount process to be done, or timed out.
	select {
	case <-time.After(MountTimeout):
		return fmt.Errorf("mounting %s timed out", m.MountPoint())
	case err := <-errs:
		return err
	case <-m.fuseConn.Ready:
	}

	// check if the mount process has an error to report
	if err := m.fuseConn.MountError; err != nil {
		return err
	}

	m.setActive(true)

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
		m.setActive(false)
		return nil
	}
	log.Warnf("fuse unmount err: %s", err)

	// try closing the fuseConn
	err = m.fuseConn.Close()
	if err == nil {
		m.setActive(false)
		return nil
	}
	log.Warnf("fuse conn error: %s", err)

	// try mount.ForceUnmountManyTimes
	if err := ForceUnmountManyTimes(m, 10); err != nil {
		return err
	}

	log.Infof("Seemingly unmounted %s", m.MountPoint())
	m.setActive(false)
	return nil
}

func (m *mount) Process() goprocess.Process {
	return m.proc
}

func (m *mount) MountPoint() string {
	return m.mpoint
}

func (m *mount) Unmount() error {
	if !m.IsActive() {
		return ErrNotMounted
	}

	// call Process Close(), which calls unmount() exactly once.
	return m.proc.Close()
}

func (m *mount) IsActive() bool {
	m.activeLock.RLock()
	defer m.activeLock.RUnlock()

	return m.active
}

func (m *mount) setActive(a bool) {
	m.activeLock.Lock()
	m.active = a
	m.activeLock.Unlock()
}
