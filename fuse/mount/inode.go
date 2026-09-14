// Inode numbering rules shared by the FUSE mounts.
//
//go:build (linux || darwin || freebsd) && !nofuse

package mount

import (
	"crypto/sha256"
	"encoding/binary"

	"github.com/ipfs/go-cid"
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
// number itself. Mounts keep their own numbers below it, so a node that slips
// through unnumbered can never be mistaken for one that was numbered.
// NewMount pins fs.Options.FirstAutomaticIno to it rather than relying on
// go-fuse's default, which is the same value but free to change.
const AutomaticIno = 1 << 63

// InoFromCid derives an inode number for a node on an immutable mount, where
// the CID is the identity of the content and nothing else has to be tracked.
// Two paths that resolve to the same CID get the same number on purpose: on a
// content-addressed tree they are the same object, and go-fuse then serves
// both from one node, which keeps a single page cache for the content behind
// both names.
func InoFromCid(c cid.Cid) uint64 {
	ino, _ := InoGenFromCid(c)
	return ino
}

// InoGenFromCid derives both halves of a node's identity on an immutable
// mount: the inode number reported to userspace, and the generation that goes
// with it in fs.StableAttr.
//
// go-fuse matches a lookup against the nodes it already holds by the whole of
// StableAttr, so two CIDs that agree on all of it are served as one object,
// and whichever was looked up first answers for both. The inode number alone
// cannot carry that identity: it is 63 bits, which two of the files a busy
// mount serves are liable to share, and a mount serves whatever content it is
// asked for, including content chosen to collide. The generation adds another
// 64 bits, out of reach of both.
//
// Both numbers come from a hash of the codec together with the multihash. The
// codec has to be in there because the same block reached as raw and as
// dag-pb is decoded differently and is not the same file; the CID version is
// left out because a CIDv0 and a CIDv1 dag-pb of the same content are.
func InoGenFromCid(c cid.Cid) (ino, gen uint64) {
	var codec [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(codec[:], c.Type())

	h := sha256.New()
	_, _ = h.Write(codec[:n])
	_, _ = h.Write(c.Hash())

	var sum [sha256.Size]byte
	h.Sum(sum[:0])

	return boundIno(binary.BigEndian.Uint64(sum[:8])), binary.BigEndian.Uint64(sum[8:16])
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
