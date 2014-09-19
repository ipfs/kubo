package bitswap

import (
	"time"

	blocks "github.com/jbenet/go-ipfs/blocks"
	u "github.com/jbenet/go-ipfs/util"
)

// Any type that implements exchange.Interface may be used as an IPFS block
// exchange protocol.
type Interface interface {

	// Block returns the block associated with a given key.
	// TODO(brian): pass a context instead of a timeout
	Block(k u.Key, timeout time.Duration) (*blocks.Block, error)

	// HasBlock asserts the existence of this block
	// TODO(brian): rename -> HasBlock
	// TODO(brian): accept a value, not a pointer
	// TODO(brian): remove error return value. Should callers be concerned with
	// whether the block was made available on the network?
	HasBlock(blocks.Block) error
}
