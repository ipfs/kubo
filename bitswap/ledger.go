package bitswap

import (
	"math/rand"

	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"

	"time"
)

// Ledger stores the data exchange relationship between two peers.
type Ledger struct {

	// Partner is the ID of the remote Peer.
	Partner peer.ID

	// Accounting tracks bytes sent and recieved.
	Accounting debtRatio

	// FirstExchnage is the time of the first data exchange.
	FirstExchange *time.Time

	// LastExchange is the time of the last data exchange.
	LastExchange *time.Time

	// WantList is a (bounded, small) set of keys that Partner desires.
	WantList KeySet
}

// LedgerMap lists Ledgers by their Partner key.
type LedgerMap map[u.Key]*Ledger

func (l *Ledger) ShouldSend() bool {
	return rand.Float64() <= probabilitySend(l.Accounting.Value())
}

func (l *Ledger) SentBytes(n uint64) {
	l.Accounting.BytesSent += n
}

func (l *Ledger) ReceivedBytes(n uint64) {
	l.Accounting.BytesRecv += n
}
