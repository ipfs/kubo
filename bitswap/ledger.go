package bitswap

import (
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"

	"time"
)

// Ledger stores the data exchange relationship between two peers.
type Ledger struct {

	// Partner is the ID of the remote Peer.
	Partner peer.ID

	// BytesSent counts the number of bytes the local peer sent to Partner
	BytesSent uint64

	// BytesReceived counts the number of bytes local peer received from Partner
	BytesReceived uint64

	// FirstExchnage is the time of the first data exchange.
	FirstExchange *time.Time

	// LastExchange is the time of the last data exchange.
	LastExchange *time.Time

	// WantList is a (bounded, small) set of keys that Partner desires.
	WantList KeySet
}

// LedgerMap lists Ledgers by their Partner key.
type LedgerMap map[u.Key]*Ledger
