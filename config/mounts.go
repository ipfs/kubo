package config

const (
	DefaultFuseAllowOther = false
	DefaultStoreMtime     = false
	DefaultStoreMode      = false
)

// Mounts stores FUSE mount point configuration.
type Mounts struct {
	// IPFS is the mountpoint for the read-only /ipfs/ namespace.
	IPFS string

	// IPNS is the mountpoint for the /ipns/ namespace. Directories backed
	// by keys this node holds are writable; all other names resolve through
	// IPNS to read-only symlinks into the /ipfs mount.
	IPNS string

	// MFS is the mountpoint for the Mutable File System (ipfs files API).
	MFS string

	// FuseAllowOther sets the FUSE allow_other mount option, letting
	// users other than the mounter access the mounted filesystem.
	FuseAllowOther Flag

	// StoreMtime controls whether writable mounts (/ipns and /mfs) persist
	// the current time as mtime in UnixFS metadata when creating a file or
	// opening it for writing. This changes the resulting CID even when file
	// content is identical.
	//
	// Reading mtime from UnixFS is always enabled on all mounts.
	StoreMtime Flag

	// StoreMode controls whether writable mounts (/ipns and /mfs) persist
	// POSIX permission bits in UnixFS metadata when a chmod request is made.
	//
	// Reading mode from UnixFS is always enabled on all mounts.
	StoreMode Flag
}
