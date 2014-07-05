package blocks

import (
	"bytes"
	"fmt"
	ds "github.com/jbenet/datastore.go"
	u "github.com/jbenet/go-ipfs/util"
	"testing"
)

func TestBlocks(t *testing.T) {

	d := ds.NewMapDatastore()
	bs, err := NewBlockService(d)
	if err != nil {
		t.Error("failed to construct block service", err)
		return
	}

	b, err := NewBlock([]byte("beep boop"))
	if err != nil {
		t.Error("failed to construct block", err)
		return
	}

	h, err := u.Hash([]byte("beep boop"))
	if err != nil {
		t.Error("failed to hash data", err)
		return
	}

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

	fmt.Printf("key: %s\n", b.Key())
	fmt.Printf("data: %v\n", b.Data)
}
