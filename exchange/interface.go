// package exchange defines the IPFS Exchange interface
package exchange

import (
	"io"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	blocks "github.com/jbenet/go-ipfs/struct/blocks"
	u "github.com/jbenet/go-ipfs/util"
)

// Any type that implements exchange.Interface may be used as an IPFS block
// exchange protocol.
type Interface interface {
	// GetBlock returns the block associated with a given key.
	GetBlock(context.Context, u.Key) (*blocks.Block, error)

	GetBlocks(context.Context, []u.Key) (<-chan *blocks.Block, error)

	// TODO Should callers be concerned with whether the block was made
	// available on the network?
	HasBlock(context.Context, *blocks.Block) error

	io.Closer
}
