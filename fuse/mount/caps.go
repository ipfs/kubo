// FUSE mount capabilities. go-fuse only builds on linux, darwin, and freebsd.
//go:build (linux || darwin || freebsd) && !nofuse

package mount

import "github.com/hanwen/go-fuse/v2/fuse"

// WritableMountCapabilities are FUSE capabilities requested for writable
// mounts (/ipns, /mfs).
//
// CAP_ATOMIC_O_TRUNC tells the kernel to pass O_TRUNC to Open instead of
// sending a separate SETATTR(size=0) before Open. Without this, the kernel
// does SETATTR first, which requires opening a write descriptor inside
// Setattr. MFS only allows one write descriptor at a time, so that
// deadlocks. With this capability, O_TRUNC is handled inside Open where
// we already hold the descriptor.
const WritableMountCapabilities = fuse.CAP_ATOMIC_O_TRUNC
