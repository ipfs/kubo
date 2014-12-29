package queue

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	peer "github.com/jbenet/go-ipfs/p2p/peer"
)

// ChanQueue makes any PeerQueue synchronizable through channels.
type ChanQueue struct {
	Queue   PeerQueue
	EnqChan chan<- peer.ID
	DeqChan <-chan peer.ID
}

// NewChanQueue creates a ChanQueue by wrapping pq.
func NewChanQueue(ctx context.Context, pq PeerQueue) *ChanQueue {
	cq := &ChanQueue{Queue: pq}
	cq.process(ctx)
	return cq
}

func (cq *ChanQueue) process(ctx context.Context) {

	// construct the channels here to be able to use them bidirectionally
	enqChan := make(chan peer.ID)
	deqChan := make(chan peer.ID)

	cq.EnqChan = enqChan
	cq.DeqChan = deqChan

	go func() {
		defer close(deqChan)

		var next peer.ID
		var item peer.ID
		var more bool

		for {
			if cq.Queue.Len() == 0 {
				select {
				case next, more = <-enqChan:
					if !more {
						return
					}

				case <-ctx.Done():
					return
				}

			} else {
				next = cq.Queue.Dequeue()
			}

			select {
			case item, more = <-enqChan:
				if !more {
					return
				}

				cq.Queue.Enqueue(item)
				cq.Queue.Enqueue(next)
				next = ""

			case deqChan <- next:
				next = ""

			case <-ctx.Done():
				return
			}
		}

	}()
}
