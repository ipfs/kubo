package queue

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	peer "github.com/jbenet/go-ipfs/peer"
)

// ChanQueue makes any PeerQueue synchronizable through channels.
type ChanQueue struct {
	Queue   PeerQueue
	EnqChan chan *peer.Peer
	DeqChan chan *peer.Peer
}

// NewChanQueue creates a ChanQueue by wrapping pq.
func NewChanQueue(ctx context.Context, pq PeerQueue) *ChanQueue {
	cq := &ChanQueue{
		Queue:   pq,
		EnqChan: make(chan *peer.Peer, 10),
		DeqChan: make(chan *peer.Peer, 10),
	}
	go cq.process(ctx)
	return cq
}

func (cq *ChanQueue) process(ctx context.Context) {
	var next *peer.Peer

	for {

		if cq.Queue.Len() == 0 {
			select {
			case next = <-cq.EnqChan:
			case <-ctx.Done():
				close(cq.DeqChan)
				return
			}

		} else {
			next = cq.Queue.Dequeue()
		}

		select {
		case item := <-cq.EnqChan:
			cq.Queue.Enqueue(item)
			cq.Queue.Enqueue(next)
			next = nil

		case cq.DeqChan <- next:
			next = nil

		case <-ctx.Done():
			close(cq.DeqChan)
			return
		}
	}
}
