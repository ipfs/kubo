//go:build (linux || darwin || freebsd) && !nofuse

package mount

import (
	"errors"
	"sync"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

var ErrNotMounted = errors.New("not mounted")

// mount implements go-ipfs/fuse/mount.
type mount struct {
	mpoint string
	server *fuse.Server

	active     bool
	activeLock *sync.RWMutex

	unmountOnce sync.Once
}

// NewMount mounts a FUSE filesystem at a given location, and returns a Mount instance.
func NewMount(root fs.InodeEmbedder, mountpoint string, opts *fs.Options) (Mount, error) {
	server, err := fs.Mount(mountpoint, root, opts)
	if err != nil {
		return nil, err
	}

	m := &mount{
		mpoint:     mountpoint,
		server:     server,
		active:     true,
		activeLock: &sync.RWMutex{},
	}

	log.Infof("Mounted %s", mountpoint)
	return m, nil
}

// unmount is called exactly once to unmount this service.
func (m *mount) unmount() error {
	log.Infof("Unmounting %s", m.MountPoint())

	err := m.server.Unmount()
	if err == nil {
		m.setActive(false)
		return nil
	}
	log.Warnf("fuse unmount err: %s", err)

	// try mount.ForceUnmountManyTimes
	if err := ForceUnmountManyTimes(m, 10); err != nil {
		return err
	}

	log.Infof("Seemingly unmounted %s", m.MountPoint())
	m.setActive(false)
	return nil
}

func (m *mount) MountPoint() string {
	return m.mpoint
}

func (m *mount) Unmount() error {
	if !m.IsActive() {
		return ErrNotMounted
	}

	var err error
	m.unmountOnce.Do(func() {
		err = m.unmount()
	})

	return err
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
