// package exchange defines the IPFS exchange interface
package exchange

import (
	"context"
	"io"

	blocks "gx/ipfs/QmP5Lp2S1BzuBUWKA8Y25Ajtatqmv1f5cdzxAN4vFoebcf/go-block-format"

	cid "gx/ipfs/QmetUj3ZqWMDVeFMRq7S9PdMauXCwBZuggfHqoS4MPt1Vy/go-cid"
)

// Any type that implements exchange.Interface may be used as an IPFS block
// exchange protocol.
type Interface interface { // type Exchanger interface
	Fetcher

	// TODO Should callers be concerned with whether the block was made
	// available on the network?
	HasBlock(blocks.Block) error

	IsOnline() bool

	io.Closer
}

// Fetcher is an object that can be used to retrieve blocks
type Fetcher interface {
	// GetBlock returns the block associated with a given key.
	GetBlock(context.Context, *cid.Cid) (blocks.Block, error)
	GetBlocks(context.Context, []*cid.Cid) (<-chan blocks.Block, error)
}
