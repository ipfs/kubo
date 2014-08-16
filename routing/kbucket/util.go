package dht

import (
	"bytes"
	"crypto/sha256"
	"errors"

	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

// Returned if a routing table query returns no results. This is NOT expected
// behaviour
var ErrLookupFailure = errors.New("failed to find any peer in table")

// ID for IpfsDHT should be a byte slice, to allow for simpler operations
// (xor). DHT ids are based on the peer.IDs.
//
// The type dht.ID signifies that its contents have been hashed from either a
// peer.ID or a util.Key. This unifies the keyspace
type ID []byte

func (id ID) Equal(other ID) bool {
	return bytes.Equal(id, other)
}

func (id ID) Less(other ID) bool {
	a, b := equalizeSizes(id, other)
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return len(a) < len(b)
}

func (id ID) commonPrefixLen() int {
	for i := 0; i < len(id); i++ {
		for j := 0; j < 8; j++ {
			if (id[i]>>uint8(7-j))&0x1 != 0 {
				return i*8 + j
			}
		}
	}
	return len(id)*8 - 1
}

func prefLen(a, b ID) int {
	return xor(a, b).commonPrefixLen()
}

func xor(a, b ID) ID {
	a, b = equalizeSizes(a, b)

	c := make(ID, len(a))
	for i := 0; i < len(a); i++ {
		c[i] = a[i] ^ b[i]
	}
	return c
}

func equalizeSizes(a, b ID) (ID, ID) {
	la := len(a)
	lb := len(b)

	if la < lb {
		na := make([]byte, lb)
		copy(na, a)
		a = na
	} else if lb < la {
		nb := make([]byte, la)
		copy(nb, b)
		b = nb
	}

	return a, b
}

func ConvertPeerID(id peer.ID) ID {
	hash := sha256.Sum256(id)
	return hash[:]
}

func ConvertKey(id u.Key) ID {
	hash := sha256.Sum256([]byte(id))
	return hash[:]
}

// Returns true if a is closer to key than b is
func Closer(a, b peer.ID, key u.Key) bool {
	aid := ConvertPeerID(a)
	bid := ConvertPeerID(b)
	tgt := ConvertKey(key)
	adist := xor(aid, tgt)
	bdist := xor(bid, tgt)

	return adist.Less(bdist)
}
