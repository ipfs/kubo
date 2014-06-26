package block

import (
  "github.com/jbenet/go-multihash"
)

type Block struct {
  Multihash []byte
  Data []byte
}
