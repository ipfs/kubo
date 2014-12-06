package strategy

import (
	"time"

	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

// keySet is just a convenient alias for maps of keys, where we only care
// access/lookups.
type keySet map[u.Key]struct{}

func newLedger(p peer.Peer, strategy strategyFunc) *ledger {
	return &ledger{
		wantList: keySet{},
		Strategy: strategy,
		Partner:  p,
	}
}

// ledger stores the data exchange relationship between two peers.
// NOT threadsafe
type ledger struct {
	// Partner is the remote Peer.
	Partner peer.Peer

	// Accounting tracks bytes sent and recieved.
	Accounting debtRatio

	// firstExchnage is the time of the first data exchange.
	firstExchange time.Time

	// lastExchange is the time of the last data exchange.
	lastExchange time.Time

	// exchangeCount is the number of exchanges with this peer
	exchangeCount uint64

	// wantList is a (bounded, small) set of keys that Partner desires.
	wantList keySet

	Strategy strategyFunc
}

func (l *ledger) ShouldSend() bool {
	return l.Strategy(l)
}

func (l *ledger) SentBytes(n int) {
	l.exchangeCount++
	l.lastExchange = time.Now()
	l.Accounting.BytesSent += uint64(n)
}

func (l *ledger) ReceivedBytes(n int) {
	l.exchangeCount++
	l.lastExchange = time.Now()
	l.Accounting.BytesRecv += uint64(n)
}

// TODO: this needs to be different. We need timeouts.
func (l *ledger) Wants(k u.Key) {
	log.Debugf("peer %s wants %s", l.Partner, k)
	l.wantList[k] = struct{}{}
}

func (l *ledger) WantListContains(k u.Key) bool {
	_, ok := l.wantList[k]
	return ok
}

func (l *ledger) ExchangeCount() uint64 {
	return l.exchangeCount
}
