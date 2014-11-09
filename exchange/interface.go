// package exchange defines the IPFS Exchange interface
package exchange

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	blocks "github.com/jbenet/go-ipfs/blocks"
	u "github.com/jbenet/go-ipfs/util"
)

// Any type that implements exchange.Interface may be used as an IPFS block
// exchange protocol.
type Interface interface {

	// Block returns the block associated with a given key.
	Block(context.Context, u.Key) (*blocks.Block, error)

	// TODO Should callers be concerned with whether the block was made
	// available on the network?
	HasBlock(context.Context, blocks.Block) error
}
