package mount

import "os"

// Default POSIX modes used by FUSE mounts when the UnixFS DAG node does
// not contain explicit permission metadata. Most data on IPFS does not
// include mode, so these apply to the majority of files and directories.
//
// Per the UnixFS spec, implementations may default to 0755 for directories
// and 0644 for files when mode is absent.
// See https://specs.ipfs.tech/unixfs/#dag-pb-optional-metadata

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

// SymlinkMode is the POSIX permission bits for symlinks. Symlink
// permissions are always 0777; access control uses the target's mode.
const SymlinkMode = os.FileMode(0o777)

// MaxReadAhead tells the kernel how far ahead to read in a single FUSE
// request. 64 MiB works well for sequential access (streaming, file
// copies) because most data is served from the local blockstore after
// the initial fetch. Network-backed reads are already chunked by the
// DAG layer, so oversized readahead does not cause extra round-trips.
const MaxReadAhead = 64 * 1024 * 1024

// XattrCID is the extended attribute name for the node's CID.
// Follows the convention used by CephFS (ceph.*), Btrfs (btrfs.*),
// and GlusterFS (glusterfs.*) of using a project-specific namespace.
const XattrCID = "ipfs.cid"

// XattrCIDDeprecated is the old xattr name, kept for backward
// compatibility on /mfs where it was previously shipped.
// TODO: remove after 2 releases.
const XattrCIDDeprecated = "ipfs_cid"
