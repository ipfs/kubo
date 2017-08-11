package gc

import (
	"testing"

	"github.com/ipfs/go-ipfs/blocks/blocksutil"

	cid "gx/ipfs/QmNp85zy9RLrQ5oQD4hPyS39ezrrXpcaa7R4Y9kxdWQLLQ/go-cid"
)

func TestInsertWhite(t *testing.T) {
	tri := newTriset()
	blkgen := blocksutil.NewBlockGenerator()

	whites := make([]*cid.Cid, 1000)
	for i := range whites {
		blk := blkgen.Next()
		whites[i] = blk.Cid()

		tri.InsertWhite(blk.Cid())
	}

	for _, v := range whites {
		if tri.colmap[v.KeyString()].getColor() != tri.white {
			t.Errorf("cid %s should be white and is not %s", v, tri.colmap[v.KeyString()])
		}
	}

}
