package bitswap

import (
  peer "github.com/jbenet/go-ipfs/peer"
)

// aliases

type Ledger struct {
  // todo
}

type BitSwap struct {
  Ledgers map[peer.ID]*Ledger
  HaveList map[multihash.Multihash]*block.Block
  WantList []*multihash.Multihash
}
