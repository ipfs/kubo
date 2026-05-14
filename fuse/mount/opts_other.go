//go:build (linux || freebsd) && !nofuse

package mount

import "github.com/hanwen/go-fuse/v2/fuse"

// PlatformMountOpts is a no-op on Linux and FreeBSD.
func PlatformMountOpts(_ *fuse.MountOptions) {}
