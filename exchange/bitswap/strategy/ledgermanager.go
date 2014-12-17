package strategy

import (
	"sync"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	bstore "github.com/jbenet/go-ipfs/blocks/blockstore"
	bsmsg "github.com/jbenet/go-ipfs/exchange/bitswap/message"
	wl "github.com/jbenet/go-ipfs/exchange/bitswap/wantlist"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

var log = u.Logger("strategy")

// LedgerMap lists Ledgers by their Partner key.
type ledgerMap map[u.Key]*ledger

type Envelope struct {
	Peer    peer.Peer
	Message bsmsg.BitSwapMessage
}

type LedgerManager struct {
	lock       sync.RWMutex
	ledgerMap  ledgerMap
	bs         bstore.Blockstore
	tasklist   *taskList
	outbox     chan Envelope
	workSignal chan struct{}
}

func NewLedgerManager(ctx context.Context, bs bstore.Blockstore) *LedgerManager {
	lm := &LedgerManager{
		ledgerMap:  make(ledgerMap),
		bs:         bs,
		tasklist:   newTaskList(),
		outbox:     make(chan Envelope, 4), // TODO extract constant
		workSignal: make(chan struct{}),
	}
	go lm.taskWorker(ctx)
	return lm
}

func (lm *LedgerManager) taskWorker(ctx context.Context) {
	for {
		nextTask := lm.tasklist.Pop()
		if nextTask == nil {
			// No tasks in the list?
			// Wait until there are!
			select {
			case <-ctx.Done():
				return
			case <-lm.workSignal:
			}
			continue
		}
		block, err := lm.bs.Get(nextTask.Key)
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
		case lm.outbox <- Envelope{Peer: nextTask.Target, Message: m}:
		}
	}
}

func (lm *LedgerManager) Outbox() <-chan Envelope {
	return lm.outbox
}

// Returns a slice of Peers with whom the local node has active sessions
func (lm *LedgerManager) Peers() []peer.Peer {
	lm.lock.RLock()
	defer lm.lock.RUnlock()

	response := make([]peer.Peer, 0)
	for _, ledger := range lm.ledgerMap {
		response = append(response, ledger.Partner)
	}
	return response
}

// BlockIsWantedByPeer returns true if peer wants the block given by this
// key
func (lm *LedgerManager) BlockIsWantedByPeer(k u.Key, p peer.Peer) bool {
	lm.lock.RLock()
	defer lm.lock.RUnlock()

	ledger := lm.findOrCreate(p)
	return ledger.WantListContains(k)
}

// MessageReceived performs book-keeping. Returns error if passed invalid
// arguments.
func (lm *LedgerManager) MessageReceived(p peer.Peer, m bsmsg.BitSwapMessage) error {
	lm.lock.Lock()
	defer lm.lock.Unlock()

	l := lm.findOrCreate(p)
	if m.Full() {
		l.wantList = wl.New()
	}
	for _, e := range m.Wantlist() {
		if e.Cancel {
			l.CancelWant(e.Key)
			lm.tasklist.Cancel(e.Key, p)
		} else {
			l.Wants(e.Key, e.Priority)
			lm.tasklist.Push(e.Key, e.Priority, p)

			// Signal task generation to restart (if stopped!)
			select {
			case lm.workSignal <- struct{}{}:
			default:
			}
		}
	}

	for _, block := range m.Blocks() {
		// FIXME extract blocks.NumBytes(block) or block.NumBytes() method
		l.ReceivedBytes(len(block.Data))
		for _, l := range lm.ledgerMap {
			if l.WantListContains(block.Key()) {
				lm.tasklist.Push(block.Key(), 1, l.Partner)

				// Signal task generation to restart (if stopped!)
				select {
				case lm.workSignal <- struct{}{}:
				default:
				}

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

func (lm *LedgerManager) MessageSent(p peer.Peer, m bsmsg.BitSwapMessage) error {
	lm.lock.Lock()
	defer lm.lock.Unlock()

	l := lm.findOrCreate(p)
	for _, block := range m.Blocks() {
		l.SentBytes(len(block.Data))
		l.wantList.Remove(block.Key())
		lm.tasklist.Cancel(block.Key(), p)
	}

	return nil
}

func (lm *LedgerManager) NumBytesSentTo(p peer.Peer) uint64 {
	lm.lock.RLock()
	defer lm.lock.RUnlock()

	return lm.findOrCreate(p).Accounting.BytesSent
}

func (lm *LedgerManager) NumBytesReceivedFrom(p peer.Peer) uint64 {
	lm.lock.RLock()
	defer lm.lock.RUnlock()

	return lm.findOrCreate(p).Accounting.BytesRecv
}

// ledger lazily instantiates a ledger
func (lm *LedgerManager) findOrCreate(p peer.Peer) *ledger {
	l, ok := lm.ledgerMap[p.Key()]
	if !ok {
		l = newLedger(p)
		lm.ledgerMap[p.Key()] = l
	}
	return l
}
