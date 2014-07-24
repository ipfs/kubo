package bitswap

import (
	"github.com/jbenet/go-ipfs/blocks"
	mh "github.com/jbenet/go-multihash"

	"time"
)

// aliases

type Ledger struct {
	Owner mh.Multihash

	Partner mh.Multihash

	BytesSent uint64

	BytesRecv uint64

	Timestamp *time.Time
}

type BitSwap struct {
	Ledgers  map[string]*Ledger
	HaveList map[string]*blocks.Block
	WantList []*multihash.Multihash
}
