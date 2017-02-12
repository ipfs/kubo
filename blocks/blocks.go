// package blocks contains the lowest level of IPFS data structures,
// the raw block with a checksum.
package blocks

import (
	"errors"
	"fmt"

	mh "gx/ipfs/QmYDds3421prZgqKbLpEK7T9Aa2eVdQ7o3YarX1LVLdP2J/go-multihash"
	u "gx/ipfs/Qmb912gdngC1UWwTkhuW8knyRbcWeu5kqkxBpveLmW8bSr/go-ipfs-util"
	cid "gx/ipfs/QmcTcsTvfaeEBRFo1TkFgT8sRmgi1n1LTZpecfVP8fzpGD/go-cid"
)

var ErrWrongHash = errors.New("data did not match given hash!")

type Block interface {
	RawData() []byte
	Cid() *cid.Cid
	String() string
	Loggable() map[string]interface{}
}

// Block is a singular block of data in ipfs
type BasicBlock struct {
	cid  *cid.Cid
	data []byte
}

// NewBlock creates a Block object from opaque data. It will hash the data.
func NewBlock(data []byte) *BasicBlock {
	// TODO: fix assumptions
	return &BasicBlock{data: data, cid: cid.NewCidV0(u.Hash(data))}
}

// NewBlockWithHash creates a new block when the hash of the data
// is already known, this is used to save time in situations where
// we are able to be confident that the data is correct
func NewBlockWithCid(data []byte, c *cid.Cid) (*BasicBlock, error) {
	if u.Debug {
		chkc, err := c.Prefix().Sum(data)
		if err != nil {
			return nil, err
		}

		if !chkc.Equals(c) {
			return nil, ErrWrongHash
		}
	}
	return &BasicBlock{data: data, cid: c}, nil
}

func (b *BasicBlock) Multihash() mh.Multihash {
	return b.cid.Hash()
}

func (b *BasicBlock) RawData() []byte {
	return b.data
}

func (b *BasicBlock) Cid() *cid.Cid {
	return b.cid
}

func (b *BasicBlock) String() string {
	return fmt.Sprintf("[Block %s]", b.Cid())
}

func (b *BasicBlock) Loggable() map[string]interface{} {
	return map[string]interface{}{
		"block": b.Cid().String(),
	}
}
