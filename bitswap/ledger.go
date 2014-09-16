package bitswap

import (
	"sync"
	"time"

	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

// Ledger stores the data exchange relationship between two peers.
type Ledger struct {
	lock sync.RWMutex

	// Partner is the remote Peer.
	Partner *peer.Peer

	// Accounting tracks bytes sent and recieved.
	Accounting debtRatio

	// firstExchnage is the time of the first data exchange.
	firstExchange time.Time

	// lastExchange is the time of the last data exchange.
	lastExchange time.Time

	// exchangeCount is the number of exchanges with this peer
	exchangeCount uint64

	// wantList is a (bounded, small) set of keys that Partner desires.
	wantList KeySet

	Strategy strategyFunc
}

// LedgerMap lists Ledgers by their Partner key.
type LedgerMap map[u.Key]*Ledger

func (l *Ledger) ShouldSend() bool {
	l.lock.Lock()
	defer l.lock.Unlock()

	return l.Strategy(l)
}

func (l *Ledger) SentBytes(n int) {
	l.lock.Lock()
	defer l.lock.Unlock()

	l.exchangeCount++
	l.lastExchange = time.Now()
	l.Accounting.BytesSent += uint64(n)
}

func (l *Ledger) ReceivedBytes(n int) {
	l.lock.Lock()
	defer l.lock.Unlock()

	l.exchangeCount++
	l.lastExchange = time.Now()
	l.Accounting.BytesRecv += uint64(n)
}

// TODO: this needs to be different. We need timeouts.
func (l *Ledger) Wants(k u.Key) {
	l.lock.Lock()
	defer l.lock.Unlock()

	l.wantList[k] = struct{}{}
}

func (l *Ledger) WantListContains(k u.Key) bool {
	l.lock.RLock()
	defer l.lock.RUnlock()

	_, ok := l.wantList[k]
	return ok
}

func (l *Ledger) ExchangeCount() uint64 {
	l.lock.RLock()
	defer l.lock.RUnlock()
	return l.exchangeCount
}
