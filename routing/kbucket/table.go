// package kbucket implements a kademlia 'k-bucket' routing table.
package kbucket

import (
	"fmt"
	"sort"
	"sync"
	"time"

	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

var log = u.Logger("table")

// RoutingTable defines the routing table.
type RoutingTable struct {

	// ID of the local peer
	local ID

	// Blanket lock, refine later for better performance
	tabLock sync.RWMutex

	// latency metrics
	metrics peer.Metrics

	// Maximum acceptable latency for peers in this cluster
	maxLatency time.Duration

	// kBuckets define all the fingers to other nodes.
	Buckets    []*Bucket
	bucketsize int
}

// NewRoutingTable creates a new routing table with a given bucketsize, local ID, and latency tolerance.
func NewRoutingTable(bucketsize int, localID ID, latency time.Duration, m peer.Metrics) *RoutingTable {
	rt := new(RoutingTable)
	rt.Buckets = []*Bucket{newBucket()}
	rt.bucketsize = bucketsize
	rt.local = localID
	rt.maxLatency = latency
	rt.metrics = m
	return rt
}

// Update adds or moves the given peer to the front of its respective bucket
// If a peer gets removed from a bucket, it is returned
func (rt *RoutingTable) Update(p peer.ID) peer.ID {
	rt.tabLock.Lock()
	defer rt.tabLock.Unlock()
	peerID := ConvertPeerID(p)
	cpl := commonPrefixLen(peerID, rt.local)

	bucketID := cpl
	if bucketID >= len(rt.Buckets) {
		bucketID = len(rt.Buckets) - 1
	}

	bucket := rt.Buckets[bucketID]
	e := bucket.find(p)
	if e == nil {
		// New peer, add to bucket
		if rt.metrics.LatencyEWMA(p) > rt.maxLatency {
			// Connection doesnt meet requirements, skip!
			return ""
		}
		bucket.pushFront(p)

		// Are we past the max bucket size?
		if bucket.len() > rt.bucketsize {
			// If this bucket is the rightmost bucket, and its full
			// we need to split it and create a new bucket
			if bucketID == len(rt.Buckets)-1 {
				return rt.nextBucket()
			} else {
				// If the bucket cant split kick out least active node
				return bucket.popBack()
			}
		}
		return ""
	}
	// If the peer is already in the table, move it to the front.
	// This signifies that it it "more active" and the less active nodes
	// Will as a result tend towards the back of the list
	bucket.moveToFront(e)
	return ""
}

func (rt *RoutingTable) nextBucket() peer.ID {
	bucket := rt.Buckets[len(rt.Buckets)-1]
	newBucket := bucket.Split(len(rt.Buckets)-1, rt.local)
	rt.Buckets = append(rt.Buckets, newBucket)
	if newBucket.len() > rt.bucketsize {
		return rt.nextBucket()
	}

	// If all elements were on left side of split...
	if bucket.len() > rt.bucketsize {
		return bucket.popBack()
	}
	return ""
}

// Find a specific peer by ID or return nil
func (rt *RoutingTable) Find(id peer.ID) peer.ID {
	srch := rt.NearestPeers(ConvertPeerID(id), 1)
	if len(srch) == 0 || srch[0] != id {
		return ""
	}
	return srch[0]
}

// NearestPeer returns a single peer that is nearest to the given ID
func (rt *RoutingTable) NearestPeer(id ID) peer.ID {
	peers := rt.NearestPeers(id, 1)
	if len(peers) > 0 {
		return peers[0]
	}

	log.Errorf("NearestPeer: Returning nil, table size = %d", rt.Size())
	return ""
}

// NearestPeers returns a list of the 'count' closest peers to the given ID
func (rt *RoutingTable) NearestPeers(id ID, count int) []peer.ID {
	rt.tabLock.RLock()
	defer rt.tabLock.RUnlock()
	cpl := commonPrefixLen(id, rt.local)

	// Get bucket at cpl index or last bucket
	var bucket *Bucket
	if cpl >= len(rt.Buckets) {
		cpl = len(rt.Buckets) - 1
	}
	bucket = rt.Buckets[cpl]

	var peerArr peerSorterArr
	if bucket.len() == 0 {
		// In the case of an unusual split, one bucket may be empty.
		// if this happens, search both surrounding buckets for nearest peer
		if cpl > 0 {
			plist := rt.Buckets[cpl-1].list
			peerArr = copyPeersFromList(id, peerArr, plist)
		}

		if cpl < len(rt.Buckets)-1 {
			plist := rt.Buckets[cpl+1].list
			peerArr = copyPeersFromList(id, peerArr, plist)
		}
	} else {
		peerArr = copyPeersFromList(id, peerArr, bucket.list)
	}

	// Sort by distance to local peer
	sort.Sort(peerArr)

	var out []peer.ID
	for i := 0; i < count && i < peerArr.Len(); i++ {
		out = append(out, peerArr[i].p)
	}

	return out
}

// Size returns the total number of peers in the routing table
func (rt *RoutingTable) Size() int {
	var tot int
	for _, buck := range rt.Buckets {
		tot += buck.len()
	}
	return tot
}

// ListPeers takes a RoutingTable and returns a list of all peers from all buckets in the table.
// NOTE: This is potentially unsafe... use at your own risk
func (rt *RoutingTable) ListPeers() []peer.ID {
	var peers []peer.ID
	for _, buck := range rt.Buckets {
		for e := buck.getIter(); e != nil; e = e.Next() {
			peers = append(peers, e.Value.(peer.ID))
		}
	}
	return peers
}

// Print prints a descriptive statement about the provided RoutingTable
func (rt *RoutingTable) Print() {
	fmt.Printf("Routing Table, bs = %d, Max latency = %d\n", rt.bucketsize, rt.maxLatency)
	rt.tabLock.RLock()

	for i, b := range rt.Buckets {
		fmt.Printf("\tbucket: %d\n", i)

		b.lk.RLock()
		for e := b.list.Front(); e != nil; e = e.Next() {
			p := e.Value.(peer.ID)
			fmt.Printf("\t\t- %s %s\n", p.Pretty(), rt.metrics.LatencyEWMA(p).String())
		}
		b.lk.RUnlock()
	}
	rt.tabLock.RUnlock()
}
