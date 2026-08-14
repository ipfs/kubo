// Inode numbering rules shared by the FUSE mounts.
//
//go:build (linux || darwin || freebsd) && !nofuse

package mount

import (
	"encoding/binary"
	"hash/fnv"

	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

// RootIno is the inode number reported for a mount point. go-fuse leaves the
// root's inode number at 0 unless it is told otherwise, and 0 is not a valid
// inode number: tools that check it read the mount point as a file that is
// not there. 1 is the conventional root number and is already the FUSE node
// ID the kernel uses for the root.
const RootIno = 1

// FirstIno is the lowest number a mount hands out to an entry, leaving 0
// (not a valid inode number) and 1 (the mount point) alone.
const FirstIno = 2

// AutomaticIno is where go-fuse starts numbering nodes a filesystem did not
// number itself (fs.Options.FirstAutomaticIno). Mounts keep their own numbers
// below it, so a node that slips through unnumbered can never be mistaken for
// one that was numbered. Staying in the lower half also keeps the numbers
// readable by 32-bit callers of stat and getdents for any realistic tree.
const AutomaticIno = 1 << 63

// InoFromCid derives an inode number for a node on an immutable mount, where
// the CID is the identity of the content and nothing else has to be tracked.
//
// The first eight bytes of the multihash digest are already uniformly
// distributed, so they are taken as they are. Two paths that resolve to the
// same CID get the same number on purpose: on a content-addressed tree they
// are the same object, and go-fuse then serves both from one node, which
// keeps a single page cache for the content behind both names.
func InoFromCid(c cid.Cid) uint64 {
	if dec, err := mh.Decode(c.Hash()); err == nil && dec.Code != mh.IDENTITY && len(dec.Digest) >= 8 {
		return boundIno(binary.BigEndian.Uint64(dec.Digest[:8]))
	}
	// An identity multihash carries the content itself rather than a digest
	// of it, so its leading bytes would give one number to every file that
	// starts alike. Digests shorter than eight bytes have too little to take.
	// Both cases fall back to hashing the whole CID.
	h := fnv.New64a()
	_, _ = h.Write([]byte(c.KeyString()))
	return boundIno(h.Sum64())
}

// boundIno moves a derived number into the range a mount may report,
// [FirstIno, AutomaticIno).
func boundIno(ino uint64) uint64 {
	ino &^= AutomaticIno
	if ino < FirstIno {
		ino += FirstIno
	}
	return ino
}
