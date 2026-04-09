// go-fuse only builds on linux, darwin, and freebsd.
//go:build (linux || darwin || freebsd) && !nofuse

package node

import (
	"os"
	"testing"
	"time"

	core "github.com/ipfs/kubo/core"
	coremock "github.com/ipfs/kubo/core/mock"
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

// TestExternalUnmount runs an external unmount on each of the three
// FUSE mounts (/ipfs, /ipns, /mfs) and confirms the corresponding
// Mount.IsActive flips to false and Unmount returns ErrNotMounted.
// This exercises the goroutine in fuse/mount/fuse.go that watches
// fuse.Server.Wait() to detect out-of-band unmounts.
func TestExternalUnmount(t *testing.T) {
	fusetest.SkipUnlessFUSE(t)

	cases := []struct {
		name   string
		target func(node *core.IpfsNode, paths mountPaths) (string, mount.Mount)
	}{
		{
			name: "ipfs",
			target: func(node *core.IpfsNode, p mountPaths) (string, mount.Mount) {
				return p.ipfs, node.Mounts.Ipfs
			},
		},
		{
			name: "ipns",
			target: func(node *core.IpfsNode, p mountPaths) (string, mount.Mount) {
				return p.ipns, node.Mounts.Ipns
			},
		},
		{
			name: "mfs",
			target: func(node *core.IpfsNode, p mountPaths) (string, mount.Mount) {
				return p.mfs, node.Mounts.Mfs
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node, paths := setupAllMounts(t)
			mountpoint, target := tc.target(node, paths)

			// Run shell command to externally unmount the directory.
			cmd, err := mount.UnmountCmd(mountpoint)
			if err != nil {
				t.Fatal(err)
			}
			if err := cmd.Run(); err != nil {
				t.Fatal(err)
			}

			// The goroutine watching fuse.Server.Wait() needs a moment
			// to observe the kernel-side unmount and flip IsActive.
			time.Sleep(100 * time.Millisecond)

			if target.IsActive() {
				t.Fatal("mount should be inactive after external unmount")
			}
			if err := target.Unmount(); err != mount.ErrNotMounted {
				t.Fatalf("expected ErrNotMounted, got %v", err)
			}
		})
	}
}

type mountPaths struct {
	ipfs, ipns, mfs string
}

// setupAllMounts builds an IpfsNode and mounts all three FUSE filesystems
// under a fresh temp directory. Cleanup unmounts whatever is still active.
//
// The node is built via coremock.NewMockNode so it is online: doMount
// only mounts /ipns when node.IsOnline is true, and the test needs all
// three mounts populated.
func setupAllMounts(t *testing.T) (*core.IpfsNode, mountPaths) {
	t.Helper()

	node, err := coremock.NewMockNode()
	if err != nil {
		t.Fatal(err)
	}
	if err := ipns.InitializeKeyspace(node, node.PrivateKey); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	paths := mountPaths{
		ipfs: dir + "/ipfs",
		ipns: dir + "/ipns",
		mfs:  dir + "/mfs",
	}
	mkdir(t, paths.ipfs)
	mkdir(t, paths.ipns)
	mkdir(t, paths.mfs)

	err = Mount(node, paths.ipfs, paths.ipns, paths.mfs)
	fusetest.MountError(t, err)

	t.Cleanup(func() {
		Unmount(node)
	})
	return node, paths
}
