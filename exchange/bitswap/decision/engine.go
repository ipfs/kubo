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

var log = u.Logger("engine")

// Envelope contains a message for a Peer
type Envelope struct {
	// Peer is the intended recipient
	Peer peer.Peer
	// Message is the payload
	Message bsmsg.BitSwapMessage
}

type Engine struct {
	// FIXME taskqueue isn't threadsafe nor is it protected by a mutex. consider
	// a way to avoid sharing the taskqueue between the worker and the receiver
	taskqueue *taskQueue

	workSignal chan struct{}

	outbox chan Envelope

	bs bstore.Blockstore

	lock sync.RWMutex
	// ledgerMap lists Ledgers by their Partner key.
	ledgerMap map[u.Key]*ledger
}

func NewEngine(ctx context.Context, bs bstore.Blockstore) *Engine {
	e := &Engine{
		ledgerMap:  make(map[u.Key]*ledger),
		bs:         bs,
		taskqueue:  newTaskQueue(),
		outbox:     make(chan Envelope, 4), // TODO extract constant
		workSignal: make(chan struct{}),
	}
	go e.taskWorker(ctx)
	return e
}

func (e *Engine) taskWorker(ctx context.Context) {
	for {
		nextTask := e.taskqueue.Pop()
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
			continue // TODO maybe return an error
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

// BlockIsWantedByPeer returns true if peer wants the block given by this
// key
func (e *Engine) BlockIsWantedByPeer(k u.Key, p peer.Peer) bool {
	e.lock.RLock()
	defer e.lock.RUnlock()

	ledger := e.findOrCreate(p)
	return ledger.WantListContains(k)
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
			e.taskqueue.Remove(entry.Key, p)
		} else {
			l.Wants(entry.Key, entry.Priority)
			newWorkExists = true
			e.taskqueue.Push(entry.Key, entry.Priority, p)
		}
	}

	for _, block := range m.Blocks() {
		// FIXME extract blocks.NumBytes(block) or block.NumBytes() method
		l.ReceivedBytes(len(block.Data))
		for _, l := range e.ledgerMap {
			if l.WantListContains(block.Key()) {
				newWorkExists = true
				e.taskqueue.Push(block.Key(), 1, l.Partner)
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
		e.taskqueue.Remove(block.Key(), p)
	}

	return nil
}

func (e *Engine) NumBytesSentTo(p peer.Peer) uint64 {
	e.lock.RLock()
	defer e.lock.RUnlock()

	return e.findOrCreate(p).Accounting.BytesSent
}

func (e *Engine) NumBytesReceivedFrom(p peer.Peer) uint64 {
	e.lock.RLock()
	defer e.lock.RUnlock()

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
