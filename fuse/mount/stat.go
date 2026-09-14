// FUSE stat helpers. go-fuse only builds on linux, darwin, and freebsd.
//go:build (linux || darwin || freebsd) && !nofuse

package mount

import (
	"strconv"
	"strings"

	"github.com/hanwen/go-fuse/v2/fuse"
)

// StatBlockSize is the POSIX stat(2) block unit. The st_blocks field
// reports allocation in 512-byte units regardless of the filesystem's
// real block size (see `man 2 stat`). Tools like `du`, `ls -s`, and
// `find -size` multiply st_blocks by this constant to compute bytes.
const StatBlockSize = 512

// DefaultBlksize is the preferred I/O size (stat.st_blksize) FUSE mounts
// advertise when no chunker-derived value applies (readonly /ipfs, or
// writable /mfs with a rabin/buzhash chunker). Larger hints let tools
// like cp, dd, and rsync use bigger buffers, amortizing FUSE syscall and
// DAG-walk overhead. 1 MiB matches the chunk size of Kubo's
// cross-implementation CID-deterministic import profile (IPIP-499).
// Hardcoded instead of tracking boxo's chunker default so the stat(2)
// contract stays stable across Kubo and boxo upgrades.
const DefaultBlksize = 1024 * 1024

// Nlink is the link count (stat.st_nlink) reported for every entry on a
// FUSE mount. Left unset it is 0, which POSIX gives to an inode with no
// remaining names, so tools read a live file as one that is on its way out.
//
// 1 is right for files and symlinks: neither IPFS nor MFS has hard links.
// Directories conventionally count their own entry, their parent's, and one
// per subdirectory, but reporting 1 for them as well is deliberate. GNU find
// skips the stat of a directory's children once it has seen st_nlink-2 of
// them, and would miss entries if the count were a guess; a count below 2
// tells it to walk the directory properly instead.
const Nlink = 1

// SizeToStatBlocks converts a byte size to the number of 512-byte blocks
// reported by POSIX stat(2) in the st_blocks field, rounded up so a
// non-empty file reports at least one block.
func SizeToStatBlocks(size uint64) uint64 {
	return (size + StatBlockSize - 1) / StatBlockSize
}

// BlksizeFromChunker derives the preferred I/O size hint for the writable
// mounts from the user's Import.UnixFSChunker setting. It extracts the
// byte count from `size-<bytes>` and returns DefaultBlksize for rabin,
// buzhash, or malformed values (where there is no single preferred size).
// Values are clamped to fuse.MAX_KERNEL_WRITE because the kernel splits
// any larger userspace read/write into MAX_KERNEL_WRITE-sized FUSE ops
// regardless, so hinting past the ceiling just wastes userspace buffers.
func BlksizeFromChunker(chunkerStr string) uint32 {
	if sizeStr, ok := strings.CutPrefix(chunkerStr, "size-"); ok {
		if size, err := strconv.ParseUint(sizeStr, 10, 64); err == nil && size > 0 {
			if size > fuse.MAX_KERNEL_WRITE {
				return fuse.MAX_KERNEL_WRITE
			}
			return uint32(size)
		}
	}
	return DefaultBlksize
}
