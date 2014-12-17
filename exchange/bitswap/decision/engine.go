package decision

import (
	"sync"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	bstore "github.com/jbenet/go-ipfs/blocks/blockstore"
	bsmsg "github.com/jbenet/go-ipfs/exchange/bitswap/message"
	wl "github.com/jbenet/go-ipfs/exchange/bitswap/wantlist"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

// TODO consider taking responsibility for other types of requests. For
// example, there could be a |cancelQueue| for all of the cancellation
// messages that need to go out. There could also be a |wantlistQueue| for
// the local peer's wantlists. Alternatively, these could all be bundled
// into a single, intelligent global queue that efficiently
// batches/combines and takes all of these into consideration.
//
// Right now, messages go onto the network for four reasons:
// 1. an initial `sendwantlist` message to a provider of the first key in a request
// 2. a periodic full sweep of `sendwantlist` messages to all providers
// 3. upon receipt of blocks, a `cancel` message to all peers
// 4. draining the priority queue of `blockrequests` from peers
//
// Presently, only `blockrequests` are handled by the decision engine.
// However, there is an opportunity to give it more responsibility! If the
// decision engine is given responsibility for all of the others, it can
// intelligently decide how to combine requests efficiently.
//
// Some examples of what would be possible:
//
// * when sending out the wantlists, include `cancel` requests
// * when handling `blockrequests`, include `sendwantlist` and `cancel` as appropriate
// * when handling `cancel`, if we recently received a wanted block from a
// 	 peer, include a partial wantlist that contains a few other high priority
//   blocks
//
// In a sense, if we treat the decision engine as a black box, it could do
// whatever it sees fit to produce desired outcomes (get wanted keys
// quickly, maintain good relationships with peers, etc).

var log = u.Logger("engine")

const (
	sizeOutboxChan = 4
)

// Envelope contains a message for a Peer
type Envelope struct {
	// Peer is the intended recipient
	Peer peer.Peer
	// Message is the payload
	Message bsmsg.BitSwapMessage
}

type Engine struct {
	// peerRequestQueue is a priority queue of requests received from peers.
	// Requests are popped from the queue, packaged up, and placed in the
	// outbox.
	peerRequestQueue *taskQueue

	// FIXME it's a bit odd for the client and the worker to both share memory
	// (both modify the peerRequestQueue) and also to communicate over the
	// workSignal channel. consider sending requests over the channel and
	// allowing the worker to have exclusive access to the peerRequestQueue. In
	// that case, no lock would be required.
	workSignal chan struct{}

	// outbox contains outgoing messages to peers
	outbox chan Envelope

	bs bstore.Blockstore

	lock sync.RWMutex // protects the fields immediatly below
	// ledgerMap lists Ledgers by their Partner key.
	ledgerMap map[u.Key]*ledger
}

func NewEngine(ctx context.Context, bs bstore.Blockstore) *Engine {
	e := &Engine{
		ledgerMap:        make(map[u.Key]*ledger),
		bs:               bs,
		peerRequestQueue: newTaskQueue(),
		outbox:           make(chan Envelope, sizeOutboxChan),
		workSignal:       make(chan struct{}),
	}
	go e.taskWorker(ctx)
	return e
}

func (e *Engine) taskWorker(ctx context.Context) {
	for {
		nextTask := e.peerRequestQueue.Pop()
		if nextTask == nil {
			// No tasks in the list?
			// Wait until there are!
			select {
			case <-ctx.Done():
				return
			case <-e.workSignal:
			}
			continue
		}
		block, err := e.bs.Get(nextTask.Entry.Key)
		if err != nil {
			log.Warning("engine: task exists to send block, but block is not in blockstore")
			continue
		}
		// construct message here so we can make decisions about any additional
		// information we may want to include at this time.
		m := bsmsg.New()
		m.AddBlock(block)
		// TODO: maybe add keys from our wantlist?
		select {
		case <-ctx.Done():
			return
		case e.outbox <- Envelope{Peer: nextTask.Target, Message: m}:
		}
	}
}

func (e *Engine) Outbox() <-chan Envelope {
	return e.outbox
}

// Returns a slice of Peers with whom the local node has active sessions
func (e *Engine) Peers() []peer.Peer {
	e.lock.RLock()
	defer e.lock.RUnlock()

	response := make([]peer.Peer, 0)
	for _, ledger := range e.ledgerMap {
		response = append(response, ledger.Partner)
	}
	return response
}

// MessageReceived performs book-keeping. Returns error if passed invalid
// arguments.
func (e *Engine) MessageReceived(p peer.Peer, m bsmsg.BitSwapMessage) error {
	newWorkExists := false
	defer func() {
		if newWorkExists {
			// Signal task generation to restart (if stopped!)
			select {
			case e.workSignal <- struct{}{}:
			default:
			}
		}
	}()
	e.lock.Lock()
	defer e.lock.Unlock()

	l := e.findOrCreate(p)
	if m.Full() {
		l.wantList = wl.New()
	}
	for _, entry := range m.Wantlist() {
		if entry.Cancel {
			l.CancelWant(entry.Key)
			e.peerRequestQueue.Remove(entry.Key, p)
		} else {
			l.Wants(entry.Key, entry.Priority)
			if exists, err := e.bs.Has(entry.Key); err == nil && exists {
				newWorkExists = true
				e.peerRequestQueue.Push(entry.Entry, p)
			}
		}
	}

	for _, block := range m.Blocks() {
		// FIXME extract blocks.NumBytes(block) or block.NumBytes() method
		l.ReceivedBytes(len(block.Data))
		for _, l := range e.ledgerMap {
			if l.WantListContains(block.Key()) {
				newWorkExists = true
				e.peerRequestQueue.Push(wl.Entry{block.Key(), 1}, l.Partner)
			}
		}
	}
	return nil
}

// TODO add contents of m.WantList() to my local wantlist? NB: could introduce
// race conditions where I send a message, but MessageSent gets handled after
// MessageReceived. The information in the local wantlist could become
// inconsistent. Would need to ensure that Sends and acknowledgement of the
// send happen atomically

func (e *Engine) MessageSent(p peer.Peer, m bsmsg.BitSwapMessage) error {
	e.lock.Lock()
	defer e.lock.Unlock()

	l := e.findOrCreate(p)
	for _, block := range m.Blocks() {
		l.SentBytes(len(block.Data))
		l.wantList.Remove(block.Key())
		e.peerRequestQueue.Remove(block.Key(), p)
	}

	return nil
}

func (e *Engine) numBytesSentTo(p peer.Peer) uint64 {
	// NB not threadsafe
	return e.findOrCreate(p).Accounting.BytesSent
}

func (e *Engine) numBytesReceivedFrom(p peer.Peer) uint64 {
	// NB not threadsafe
	return e.findOrCreate(p).Accounting.BytesRecv
}

// ledger lazily instantiates a ledger
func (e *Engine) findOrCreate(p peer.Peer) *ledger {
	l, ok := e.ledgerMap[p.Key()]
	if !ok {
		l = newLedger(p)
		e.ledgerMap[p.Key()] = l
	}
	return l
}
