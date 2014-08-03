package dht

import (
	"bytes"
	"container/list"
	"sort"

	"crypto/sha256"

	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

// ID for IpfsDHT should be a byte slice, to allow for simpler operations
// (xor). DHT ids are based on the peer.IDs.
//
// NOTE: peer.IDs are biased because they are multihashes (first bytes
// biased). Thus, may need to re-hash keys (uniform dist). TODO(jbenet)
type ID []byte

// Bucket holds a list of peers.
type Bucket list.List

func (b *Bucket) Find(id peer.ID) *list.Element {
	bucket_list := (*list.List)(b)
	for e := bucket_list.Front(); e != nil; e = e.Next() {
		if e.Value.(*peer.Peer).ID.Equal(id) {
			return e
		}
	}
	return nil
}

func (b *Bucket) MoveToFront(e *list.Element) {
	bucket_list := (*list.List)(b)
	bucket_list.MoveToFront(e)
}

func (b *Bucket) PushFront(p *peer.Peer) {
	bucket_list := (*list.List)(b)
	bucket_list.PushFront(p)
}

func (b *Bucket) PopBack() *peer.Peer {
	bucket_list := (*list.List)(b)
	last := bucket_list.Back()
	bucket_list.Remove(last)
	return last.Value.(*peer.Peer)
}

func (b *Bucket) Len() int {
	bucket_list := (*list.List)(b)
	return bucket_list.Len()
}

func (b *Bucket) Split(cpl int, target ID) *Bucket {
	bucket_list := (*list.List)(b)
	out := list.New()
	e := bucket_list.Front()
	for e != nil {
		peer_id := convertPeerID(e.Value.(*peer.Peer).ID)
		peer_cpl := xor(peer_id, target).commonPrefixLen()
		if peer_cpl > cpl {
			cur := e
			out.PushBack(e.Value)
			e = e.Next()
			bucket_list.Remove(cur)
			continue
		}
	}
	return (*Bucket)(out)
}

// RoutingTable defines the routing table.
type RoutingTable struct {

	// ID of the local peer
	local ID

	// kBuckets define all the fingers to other nodes.
	Buckets []*Bucket
	bucketsize int
}

func convertPeerID(id peer.ID) ID {
	hash := sha256.Sum256(id)
	return hash[:]
}

func convertKey(id u.Key) ID {
	hash := sha256.Sum256([]byte(id))
	return hash[:]
}

// Update adds or moves the given peer to the front of its respective bucket
// If a peer gets removed from a bucket, it is returned
func (rt *RoutingTable) Update(p *peer.Peer) *peer.Peer {
	peer_id := convertPeerID(p.ID)
	cpl := xor(peer_id, rt.local).commonPrefixLen()

	b_id := cpl
	if b_id >= len(rt.Buckets) {
		b_id = len(rt.Buckets) - 1
	}

	bucket := rt.Buckets[b_id]
	e := bucket.Find(p.ID)
	if e == nil {
		// New peer, add to bucket
		bucket.PushFront(p)

		// Are we past the max bucket size?
		if bucket.Len() > rt.bucketsize {
			if b_id == len(rt.Buckets) - 1 {
				new_bucket := bucket.Split(b_id, rt.local)
				rt.Buckets = append(rt.Buckets, new_bucket)

				// If all elements were on left side of split...
				if bucket.Len() > rt.bucketsize {
					return bucket.PopBack()
				}
			} else {
				// If the bucket cant split kick out least active node
				return bucket.PopBack()
			}
		}
		return nil
	} else {
		// If the peer is already in the table, move it to the front.
		// This signifies that it it "more active" and the less active nodes
		// Will as a result tend towards the back of the list
		bucket.MoveToFront(e)
		return nil
	}
}

// A helper struct to sort peers by their distance to the local node
type peerDistance struct {
	p *peer.Peer
	distance ID
}
type peerSorterArr []*peerDistance
func (p peerSorterArr) Len() int {return len(p)}
func (p peerSorterArr) Swap(a, b int) {p[a],p[b] = p[b],p[a]}
func (p peerSorterArr) Less(a, b int) bool {
	return p[a].distance.Less(p[b])
}
//

func (rt *RoutingTable) NearestPeer(id ID) *peer.Peer {
	peers := rt.NearestPeers(id, 1)
	return peers[0]
}

//TODO: make this accept an ID, requires method of converting keys to IDs
func (rt *RoutingTable) NearestPeers(id ID, count int) []*peer.Peer {
	cpl := xor(id, rt.local).commonPrefixLen()

	// Get bucket at cpl index or last bucket
	var bucket *Bucket
	if cpl >= len(rt.Buckets) {
		bucket = rt.Buckets[len(rt.Buckets) - 1]
	} else {
		bucket = rt.Buckets[cpl]
	}

	if bucket.Len() == 0 {
		// This can happen, very rarely.
		panic("Case not yet implemented.")
	}

	var peerArr peerSorterArr

	plist := (*list.List)(bucket)
	for e := plist.Front();e != nil; e = e.Next() {
		p := e.Value.(*peer.Peer)
		p_id := convertPeerID(p.ID)
		pd := peerDistance{
			p: p,
			distance: xor(rt.local, p_id),
		}
		peerArr = append(peerArr, &pd)
	}

	sort.Sort(peerArr)

	var out []*peer.Peer
	for i := 0; i < count && i < peerArr.Len(); i++ {
		out = append(out, peerArr[i].p)
	}

	return out
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
