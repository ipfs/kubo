package bitswap

import (
  "time"
  mh "github.com/jbenet/go-multihash"
  blocks "github.com/jbenet/go-ipfs/blocks"
  u "github.com/jbenet/go-ipfs/util"
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
  Ledgers map[u.Key]*Ledger  // key is peer.ID
  HaveList map[u.Key]*blocks.Block // key is multihash
  WantList []*mh.Multihash
}
