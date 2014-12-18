// package mount provides a simple abstraction around a mount point
package mount

import (
	"fmt"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ctxgroup "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-ctxgroup"

	u "github.com/jbenet/go-ipfs/util"
)

var log = u.Logger("mount")

// Mount represents a filesystem mount
type Mount interface {

	// MountPoint is the path at which this mount is mounted
	MountPoint() string

	// Mount function sets up a mount + registers the unmount func
	Mount(mount MountFunc, unmount UnmountFunc)

	// Unmount calls Close.
	Unmount() error

	ctxgroup.ContextGroup
}

// UnmountFunc is a function used to Unmount a mount
type UnmountFunc func(Mount) error

// MountFunc is a function used to Mount a mount
type MountFunc func(Mount) error

// New constructs a new Mount instance. ctx is a context to wait upon,
// the mountpoint is the directory that the mount was mounted at, and unmount
// in an UnmountFunc to perform the unmounting logic.
func New(ctx context.Context, mountpoint string) Mount {
	m := &mount{mpoint: mountpoint}
	m.ContextGroup = ctxgroup.WithContextAndTeardown(ctx, m.persistentUnmount)
	return m
}

type mount struct {
	ctxgroup.ContextGroup

	unmount UnmountFunc
	mpoint  string
}

// umount is called after the mount is closed.
// TODO this is hacky, make it better.
func (m *mount) persistentUnmount() error {
	// no unmount func.
	if m.unmount == nil {
		return nil
	}

	// ok try to unmount a whole bunch of times...
	for i := 0; i < 34; i++ {
		err := m.unmount(m)
		if err == nil {
			return nil
		}
		time.Sleep(time.Millisecond * 300)
	}

	// didnt work.
	return fmt.Errorf("Unmount %s failed after 10 seconds of trying.")
}

func (m *mount) MountPoint() string {
	return m.mpoint
}

func (m *mount) Unmount() error {
	return m.Close()
}

func (m *mount) Mount(mount MountFunc, unmount UnmountFunc) {
	m.unmount = unmount

	// go serve the mount
	m.ContextGroup.AddChildFunc(func(parent ctxgroup.ContextGroup) {
		if err := mount(m); err != nil {
			log.Error("%s mount: %s", m.MountPoint(), err)
		}
		m.Unmount()
	})
}
