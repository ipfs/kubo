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
type ledgerMap map[peerKey]*ledger

// FIXME share this externally
type peerKey u.Key

type LedgerManager struct {
	lock       sync.RWMutex
	ledgerMap  ledgerMap
	bs         bstore.Blockstore
	tasklist   *TaskList
	taskOut    chan *Task
	workSignal chan struct{}
	ctx        context.Context
}

func NewLedgerManager(bs bstore.Blockstore, ctx context.Context) *LedgerManager {
	lm := &LedgerManager{
		ledgerMap:  make(ledgerMap),
		bs:         bs,
		tasklist:   NewTaskList(),
		taskOut:    make(chan *Task, 4),
		workSignal: make(chan struct{}),
		ctx:        ctx,
	}
	go lm.taskWorker()
	return lm
}

func (lm *LedgerManager) taskWorker() {
	for {
		nextTask := lm.tasklist.Pop()
		if nextTask == nil {
			// No tasks in the list?
			// Wait until there are!
			select {
			case <-lm.ctx.Done():
				return
			case <-lm.workSignal:
			}
			continue
		}

		select {
		case <-lm.ctx.Done():
			return
		case lm.taskOut <- nextTask:
		}
	}
}

func (lm *LedgerManager) GetTaskChan() <-chan *Task {
	return lm.taskOut
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

	ledger := lm.ledger(p)
	return ledger.WantListContains(k)
}

// MessageReceived performs book-keeping. Returns error if passed invalid
// arguments.
func (lm *LedgerManager) MessageReceived(p peer.Peer, m bsmsg.BitSwapMessage) error {
	lm.lock.Lock()
	defer lm.lock.Unlock()

	l := lm.ledger(p)
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

	l := lm.ledger(p)
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

	return lm.ledger(p).Accounting.BytesSent
}

func (lm *LedgerManager) NumBytesReceivedFrom(p peer.Peer) uint64 {
	lm.lock.RLock()
	defer lm.lock.RUnlock()

	return lm.ledger(p).Accounting.BytesRecv
}

// ledger lazily instantiates a ledger
func (lm *LedgerManager) ledger(p peer.Peer) *ledger {
	l, ok := lm.ledgerMap[peerKey(p.Key())]
	if !ok {
		l = newLedger(p)
		lm.ledgerMap[peerKey(p.Key())] = l
	}
	return l
}
