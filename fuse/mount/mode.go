package mount

import (
	"os"

	"github.com/hanwen/go-fuse/v2/fuse"
)

// Default POSIX modes used by FUSE mounts when the UnixFS DAG node does
// not contain explicit permission metadata. Most data on IPFS does not
// include mode, so these apply to the majority of files and directories.

// Writable mounts (/ipns, /mfs): standard POSIX defaults matching umask 022.
const (
	DefaultFileModeRW = os.FileMode(0o644)
	DefaultDirModeRW  = os.ModeDir | 0o755
)

// Read-only mount (/ipfs): no write bits.
const (
	DefaultFileModeRO = os.FileMode(0o444)
	DefaultDirModeRO  = os.ModeDir | 0o555
)

// NamespaceRootMode is for the /ipfs/ and /ipns/ root directories.
// Execute-only: these are virtual namespaces where users traverse by
// name (CID or IPNS key) but listing the full namespace is not possible.
const NamespaceRootMode = os.ModeDir | 0o111

// MaxReadAhead tells the kernel how far ahead to read in a single FUSE
// request. 64 MiB works well for sequential access (streaming, file
// copies) because most data is served from the local blockstore after
// the initial fetch. Network-backed reads are already chunked by the
// DAG layer, so oversized readahead does not cause extra round-trips.
const MaxReadAhead = 64 * 1024 * 1024

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

// XattrCID is the extended attribute name for the node's CID.
// Follows the convention used by CephFS (ceph.*), Btrfs (btrfs.*),
// and GlusterFS (glusterfs.*) of using a project-specific namespace.
const XattrCID = "ipfs.cid"

// XattrCIDDeprecated is the old xattr name, kept for backward
// compatibility on /mfs where it was previously shipped.
// TODO: remove after 2 releases.
const XattrCIDDeprecated = "ipfs_cid"
