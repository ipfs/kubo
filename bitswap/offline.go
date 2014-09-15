package bitswap

import (
	"errors"
	"time"

	blocks "github.com/jbenet/go-ipfs/blocks"
	u "github.com/jbenet/go-ipfs/util"
)

func NewOfflineExchange() Exchange {
	return &offlineExchange{}
}

// offlineExchange implements the Exchange interface but doesn't return blocks.
// For use in offline mode.
type offlineExchange struct {
}

// Block returns nil to signal that a block could not be retrieved for the
// given key.
// NB: This function may return before the timeout expires.
func (_ *offlineExchange) Block(k u.Key, timeout time.Duration) (*blocks.Block, error) {
	return nil, errors.New("Block unavailable. Operating in offline mode")
}

// HasBlock always returns nil.
func (_ *offlineExchange) HasBlock(blocks.Block) error {
	return nil
}
