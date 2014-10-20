package queue

import peer "github.com/jbenet/go-ipfs/peer"

// PeerQueue maintains a set of peers ordered according to a metric.
// Implementations of PeerQueue could order peers based on distances along
// a KeySpace, latency measurements, trustworthiness, reputation, etc.
type PeerQueue interface {

	// Len returns the number of items in PeerQueue
	Len() int

	// Enqueue adds this node to the queue.
	Enqueue(peer.Peer)

	// Dequeue retrieves the highest (smallest int) priority node
	Dequeue() peer.Peer
}
