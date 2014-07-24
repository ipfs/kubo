package bitswap

import (
  "github.com/jbenet/go-ipfs/blocks"
  "github.com/jbenet/go-multihash"
)

// aliases

type Ledger struct {
  // todo
}

type BitSwap struct {
  Ledgers  map[string]*Ledger
  HaveList map[string]*blocks.Block
  WantList []*multihash.Multihash
}
