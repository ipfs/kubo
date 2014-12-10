package strategy

import (
	blocks "github.com/jbenet/go-ipfs/blocks"
	bstore "github.com/jbenet/go-ipfs/blocks/blockstore"
	wl "github.com/jbenet/go-ipfs/exchange/bitswap/wantlist"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

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
		strategyFunc: stratFunc,
	}
}

type strategist struct {
	strategyFunc
}

type Task struct {
	Peer   peer.Peer
	Blocks []*blocks.Block
}

func (s *strategist) GetTasks(bandwidth int, ledgers *LedgerSet, bs bstore.Blockstore) ([]*Task, error) {
	var tasks []*Task

	ledgers.lock.RLock()
	var partners []peer.Peer
	for _, ledger := range ledgers.ledgerMap {
		if s.strategyFunc(ledger) {
			partners = append(partners, ledger.Partner)
		}
	}
	ledgers.lock.RUnlock()
	if len(partners) == 0 {
		return nil, nil
	}

	bandwidthPerPeer := bandwidth / len(partners)
	for _, p := range partners {
		blksForPeer, err := s.getSendableBlocks(ledgers.ledger(p).wantList, bs, bandwidthPerPeer)
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
		if err == bstore.ErrNotFound {
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

func test() {}
func (s *strategist) Seed(int64) {
	// TODO
}
