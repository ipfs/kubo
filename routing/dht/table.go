package dht

import (
	"bytes"
	"container/list"
)

// ID for IpfsDHT should be a byte slice, to allow for simpler operations
// (xor). DHT ids are based on the peer.IDs.
//
// NOTE: peer.IDs are biased because they are multihashes (first bytes
// biased). Thus, may need to re-hash keys (uniform dist). TODO(jbenet)
type ID []byte

// Bucket holds a list of peers.
type Bucket []*list.List

// RoutingTable defines the routing table.
type RoutingTable struct {

	// kBuckets define all the fingers to other nodes.
	Buckets []Bucket
}

func (id ID) Equal(other ID) bool {
	return bytes.Equal(id, other)
}

func (id ID) Less(other interface{}) bool {
	a, b := equalizeSizes(id, other.(ID))
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
