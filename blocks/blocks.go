package blocks

import (
	"errors"
	"fmt"

	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
	u "github.com/jbenet/go-ipfs/util"
)

// Block is a singular block of data in ipfs
type Block struct {
	Multihash mh.Multihash
	Data      []byte
}

// NewBlock creates a Block object from opaque data. It will hash the data.
func NewBlock(data []byte) *Block {
	return &Block{Data: data, Multihash: u.Hash(data)}
}

// NewBlockWithHash creates a new block when the hash of the data
// is already known, this is used to save time in situations where
// we are able to be confident that the data is correct
func NewBlockWithHash(data []byte, h mh.Multihash) (*Block, error) {
	if u.Debug {
		chk := u.Hash(data)
		if string(chk) != string(h) {
			return nil, errors.New("Data did not match given hash!")
		}
	}
	return &Block{Data: data, Multihash: h}, nil
}

// Key returns the block's Multihash as a Key value.
func (b *Block) Key() u.Key {
	return u.Key(b.Multihash)
}

func (b *Block) String() string {
	return fmt.Sprintf("[Block %s]", b.Key())
}
