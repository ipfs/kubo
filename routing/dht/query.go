package dht

import (
	peer "github.com/jbenet/go-ipfs/peer"
	queue "github.com/jbenet/go-ipfs/peer/queue"
	u "github.com/jbenet/go-ipfs/util"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
)

type dhtQuery struct {
	// a PeerQueue
	peers queue.PeerQueue

	// the function to execute per peer
	qfunc queryFunc
}

// QueryFunc is a function that runs a particular query with a given peer.
// It returns either:
// - the value
// - a list of peers potentially better able to serve the query
// - an error
type queryFunc func(context.Context, *peer.Peer) (interface{}, []*peer.Peer, error)

func (q *dhtQuery) Run(ctx context.Context, concurrency int) (interface{}, error) {
	// get own cancel function to signal when we've found the value
	ctx, cancel := context.WithCancel(ctx)

	// the variable waiting to be populated upon success
	var result interface{}

	// chanQueue is how workers receive their work
	chanQueue := queue.NewChanQueue(ctx, q.peers)

	// worker
	worker := func() {
		for {
			select {
			case p := <-chanQueue.DeqChan:

				val, closer, err := q.qfunc(ctx, p)
				if err != nil {
					u.PErr("error running query: %v\n", err)
					continue
				}

				if val != nil {
					result = val
					cancel() // signal we're done.
					return
				}

				if closer != nil {
					for _, p := range closer {
						select {
						case chanQueue.EnqChan <- p:
						case <-ctx.Done():
							return
						}
					}
				}

			case <-ctx.Done():
				return
			}
		}
	}

	// launch all workers
	for i := 0; i < concurrency; i++ {
		go worker()
	}

	// wait until we're done. yep.
	select {
	case <-ctx.Done():
	}

	if result != nil {
		return result, nil
	}

	return nil, ctx.Err()
}
