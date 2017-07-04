// Package blocksutil provides utility functions for working
// with Blocks.
package blocksutil

import "gx/ipfs/QmXxGS5QsUxpR3iqL5DjmsYPHR1Yz74siRQ4ChJqWFosMh/go-block-format"

// NewBlockGenerator returns an object capable of
// producing blocks.
func NewBlockGenerator() BlockGenerator {
	return BlockGenerator{}
}

// BlockGenerator generates BasicBlocks on demand.
// For each instace of BlockGenerator,
// each new block is different from the previous,
// although two different instances will produce the same.
type BlockGenerator struct {
	seq int
}

// Next generates a new BasicBlock.
func (bg *BlockGenerator) Next() *blocks.BasicBlock {
	bg.seq++
	return blocks.NewBlock([]byte(string(bg.seq)))
}

// Blocks generates as many BasicBlocks as specified by n.
func (bg *BlockGenerator) Blocks(n int) []*blocks.BasicBlock {
	blocks := make([]*blocks.BasicBlock, 0)
	for i := 0; i < n; i++ {
		b := bg.Next()
		blocks = append(blocks, b)
	}
	return blocks
}
