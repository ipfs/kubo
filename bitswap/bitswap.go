package bitswap

import (
  "github.com/jbenet/go-ipfs/peer"
)

// aliases

type Ledger struct {
  // todo
}

type BitSwap struct {
  Ledgers map[peer.PeerId]*Ledger
  HaveList map[multihash.Multihash]*block.Block
  WantList []*multihash.Multihash
}
