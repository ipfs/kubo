package blockservice

import (
	"bytes"
	"testing"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	blocks "github.com/jbenet/go-ipfs/blocks"
	u "github.com/jbenet/go-ipfs/util"
)

func TestBlocks(t *testing.T) {
	d := ds.NewMapDatastore()
	bs, err := NewBlockService(d, nil)
	if err != nil {
		t.Error("failed to construct block service", err)
		return
	}

	b := blocks.NewBlock([]byte("beep boop"))
	h := u.Hash([]byte("beep boop"))
	if !bytes.Equal(b.Multihash, h) {
		t.Error("Block Multihash and data multihash not equal")
	}

	if b.Key() != u.Key(h) {
		t.Error("Block key and data multihash key not equal")
	}

	k, err := bs.AddBlock(b)
	if err != nil {
		t.Error("failed to add block to BlockService", err)
		return
	}

	if k != b.Key() {
		t.Error("returned key is not equal to block key", err)
	}

	b2, err := bs.GetBlock(b.Key())
	if err != nil {
		t.Error("failed to retrieve block from BlockService", err)
		return
	}

	if b.Key() != b2.Key() {
		t.Error("Block keys not equal.")
	}

	if !bytes.Equal(b.Data, b2.Data) {
		t.Error("Block data is not equal.")
	}
}
