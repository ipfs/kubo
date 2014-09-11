package bitswap

import (
	"time"

	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

// Ledger stores the data exchange relationship between two peers.
type Ledger struct {

	// Partner is the remote Peer.
	Partner *peer.Peer

	// Accounting tracks bytes sent and recieved.
	Accounting debtRatio

	// FirstExchnage is the time of the first data exchange.
	FirstExchange time.Time

	// LastExchange is the time of the last data exchange.
	LastExchange time.Time

	// Number of exchanges with this peer
	ExchangeCount uint64

	// WantList is a (bounded, small) set of keys that Partner desires.
	WantList KeySet

	Strategy StrategyFunc
}

// LedgerMap lists Ledgers by their Partner key.
type LedgerMap map[u.Key]*Ledger

func (l *Ledger) ShouldSend() bool {
	return l.Strategy(l)
}

func (l *Ledger) SentBytes(n int) {
	l.ExchangeCount++
	l.LastExchange = time.Now()
	l.Accounting.BytesSent += uint64(n)
}

func (l *Ledger) ReceivedBytes(n int) {
	l.ExchangeCount++
	l.LastExchange = time.Now()
	l.Accounting.BytesRecv += uint64(n)
}
