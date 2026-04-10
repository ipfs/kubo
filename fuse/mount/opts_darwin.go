//go:build darwin && !nofuse

package mount

import "github.com/hanwen/go-fuse/v2/fuse"

// PlatformMountOpts applies macOS-specific FUSE mount options.
func PlatformMountOpts(opts *fuse.MountOptions) {
	// volname: Finder shows this instead of the generic "macfuse Volume 0".
	if opts.FsName != "" {
		opts.Options = append(opts.Options, "volname="+opts.FsName)
	}

	// noapplexattr: prevents Finder from probing com.apple.FinderInfo,
	// com.apple.ResourceFork, and other Apple-private xattrs on every
	// file access. Without this, each stat triggers multiple Getxattr
	// calls that all return ENOATTR, adding latency on network-backed
	// mounts.
	opts.Options = append(opts.Options, "noapplexattr")

	// noappledouble: prevents macOS from creating ._ resource fork
	// sidecar files when copying or editing files on the mount. These
	// AppleDouble files pollute the DAG with metadata that only macOS
	// understands and inflate the CID tree.
	opts.Options = append(opts.Options, "noappledouble")
}
