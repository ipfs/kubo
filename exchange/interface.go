// package exchange defines the IPFS exchange interface
package exchange

import (
	"context"
	"io"

	blocks "gx/ipfs/QmbJUay5h1HtzhJb5QQk2t26yCnJksHynvhcqp18utBPqG/go-block-format"

	cid "gx/ipfs/QmNw61A6sJoXMeP37mJRtQZdNhj5e3FdjoTN3v4FyE96Gk/go-cid"
)

// Any type that implements exchange.Interface may be used as an IPFS block
// exchange protocol.
type Interface interface { // type Exchanger interface
	// GetBlock returns the block associated with a given key.
	GetBlock(context.Context, *cid.Cid) (blocks.Block, error)

	GetBlocks(context.Context, []*cid.Cid) (<-chan blocks.Block, error)

	// TODO Should callers be concerned with whether the block was made
	// available on the network?
	HasBlock(blocks.Block) error

	IsOnline() bool

	io.Closer
}
