package blocksutil

import "github.com/ipfs/go-ipfs/blocks"

func NewBlockGenerator() BlockGenerator {
	return BlockGenerator{}
}

type BlockGenerator struct {
	seq int
}

func (bg *BlockGenerator) Next() *blocks.BasicBlock {
	bg.seq++
	return blocks.NewBlock([]byte(string(bg.seq)))
}

func (bg *BlockGenerator) Blocks(n int) []*blocks.BasicBlock {
	blocks := make([]*blocks.BasicBlock, 0)
	for i := 0; i < n; i++ {
		b := bg.Next()
		blocks = append(blocks, b)
	}
	return blocks
}
