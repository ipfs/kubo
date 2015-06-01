package dht

import (
	"sync"

	key "github.com/ipfs/go-ipfs/blocks/key"
	notif "github.com/ipfs/go-ipfs/notifications"
	peer "github.com/ipfs/go-ipfs/p2p/peer"
	queue "github.com/ipfs/go-ipfs/p2p/peer/queue"
	"github.com/ipfs/go-ipfs/routing"
	eventlog "github.com/ipfs/go-ipfs/thirdparty/eventlog"
	u "github.com/ipfs/go-ipfs/util"
	pset "github.com/ipfs/go-ipfs/util/peerset"
	todoctr "github.com/ipfs/go-ipfs/util/todocounter"

	process "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/goprocess"
	ctxproc "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/goprocess/context"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
)

var maxQueryConcurrency = AlphaValue

type dhtQuery struct {
	dht         *IpfsDHT
	key         key.Key   // the key we're querying for
	qfunc       queryFunc // the function to execute per peer
	concurrency int       // the concurrency parameter
}

type dhtQueryResult struct {
	value         []byte          // GetValue
	peer          peer.PeerInfo   // FindPeer
	providerPeers []peer.PeerInfo // GetProviders
	closerPeers   []peer.PeerInfo // *
	success       bool
}

// constructs query
func (dht *IpfsDHT) newQuery(k key.Key, f queryFunc) *dhtQuery {
	return &dhtQuery{
		key:         k,
		dht:         dht,
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
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	runner := newQueryRunner(q)
	return runner.Run(ctx, peers)
}

type dhtQueryRunner struct {
	query          *dhtQuery        // query to run
	peersSeen      *pset.PeerSet    // all peers queried. prevent querying same peer 2x
	peersToQuery   *queue.ChanQueue // peers remaining to be queried
	peersRemaining todoctr.Counter  // peersToQuery + currently processing

	result *dhtQueryResult // query result
	errs   u.MultiErr      // result errors. maybe should be a map[peer.ID]error

	rateLimit chan struct{} // processing semaphore
	log       eventlog.EventLogger

	proc process.Process
	sync.RWMutex
}

func newQueryRunner(q *dhtQuery) *dhtQueryRunner {
	proc := process.WithParent(process.Background())
	ctx := ctxproc.WithProcessClosing(context.Background(), proc)
	return &dhtQueryRunner{
		query:          q,
		peersToQuery:   queue.NewChanQueue(ctx, queue.NewXORDistancePQ(q.key)),
		peersRemaining: todoctr.NewSyncCounter(),
		peersSeen:      pset.New(),
		rateLimit:      make(chan struct{}, q.concurrency),
		proc:           proc,
	}
}

func (r *dhtQueryRunner) Run(ctx context.Context, peers []peer.ID) (*dhtQueryResult, error) {
	r.log = log

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
		r.addPeerToQuery(p)
	}

	// go do this thing.
	// do it as a child proc to make sure Run exits
	// ONLY AFTER spawn workers has exited.
	r.proc.Go(r.spawnWorkers)

	// so workers are working.

	// wait until they're done.
	err := routing.ErrNotFound

	// now, if the context finishes, close the proc.
	// we have to do it here because the logic before is setup, which
	// should run without closing the proc.
	go func() {
		<-ctx.Done()
		r.proc.Close()
	}()

	select {
	case <-r.peersRemaining.Done():
		r.proc.Close()
		r.RLock()
		defer r.RUnlock()

		err = routing.ErrNotFound

		// if every query to every peer failed, something must be very wrong.
		if len(r.errs) > 0 && len(r.errs) == r.peersSeen.Size() {
			log.Debugf("query errs: %s", r.errs)
			err = r.errs[0]
		}

	case <-r.proc.Closed():
		r.RLock()
		defer r.RUnlock()
		err = context.DeadlineExceeded
	}

	if r.result != nil && r.result.success {
		return r.result, nil
	}

	return nil, err
}

