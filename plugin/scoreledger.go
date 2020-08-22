package plugin

import (
	"github.com/ipfs/go-bitswap/decision"
)

// With a PluginScoreLedger plugin it is possible to replace the default
// BitSwap decision logic with a different one.
type PluginScoreLedger interface {
	Plugin
	Ledger() (decision.ScoreLedger, error)
}
