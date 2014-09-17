package queue

import (
	"container/heap"
	"math/big"

	peer "github.com/jbenet/go-ipfs/peer"
	ks "github.com/jbenet/go-ipfs/routing/keyspace"
	u "github.com/jbenet/go-ipfs/util"
)

// peerDistance tracks a peer and its distance to something else.
type peerDistance struct {
	// the peer
	peer *peer.Peer

	// big.Int for XOR metric
	distance *big.Int
}

// distancePQ implements heap.Interface and PeerQueue
type distancePQ struct {
	// from is the Key this PQ measures against
	from ks.Key

	// peers is a heap of peerDistance items
	peers []*peerDistance
}

func (pq distancePQ) Len() int {
	return len(pq.peers)
}

func (pq distancePQ) Less(i, j int) bool {
	return -1 == pq.peers[i].distance.Cmp(pq.peers[j].distance)
}

func (pq distancePQ) Swap(i, j int) {
	p := pq.peers
	p[i], p[j] = p[j], p[i]
}

func (pq *distancePQ) Push(x interface{}) {
	item := x.(*peerDistance)
	pq.peers = append(pq.peers, item)
}

func (pq *distancePQ) Pop() interface{} {
	old := pq.peers
	n := len(old)
	item := old[n-1]
	pq.peers = old[0 : n-1]
	return item
}

func (pq *distancePQ) Enqueue(p *peer.Peer) {
	distance := ks.XORKeySpace.Key(p.ID).Distance(pq.from)

	heap.Push(pq, &peerDistance{
		peer:     p,
		distance: distance,
	})
}

func (pq *distancePQ) Dequeue() *peer.Peer {
	if len(pq.peers) < 1 {
		panic("called Dequeue on an empty PeerQueue")
		// will panic internally anyway, but we can help debug here
	}

	o := heap.Pop(pq)
	p := o.(*peerDistance)
	return p.peer
}

// NewXORDistancePQ returns a PeerQueue which maintains its peers sorted
// in terms of their distances to each other in an XORKeySpace (i.e. using
// XOR as a metric of distance).
func NewXORDistancePQ(fromKey u.Key) PeerQueue {
	return &distancePQ{
		from:  ks.XORKeySpace.Key([]byte(fromKey)),
		peers: []*peerDistance{},
	}
}
