// package mount provides a simple abstraction around a mount point
package mount

import (
	"fmt"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	u "github.com/jbenet/go-ipfs/util"
	ctxc "github.com/jbenet/go-ipfs/util/ctxcloser"
)

var log = u.Logger("mount")

// Mount represents a filesystem mount
type Mount interface {

	// MountPoint is the path at which this mount is mounted
	MountPoint() string

	// Unmount calls Close.
	Unmount() error

	ctxc.ContextCloser
}

// UnmountFunc is a function used to unmount a mount
type UnmountFunc func(mountpoint string) error

// New constructs a new Mount instance. ctx is a context to wait upon,
// the mountpoint is the directory that the mount was mounted at, and unmount
// in an UnmountFunc to perform the unmounting logic.
func New(ctx context.Context, mountpoint string, unmount UnmountFunc) Mount {
	m := &mount{
		mpoint:  mountpoint,
		unmount: unmount,
	}
	m.ContextCloser = ctxc.NewContextCloser(ctx, m.persistentUnmount)
	return m
}

type mount struct {
	ctxc.ContextCloser

	unmount UnmountFunc
	mpoint  string
}

// umount is called after the mount is closed.
// TODO this is hacky, make it better.
func (m *mount) persistentUnmount() error {

	// ok try to unmount a whole bunch of times...
	for i := 0; i < 34; i++ {
		err := m.unmount(m.mpoint)
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

func ServeMount(m Mount, mount func(Mount) error) {
	m.Children().Add(1)

	// go serve the mount
	go func() {
		if err := mount(m); err != nil {
			log.Error("%s mount: %s", m.MountPoint(), err)
		}
		m.Children().Done()
		m.Unmount()
	}()
}
