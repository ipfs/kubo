// package exchange defines the IPFS Exchange interface
package exchange

import (
	"io"

	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
	blocks "github.com/ipfs/go-ipfs/blocks"
	key "github.com/ipfs/go-ipfs/blocks/key"
)

// Any type that implements exchange.Interface may be used as an IPFS block
// exchange protocol.
type Interface interface {
	// GetBlock returns the block associated with a given key.
	GetBlock(context.Context, key.Key) (*blocks.Block, error)

	GetBlocks(context.Context, []key.Key) (<-chan *blocks.Block, error)

	// TODO Should callers be concerned with whether the block was made
	// available on the network?
	HasBlock(*blocks.Block) error

	io.Closer
}
