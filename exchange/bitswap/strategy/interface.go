package strategy

import (
	bstore "github.com/jbenet/go-ipfs/blocks/blockstore"
)

type Strategy interface {
	// Seed initializes the decider to a deterministic state
	Seed(int64)

	GetTasks(bandwidth int, ledgers *LedgerManager, bs bstore.Blockstore) ([]*Task, error)
}
