package decision

import (
	"time"

	key "github.com/ipfs/go-ipfs/blocks/key"
	wl "github.com/ipfs/go-ipfs/exchange/bitswap/wantlist"
	peer "gx/ipfs/QmZwZjMVGss5rqYsJVGy18gNbkTJffFyq2x1uJ4e4p3ZAt/go-libp2p-peer"
)

// keySet is just a convenient alias for maps of keys, where we only care
// access/lookups.
type keySet map[key.Key]struct{}

func newLedger(p peer.ID) *ledger {
	return &ledger{
		wantList:   wl.New(),
		Partner:    p,
		sentToPeer: make(map[key.Key]time.Time),
	}
}

// ledger stores the data exchange relationship between two peers.
// NOT threadsafe
type ledger struct {
	// Partner is the remote Peer.
	Partner peer.ID

	// Accounting tracks bytes sent and recieved.
	Accounting debtRatio

	// firstExchnage is the time of the first data exchange.
	firstExchange time.Time

	// lastExchange is the time of the last data exchange.
	lastExchange time.Time

	// exchangeCount is the number of exchanges with this peer
	exchangeCount uint64

	// wantList is a (bounded, small) set of keys that Partner desires.
	wantList *wl.Wantlist

	// sentToPeer is a set of keys to ensure we dont send duplicate blocks
	// to a given peer
	sentToPeer map[key.Key]time.Time
}

type debtRatio struct {
	BytesSent uint64
	BytesRecv uint64
}

func (dr *debtRatio) Value() float64 {
	return float64(dr.BytesSent) / float64(dr.BytesRecv+1)
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

func (l *ledger) Wants(k key.Key, priority int) {
	log.Debugf("peer %s wants %s", l.Partner, k)
	l.wantList.Add(k, priority)
}

func (l *ledger) CancelWant(k key.Key) {
	l.wantList.Remove(k)
}

func (l *ledger) WantListContains(k key.Key) (wl.Entry, bool) {
	return l.wantList.Contains(k)
}

func (l *ledger) ExchangeCount() uint64 {
	return l.exchangeCount
}
