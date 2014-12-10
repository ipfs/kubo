package strategy

import (
	"sync"

	bsmsg "github.com/jbenet/go-ipfs/exchange/bitswap/message"
	wl "github.com/jbenet/go-ipfs/exchange/bitswap/wantlist"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

// LedgerMap lists Ledgers by their Partner key.
type ledgerMap map[peerKey]*ledger

// FIXME share this externally
type peerKey u.Key

type LedgerSet struct {
	lock      sync.RWMutex
	ledgerMap ledgerMap
}

func NewLedgerSet() *LedgerSet {
	return &LedgerSet{
		ledgerMap: make(ledgerMap),
	}
}

// Returns a slice of Peers with whom the local node has active sessions
func (ls *LedgerSet) Peers() []peer.Peer {
	ls.lock.RLock()
	defer ls.lock.RUnlock()

	response := make([]peer.Peer, 0)
	for _, ledger := range ls.ledgerMap {
		response = append(response, ledger.Partner)
	}
	return response
}

// BlockIsWantedByPeer returns true if peer wants the block given by this
// key
func (ls *LedgerSet) BlockIsWantedByPeer(k u.Key, p peer.Peer) bool {
	ls.lock.RLock()
	defer ls.lock.RUnlock()

	ledger := ls.ledger(p)
	return ledger.WantListContains(k)
}

// MessageReceived performs book-keeping. Returns error if passed invalid
// arguments.
func (ls *LedgerSet) MessageReceived(p peer.Peer, m bsmsg.BitSwapMessage) error {
	ls.lock.Lock()
	defer ls.lock.Unlock()

	// TODO find a more elegant way to handle this check
	/*
		if p == nil {
			return errors.New("Strategy received nil peer")
		}
		if m == nil {
			return errors.New("Strategy received nil message")
		}
	*/
	l := ls.ledger(p)
	if m.Full() {
		l.wantList = wl.New()
	}
	for _, e := range m.Wantlist() {
		if e.Cancel {
			l.CancelWant(e.Key)
		} else {
			l.Wants(e.Key, e.Priority)
		}
	}
	for _, block := range m.Blocks() {
		// FIXME extract blocks.NumBytes(block) or block.NumBytes() method
		l.ReceivedBytes(len(block.Data))
	}
	return nil
}

// TODO add contents of m.WantList() to my local wantlist? NB: could introduce
// race conditions where I send a message, but MessageSent gets handled after
// MessageReceived. The information in the local wantlist could become
// inconsistent. Would need to ensure that Sends and acknowledgement of the
// send happen atomically

func (ls *LedgerSet) MessageSent(p peer.Peer, m bsmsg.BitSwapMessage) error {
	ls.lock.Lock()
	defer ls.lock.Unlock()

	l := ls.ledger(p)
	for _, block := range m.Blocks() {
		l.SentBytes(len(block.Data))
		l.wantList.Remove(block.Key())
	}

	return nil
}

func (ls *LedgerSet) NumBytesSentTo(p peer.Peer) uint64 {
	ls.lock.RLock()
	defer ls.lock.RUnlock()

	return ls.ledger(p).Accounting.BytesSent
}

func (ls *LedgerSet) NumBytesReceivedFrom(p peer.Peer) uint64 {
	ls.lock.RLock()
	defer ls.lock.RUnlock()

	return ls.ledger(p).Accounting.BytesRecv
}

// ledger lazily instantiates a ledger
func (ls *LedgerSet) ledger(p peer.Peer) *ledger {
	l, ok := ls.ledgerMap[peerKey(p.Key())]
	if !ok {
		l = newLedger(p)
		ls.ledgerMap[peerKey(p.Key())] = l
	}
	return l
}
