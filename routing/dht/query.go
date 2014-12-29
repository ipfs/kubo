package dht

import (
	"sync"

	inet "github.com/jbenet/go-ipfs/net"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	queue "github.com/jbenet/go-ipfs/p2p/peer/queue"
	"github.com/jbenet/go-ipfs/routing"
	u "github.com/jbenet/go-ipfs/util"
	pset "github.com/jbenet/go-ipfs/util/peerset"
	todoctr "github.com/jbenet/go-ipfs/util/todocounter"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ctxgroup "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-ctxgroup"
)

var maxQueryConcurrency = AlphaValue

type dhtQuery struct {
	// the key we're querying for
	key u.Key

	// dialer used to ensure we're connected to peers
	dialer inet.Dialer

	// the function to execute per peer
	qfunc queryFunc

	// the concurrency parameter
	concurrency int
}

type dhtQueryResult struct {
	value         []byte          // GetValue
	peer          peer.PeerInfo   // FindPeer
	providerPeers []peer.PeerInfo // GetProviders
	closerPeers   []peer.PeerInfo // *
	success       bool
}

// constructs query
func newQuery(k u.Key, d inet.Dialer, f queryFunc) *dhtQuery {
	return &dhtQuery{
		key:         k,
		dialer:      d,
		qfunc:       f,
		concurrency: maxQueryConcurrency,
	}
}

// QueryFunc is a function that runs a particular query with a given peer.
// It returns either:
// - the value
// - a list of peers potentially better able to serve the query
// - an error
type queryFunc func(context.Context, peer.ID) (*dhtQueryResult, error)

// Run runs the query at hand. pass in a list of peers to use first.
func (q *dhtQuery) Run(ctx context.Context, peers []peer.ID) (*dhtQueryResult, error) {
	runner := newQueryRunner(ctx, q)
	return runner.Run(peers)
}

type dhtQueryRunner struct {

	// the query to run
	query *dhtQuery

	// peersToQuery is a list of peers remaining to query
	peersToQuery *queue.ChanQueue

	// peersSeen are all the peers queried. used to prevent querying same peer 2x
	peersSeen *pset.PeerSet

	// rateLimit is a channel used to rate limit our processing (semaphore)
	rateLimit chan struct{}

	// peersRemaining is a counter of peers remaining (toQuery + processing)
	peersRemaining todoctr.Counter

	// context group
	cg ctxgroup.ContextGroup

	// result
	result *dhtQueryResult

	// result errors
	errs []error

	// lock for concurrent access to fields
	sync.RWMutex
}

func newQueryRunner(ctx context.Context, q *dhtQuery) *dhtQueryRunner {
	return &dhtQueryRunner{
		query:          q,
		peersToQuery:   queue.NewChanQueue(ctx, queue.NewXORDistancePQ(q.key)),
		peersRemaining: todoctr.NewSyncCounter(),
		peersSeen:      pset.New(),
		rateLimit:      make(chan struct{}, q.concurrency),
		cg:             ctxgroup.WithContext(ctx),
	}
}

func (r *dhtQueryRunner) Run(peers []peer.ID) (*dhtQueryResult, error) {
	log.Debugf("Run query with %d peers.", len(peers))
	if len(peers) == 0 {
		log.Warning("Running query with no peers!")
		return nil, nil
	}

	// setup concurrency rate limiting
	for i := 0; i < r.query.concurrency; i++ {
		r.rateLimit <- struct{}{}
	}

	// add all the peers we got first.
	for _, p := range peers {
		r.addPeerToQuery(r.cg.Context(), p)
	}

	// go do this thing.
	// do it as a child func to make sure Run exits
	// ONLY AFTER spawn workers has exited.
	r.cg.AddChildFunc(r.spawnWorkers)

	// so workers are working.

	// wait until they're done.
	err := routing.ErrNotFound

	select {
	case <-r.peersRemaining.Done():
		r.cg.Close()
		r.RLock()
		defer r.RUnlock()

		if len(r.errs) > 0 {
			err = r.errs[0]
		}

	case <-r.cg.Closed():
		r.RLock()
		defer r.RUnlock()
		err = r.cg.Context().Err() // collect the error.
	}

	if r.result != nil && r.result.success {
		return r.result, nil
	}

	return nil, err
}

