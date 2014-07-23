package dht

import (
  "container/list"
)


// ID for IpfsDHT should be a byte slice, to allow for simpler operations
// (xor). DHT ids are based on the peer.IDs.
//
// NOTE: peer.IDs are biased because they are (a) multihashes (first bytes
// biased), and (b) first bits are zeroes when using the S/Kademlia PoW.
// Thus, may need to re-hash keys (uniform dist). TODO(jbenet)
type ID []byte

// Bucket holds a list of peers.
type Bucket []*list.List


// RoutingTable defines the routing table.
type RoutingTable struct {

  // kBuckets define all the fingers to other nodes.
  Buckets []Bucket
}


func (id ID) commonPrefixLen() int {
  for i := 0; i < len(id); i++ {
    for j := 0; j < 8; j++ {
      if (id[i] >> uint8(7 - j)) & 0x1 != 0 {
        return i * 8 + j;
      }
    }
  }
  return len(id) * 8 - 1;
}

func xor(a, b ID) ID {

  // ids may actually be of different sizes.
  var ba ID
  var bb ID
  if len(a) >= len(b) {
    ba = a
    bb = b
  } else {
    ba = b
    bb = a
  }

  c := make(ID, len(ba))
  for i := 0; i < len(ba); i++ {
    if len(bb) > i {
      c[i] = ba[i] ^ bb[i]
    } else {
      c[i] = ba[i] ^ 0
    }
  }
  return c
}
