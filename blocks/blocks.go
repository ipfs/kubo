// package blocks contains the lowest level of ipfs data structures,
// the raw block with a checksum.
package blocks

import (
	"errors"
	"fmt"

	key "gx/ipfs/QmYEoKZXHoAToWfhGF3vryhMn3WWhE1o2MasQ8uzY5iDi9/go-key"

	mh "gx/ipfs/QmYDds3421prZgqKbLpEK7T9Aa2eVdQ7o3YarX1LVLdP2J/go-multihash"
	cid "gx/ipfs/QmakyCk6Vnn16WEKjbkxieZmM2YLTzkFWizbmGowoYPjro/go-cid"
	u "gx/ipfs/Qmb912gdngC1UWwTkhuW8knyRbcWeu5kqkxBpveLmW8bSr/go-ipfs-util"
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