func (r *dhtQueryRunner) addPeerToQuery(ctx context.Context, next peer.ID) {
	// if new peer is ourselves...
	if next == r.query.dialer.LocalPeer() {
		return
	}

	if !r.peersSeen.TryAdd(next) {
		log.Debug("query peer was already seen")
		return
	}

	log.Debugf("adding peer to query: %v", next)

	// do this after unlocking to prevent possible deadlocks.
	r.peersRemaining.Increment(1)
	select {
	case r.peersToQuery.EnqChan <- next:
	case <-ctx.Done():
	}
}

func (r *dhtQueryRunner) spawnWorkers(parent ctxgroup.ContextGroup) {
	for {

		select {
		case <-r.peersRemaining.Done():
			return

		case <-r.cg.Closing():
			return

		case p, more := <-r.peersToQuery.DeqChan:
			if !more {
				return // channel closed.
			}
			log.Debugf("spawning worker for: %v", p)

			// do it as a child func to make sure Run exits
			// ONLY AFTER spawn workers has exited.
			parent.AddChildFunc(func(cg ctxgroup.ContextGroup) {
				r.queryPeer(cg, p)
			})
		}
	}
}

func (r *dhtQueryRunner) queryPeer(cg ctxgroup.ContextGroup, p peer.ID) {
	log.Debugf("spawned worker for: %v", p)

	// make sure we rate limit concurrency.
	select {
	case <-r.rateLimit:
	case <-cg.Closing():
		r.peersRemaining.Decrement(1)
		return
	}

	// ok let's do this!
	log.Debugf("running worker for: %v", p)

	// make sure we do this when we exit
	defer func() {
		// signal we're done proccessing peer p
		log.Debugf("completing worker for: %v", p)
		r.peersRemaining.Decrement(1)
		r.rateLimit <- struct{}{}
	}()

	// make sure we're connected to the peer.
	if conns := r.query.dialer.ConnsToPeer(p); len(conns) == 0 {
		log.Infof("worker for: %v -- not connected. dial start", p)

		if err := r.query.dialer.DialPeer(cg.Context(), p); err != nil {
			log.Debugf("ERROR worker for: %v -- err connecting: %v", p, err)
			r.Lock()
			r.errs = append(r.errs, err)
			r.Unlock()
			return
		}

		log.Infof("worker for: %v -- not connected. dial success!", p)
	}

	// finally, run the query against this peer
	res, err := r.query.qfunc(cg.Context(), p)

	if err != nil {
		log.Debugf("ERROR worker for: %v %v", p, err)
		r.Lock()
		r.errs = append(r.errs, err)
		r.Unlock()

	} else if res.success {
		log.Debugf("SUCCESS worker for: %v", p, res)
		r.Lock()
		r.result = res
		r.Unlock()
		go r.cg.Close() // signal to everyone that we're done.
		// must be async, as we're one of the children, and Close blocks.

	} else if len(res.closerPeers) > 0 {
		log.Debugf("PEERS CLOSER -- worker for: %v (%d closer peers)", p, len(res.closerPeers))
		for _, next := range res.closerPeers {
			// add their addresses to the dialer's peerstore
			conns := r.query.dialer.ConnsToPeer(next.ID)
			if len(conns) == 0 {
				log.Infof("PEERS CLOSER -- worker for %v FOUND NEW PEER: %s %s", p, next.ID, next.Addrs)
			}

			r.query.dialer.Peerstore().AddAddresses(next.ID, next.Addrs)
			r.addPeerToQuery(cg.Context(), next.ID)
			log.Debugf("PEERS CLOSER -- worker for: %v added %v (%v)", p, next.ID, next.Addrs)
		}
	} else {
		log.Debugf("QUERY worker for: %v - not found, and no closer peers.", p)
	}
}