func (r *dhtQueryRunner) addPeerToQuery(next peer.ID) {
	// if new peer is ourselves...
	if next == r.query.dht.self {
		r.log.Debug("addPeerToQuery skip self")
		return
	}

	if !r.peersSeen.TryAdd(next) {
		return
	}

	r.peersRemaining.Increment(1)
	select {
	case r.peersToQuery.EnqChan <- next:
	case <-r.proc.Closing():
	}
}

func (r *dhtQueryRunner) spawnWorkers(proc process.Process) {
	for {

		select {
		case <-r.peersRemaining.Done():
			return

		case <-r.proc.Closing():
			return

		case p, more := <-r.peersToQuery.DeqChan:
			if !more {
				return // channel closed.
			}

			// do it as a child func to make sure Run exits
			// ONLY AFTER spawn workers has exited.
			proc.Go(func(proc process.Process) {
				r.queryPeer(proc, p)
			})
		}
	}
}

func (r *dhtQueryRunner) queryPeer(proc process.Process, p peer.ID) {
	// make sure we rate limit concurrency.
	select {
	case <-r.rateLimit:
	case <-proc.Closing():
		r.peersRemaining.Decrement(1)
		return
	}

	// ok let's do this!

	// create a context from our proc.
	ctx := ctxproc.WithProcessClosing(context.Background(), proc)

	// make sure we do this when we exit
	defer func() {
		// signal we're done proccessing peer p
		r.peersRemaining.Decrement(1)
		r.rateLimit <- struct{}{}
	}()

	// make sure we're connected to the peer.
	// FIXME abstract away into the network layer
	if conns := r.query.dht.host.Network().ConnsToPeer(p); len(conns) == 0 {
		log.Infof("not connected. dialing.")
		// while we dial, we do not take up a rate limit. this is to allow
		// forward progress during potentially very high latency dials.
		r.rateLimit <- struct{}{}

		pi := peer.PeerInfo{ID: p}

		if err := r.query.dht.host.Connect(ctx, pi); err != nil {
			log.Debugf("Error connecting: %s", err)

			notif.PublishQueryEvent(ctx, &notif.QueryEvent{
				Type:  notif.QueryError,
				Extra: err.Error(),
			})

			r.Lock()
			r.errs = append(r.errs, err)
			r.Unlock()
			<-r.rateLimit // need to grab it again, as we deferred.
			return
		}
		<-r.rateLimit // need to grab it again, as we deferred.
		log.Debugf("connected. dial success.")
	}

	// finally, run the query against this peer
	res, err := r.query.qfunc(ctx, p)

	if err != nil {
		log.Debugf("ERROR worker for: %v %v", p, err)
		r.Lock()
		r.errs = append(r.errs, err)
		r.Unlock()

	} else if res.success {
		log.Debugf("SUCCESS worker for: %v %s", p, res)
		r.Lock()
		r.result = res
		r.Unlock()
		go r.proc.Close() // signal to everyone that we're done.
		// must be async, as we're one of the children, and Close blocks.

	} else if len(res.closerPeers) > 0 {
		log.Debugf("PEERS CLOSER -- worker for: %v (%d closer peers)", p, len(res.closerPeers))
		for _, next := range res.closerPeers {
			if next.ID == r.query.dht.self { // dont add self.
				log.Debugf("PEERS CLOSER -- worker for: %v found self", p)
				continue
			}

			// add their addresses to the dialer's peerstore
			r.query.dht.peerstore.AddAddrs(next.ID, next.Addrs, peer.TempAddrTTL)
			r.addPeerToQuery(next.ID)
			log.Debugf("PEERS CLOSER -- worker for: %v added %v (%v)", p, next.ID, next.Addrs)
		}
	} else {
		log.Debugf("QUERY worker for: %v - not found, and no closer peers.", p)
	}
}
