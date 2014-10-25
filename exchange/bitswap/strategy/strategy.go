package strategy

import (
	"errors"
	"sync"

	bsmsg "github.com/jbenet/go-ipfs/exchange/bitswap/message"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

// TODO niceness should be on a per-peer basis. Use-case: Certain peers are
// "trusted" and/or controlled by a single human user. The user may want for
// these peers to exchange data freely
func New(nice bool) Strategy {
	var stratFunc strategyFunc
	if nice {
		stratFunc = yesManStrategy
	} else {
		stratFunc = standardStrategy
	}
	return &strategist{
		ledgerMap:    ledgerMap{},
		strategyFunc: stratFunc,
	}
}

type strategist struct {
	lock sync.RWMutex
	ledgerMap
	strategyFunc
}

// LedgerMap lists Ledgers by their Partner key.
type ledgerMap map[peerKey]*ledger

// FIXME share this externally
type peerKey u.Key

// Peers returns a list of peers
func (s *strategist) Peers() []peer.Peer {
	s.lock.RLock()
	defer s.lock.RUnlock()

	response := make([]peer.Peer, 0)
	for _, ledger := range s.ledgerMap {
		response = append(response, ledger.Partner)
	}
	return response
}

func (s *strategist) BlockIsWantedByPeer(k u.Key, p peer.Peer) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	ledger := s.ledger(p)
	return ledger.WantListContains(k)
}

func (s *strategist) ShouldSendBlockToPeer(k u.Key, p peer.Peer) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	ledger := s.ledger(p)
	return ledger.ShouldSend()
}

func (s *strategist) Seed(int64) {
	s.lock.Lock()
	defer s.lock.Unlock()

	// TODO
}

func (s *strategist) MessageReceived(p peer.Peer, m bsmsg.BitSwapMessage) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	// TODO find a more elegant way to handle this check
	if p == nil {
		return errors.New("Strategy received nil peer")
	}
	if m == nil {
		return errors.New("Strategy received nil message")
	}
	l := s.ledger(p)
	for _, key := range m.Wantlist() {
		l.Wants(key)
	}
	for _, block := range m.Blocks() {
		// FIXME extract blocks.NumBytes(block) or block.NumBytes() method
		l.ReceivedBytes(len(block.Data))
	}
	return errors.New("TODO")
}

// TODO add contents of m.WantList() to my local wantlist? NB: could introduce
// race conditions where I send a message, but MessageSent gets handled after
// MessageReceived. The information in the local wantlist could become
// inconsistent. Would need to ensure that Sends and acknowledgement of the
// send happen atomically

func (s *strategist) MessageSent(p peer.Peer, m bsmsg.BitSwapMessage) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	l := s.ledger(p)
	for _, block := range m.Blocks() {
		l.SentBytes(len(block.Data))
	}

	// TODO remove these blocks from peer's want list

	return nil
}

func (s *strategist) NumBytesSentTo(p peer.Peer) uint64 {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.ledger(p).Accounting.BytesSent
}

func (s *strategist) NumBytesReceivedFrom(p peer.Peer) uint64 {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.ledger(p).Accounting.BytesRecv
}

// ledger lazily instantiates a ledger
func (s *strategist) ledger(p peer.Peer) *ledger {
	l, ok := s.ledgerMap[peerKey(p.Key())]
	if !ok {
		l = newLedger(p, s.strategyFunc)
		s.ledgerMap[peerKey(p.Key())] = l
	}
	return l
}
