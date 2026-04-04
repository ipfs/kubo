package mount

import "os"

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

// XattrCID is the extended attribute name for the node's CID.
// Follows the convention used by CephFS (ceph.*), Btrfs (btrfs.*),
// and GlusterFS (glusterfs.*) of using a project-specific namespace.
const XattrCID = "ipfs.cid"

// XattrCIDDeprecated is the old xattr name, kept for backward
// compatibility on /mfs where it was previously shipped.
// TODO: remove after 2 releases.
const XattrCIDDeprecated = "ipfs_cid"
