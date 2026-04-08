//go:build !openbsd && !nofuse && !netbsd && !plan9

package node

import (
	"context"
	"os"
	"testing"
	"time"

	core "github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/fuse/fusetest"
	ipns "github.com/ipfs/kubo/fuse/ipns"
	mount "github.com/ipfs/kubo/fuse/mount"
)

func mkdir(t *testing.T, path string) {
	err := os.Mkdir(path, os.ModeDir|os.ModePerm)
	if err != nil {
		t.Fatal(err)
	}
}

// Test externally unmounting, then trying to unmount in code.
func TestExternalUnmount(t *testing.T) {

	// TODO: needed?
	fusetest.SkipUnlessFUSE(t)

	node, err := core.NewNode(context.Background(), &core.BuildCfg{})
	if err != nil {
		t.Fatal(err)
	}

	err = ipns.InitializeKeyspace(node, node.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}

	// get the test dir paths (/tmp/TestExternalUnmount)
	dir := t.TempDir()

	ipfsDir := dir + "/ipfs"
	ipnsDir := dir + "/ipns"
	mfsDir := dir + "/mfs"
	mkdir(t, ipfsDir)
	mkdir(t, ipnsDir)
	mkdir(t, mfsDir)

	err = Mount(node, ipfsDir, ipnsDir, mfsDir)
	fusetest.MountError(t, err)

	t.Cleanup(func() {
		if node.Mounts.Mfs != nil && node.Mounts.Mfs.IsActive() {
			if err := node.Mounts.Mfs.Unmount(); err != nil {
				t.Fatal(err)
			}
		}
		if node.Mounts.Ipns != nil && node.Mounts.Ipns.IsActive() {
			if err := node.Mounts.Ipns.Unmount(); err != nil {
				t.Fatal(err)
			}
		}
	})

	// Run shell command to externally unmount the directory
	cmd, err := mount.UnmountCmd(ipfsDir)
	if err != nil {
		t.Fatal(err)
	}

	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	// TODO(noffle): it takes a moment for the goroutine that's running fs.Serve to be notified and do its cleanup.
	time.Sleep(time.Millisecond * 100)

	// Attempt to unmount IPFS; it should unmount successfully.
	err = node.Mounts.Ipfs.Unmount()
	if err != mount.ErrNotMounted {
		t.Fatal("Unmount should have failed")
	}
}
