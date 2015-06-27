// package exchange defines the IPFS Exchange interface
// this is here to avoid vendoring IPFS for now.
package exchange

import (
	"io"

	blocks "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-blocks"
	key "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-blocks/key"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
)

// Any type that implements exchange.Interface may be used as an IPFS block
// exchange protocol.
type Interface interface {
	// GetBlock returns the block associated with a given key.
	GetBlock(context.Context, key.Key) (*blocks.Block, error)

	GetBlocks(context.Context, []key.Key) (<-chan *blocks.Block, error)

	// TODO Should callers be concerned with whether the block was made
	// available on the network?
	HasBlock(context.Context, *blocks.Block) error

	io.Closer
}
