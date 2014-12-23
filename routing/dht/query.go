package dht

import (
	"sync"

	inet "github.com/jbenet/go-ipfs/net"
	peer "github.com/jbenet/go-ipfs/peer"
	queue "github.com/jbenet/go-ipfs/peer/queue"
	"github.com/jbenet/go-ipfs/routing"
	kb "github.com/jbenet/go-ipfs/routing/kbucket"
	u "github.com/jbenet/go-ipfs/util"
	todoctr "github.com/jbenet/go-ipfs/util/todocounter"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
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
	peersSeen peer.Set

	// rateLimit is a channel used to rate limit our processing (semaphore)
	rateLimit chan struct{}

	// peersRemaining is a counter of peers remaining (toQuery + processing)
	peersRemaining todoctr.Counter

	// context
	ctx    context.Context
	cancel context.CancelFunc

	// result
	result *dhtQueryResult

	// result errors
	errs []error

	// lock for concurrent access to fields
	sync.RWMutex
}

func newQueryRunner(ctx context.Context, q *dhtQuery) *dhtQueryRunner {
	ctx, cancel := context.WithCancel(ctx)

	return &dhtQueryRunner{
		ctx:            ctx,
		cancel:         cancel,
		query:          q,
		peersToQuery:   queue.NewChanQueue(ctx, queue.NewXORDistancePQ(q.key)),
		peersRemaining: todoctr.NewSyncCounter(),
		peersSeen:      peer.Set{},
		rateLimit:      make(chan struct{}, q.concurrency),
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
		r.addPeerToQuery(p, "") // don't have access to self here...
	}

	// go do this thing.
	go r.spawnWorkers()

	// so workers are working.

	// wait until they're done.
	err := routing.ErrNotFound

	select {
	case <-r.peersRemaining.Done():
		r.cancel() // ran all and nothing. cancel all outstanding workers.
		r.RLock()
		defer r.RUnlock()

		if len(r.errs) > 0 {
			err = r.errs[0]
		}

	case <-r.ctx.Done():
		r.RLock()
		defer r.RUnlock()
		err = r.ctx.Err()
	}

	if r.result != nil && r.result.success {
		return r.result, nil
	}

	return nil, err
}

func (r *dhtQueryRunner) addPeerToQuery(next peer.ID, benchmark peer.ID) {
	// if new peer is ourselves...
	if next == r.query.dialer.LocalPeer() {
		return
	}

	// if new peer further away than whom we got it from, don't bother (loops)
	// TODO----------- this benchmark should be replaced by a heap:
	// we should be doing the s/kademlia "continue to search"
	// (i.e. put all of them in a heap sorted by dht distance and then just
	// pull from the the top until  a) you exhaust all peers you get,
	// b) you succeed, c) your context expires.
	if benchmark != "" && kb.Closer(benchmark, next, r.query.key) {
		return
	}

	// if already seen, no need.
	r.Lock()
	_, found := r.peersSeen[next]
	if found {
		r.Unlock()
		return
	}
	r.peersSeen[next] = struct{}{}
	r.Unlock()

	log.Debugf("adding peer to query: %v\n", next)

	// do this after unlocking to prevent possible deadlocks.
	r.peersRemaining.Increment(1)
	select {
	case r.peersToQuery.EnqChan <- next:
	case <-r.ctx.Done():
	}
}

func (r *dhtQueryRunner) spawnWorkers() {
	for {

		select {
		case <-r.peersRemaining.Done():
			return

		case <-r.ctx.Done():
			return

		case p, more := <-r.peersToQuery.DeqChan:
			if !more {
				return // channel closed.
			}
			log.Debugf("spawning worker for: %v\n", p)
			go r.queryPeer(p)
		}
	}
}

func (r *dhtQueryRunner) queryPeer(p peer.ID) {
	log.Debugf("spawned worker for: %v", p)

	// make sure we rate limit concurrency.
	select {
	case <-r.rateLimit:
	case <-r.ctx.Done():
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
	err := r.query.dialer.DialPeer(r.ctx, p)
	if err != nil {
		log.Debugf("ERROR worker for: %v -- err connecting: %v", p, err)
		r.Lock()
		r.errs = append(r.errs, err)
		r.Unlock()
		return
	}

	// finally, run the query against this peer
	res, err := r.query.qfunc(r.ctx, p)

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
		r.cancel() // signal to everyone that we're done.

	} else if len(res.closerPeers) > 0 {
		log.Debugf("PEERS CLOSER -- worker for: %v (%d closer peers)", p, len(res.closerPeers))
		for _, next := range res.closerPeers {
			// add their addresses to the dialer's peerstore
			r.query.dialer.Peerstore().AddAddresses(next.ID, next.Addrs)
			r.addPeerToQuery(next.ID, p)
			log.Debugf("PEERS CLOSER -- worker for: %v added %v (%v)", p, next.ID, next.Addrs)
		}
	} else {
		log.Debugf("QUERY worker for: %v - not found, and no closer peers.", p)
	}
}
