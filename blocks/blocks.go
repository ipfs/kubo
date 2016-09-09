// package blocks contains the lowest level of ipfs data structures,
// the raw block with a checksum.
package blocks

import (
	"errors"
	"fmt"

	key "gx/ipfs/Qmce4Y4zg3sYr7xKM5UueS67vhNni6EeWgCRnb7MbLJMew/go-key"

	mh "gx/ipfs/QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku/go-multihash"
	u "gx/ipfs/QmZNVWh8LLjAavuQ2JXuFmuYH3C11xo988vSgp7UQrTRj1/go-ipfs-util"
	cid "gx/ipfs/QmfSc2xehWmWLnwwYR91Y8QF4xdASypTFVknutoKQS3GHp/go-cid"
)

var ErrWrongHash = errors.New("data did not match given hash!")

type Block interface {
	Multihash() mh.Multihash
	RawData() []byte
	Key() key.Key
	String() string
	Loggable() map[string]interface{}
}

// Block is a singular block of data in ipfs
type BasicBlock struct {
	multihash mh.Multihash
	data      []byte
}

// NewBlock creates a Block object from opaque data. It will hash the data.
func NewBlock(data []byte) *BasicBlock {
	return &BasicBlock{data: data, multihash: u.Hash(data)}
}

// NewBlockWithHash creates a new block when the hash of the data
// is already known, this is used to save time in situations where
// we are able to be confident that the data is correct
func NewBlockWithHash(data []byte, h mh.Multihash) (*BasicBlock, error) {
	if u.Debug {
		chk := u.Hash(data)
		if string(chk) != string(h) {
			return nil, ErrWrongHash
		}
	}
	return &BasicBlock{data: data, multihash: h}, nil
}

func (b *BasicBlock) Multihash() mh.Multihash {
	return b.multihash
}

func (b *BasicBlock) RawData() []byte {
	return b.data
}

func (b *BasicBlock) Cid() *cid.Cid {
	return cid.NewCidV0(b.multihash)
}

// Key returns the block's Multihash as a Key value.
func (b *BasicBlock) Key() key.Key {
	return key.Key(b.multihash)
}

func (b *BasicBlock) String() string {
	return fmt.Sprintf("[Block %s]", b.Key())
}

func (b *BasicBlock) Loggable() map[string]interface{} {
	return map[string]interface{}{
		"block": b.Key().String(),
	}
}
