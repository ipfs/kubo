// package exchange defines the IPFS Exchange interface
package exchange

import (
	"io"

	blocks "github.com/ipfs/go-ipfs/blocks"
	key "gx/ipfs/QmYEoKZXHoAToWfhGF3vryhMn3WWhE1o2MasQ8uzY5iDi9/go-key"

	context "context"
)

// Any type that implements exchange.Interface may be used as an IPFS block
// exchange protocol.
type Interface interface { // type Exchanger interface
	// GetBlock returns the block associated with a given key.
	GetBlock(context.Context, key.Key) (blocks.Block, error)

	GetBlocks(context.Context, []key.Key) (<-chan blocks.Block, error)

	// TODO Should callers be concerned with whether the block was made
	// available on the network?
	HasBlock(blocks.Block) error

	IsOnline() bool

	io.Closer
}
