package blocks

import (
  "github.com/jbenet/go-ipfs/bitswap"
  "github.com/jbenet/go-ipfs/storage"
)

// Blocks is the ipfs blocks service. It is the way
// to retrieve blocks by the higher level ipfs modules

type BlockService struct {
  Local *storage.Storage
  Remote *bitswap.BitSwap
}
