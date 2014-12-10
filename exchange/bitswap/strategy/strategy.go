package strategy

import (
	"errors"
	"sync"
	"time"

	blocks "github.com/jbenet/go-ipfs/blocks"
	bstore "github.com/jbenet/go-ipfs/blocks/blockstore"
	bsmsg "github.com/jbenet/go-ipfs/exchange/bitswap/message"
	wl "github.com/jbenet/go-ipfs/exchange/bitswap/wantlist"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

const resendTimeoutPeriod = time.Minute

var log = u.Logger("strategy")

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

	// Dont resend blocks within a certain time period
	t, ok := ledger.sentToPeer[k]
	if ok && t.Add(resendTimeoutPeriod).After(time.Now()) {
		return false
	}

	return ledger.ShouldSend()
}

type Task struct {
	Peer   peer.Peer
	Blocks []*blocks.Block
}

func (s *strategist) GetAllocation(bandwidth int, bs bstore.Blockstore) ([]*Task, error) {
	var tasks []*Task

	s.lock.RLock()
	defer s.lock.RUnlock()
	var partners []peer.Peer
	for _, ledger := range s.ledgerMap {
		if ledger.ShouldSend() {
			partners = append(partners, ledger.Partner)
		}
	}
	if len(partners) == 0 {
		return nil, nil
	}

	bandwidthPerPeer := bandwidth / len(partners)
	for _, p := range partners {
		blksForPeer, err := s.getSendableBlocks(s.ledger(p).wantList, bs, bandwidthPerPeer)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, &Task{
			Peer:   p,
			Blocks: blksForPeer,
		})
	}

	return tasks, nil
}

func (s *strategist) getSendableBlocks(wantlist *wl.Wantlist, bs bstore.Blockstore, bw int) ([]*blocks.Block, error) {
	var outblocks []*blocks.Block
	for _, e := range wantlist.Entries() {
		block, err := bs.Get(e.Value)
		if err == u.ErrNotFound {
			continue
		}
		if err != nil {
			return nil, err
		}
		outblocks = append(outblocks, block)
		bw -= len(block.Data)
		if bw <= 0 {
			break
		}
	}
	return outblocks, nil
}

func (s *strategist) BlockSentToPeer(k u.Key, p peer.Peer) {
	s.lock.Lock()
	defer s.lock.Unlock()

	ledger := s.ledger(p)
	ledger.sentToPeer[k] = time.Now()
}

func (s *strategist) Seed(int64) {
	s.lock.Lock()
	defer s.lock.Unlock()

	// TODO
}

// MessageReceived performs book-keeping. Returns error if passed invalid
// arguments.
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
	if m.Full() {
		l.wantList = wl.NewWantlist()
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

func (s *strategist) GetBatchSize() int {
	return 10
}

func (s *strategist) GetRebroadcastDelay() time.Duration {
	return time.Second * 10
}
