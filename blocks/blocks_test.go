package blocks

import (
	"bytes"
	"testing"

	mh "gx/ipfs/QmYDds3421prZgqKbLpEK7T9Aa2eVdQ7o3YarX1LVLdP2J/go-multihash"
	u "gx/ipfs/Qmb912gdngC1UWwTkhuW8knyRbcWeu5kqkxBpveLmW8bSr/go-ipfs-util"
)

func TestBlocksBasic(t *testing.T) {

	// Test empty data
	empty := []byte{}
	NewBlock(empty)

	// Test nil case
	NewBlock(nil)

	// Test some data
	NewBlock([]byte("Hello world!"))
}

func TestData(t *testing.T) {
	data := []byte("some data")
	block := NewBlock(data)

	if !bytes.Equal(block.RawData(), data) {
		t.Error("data is wrong")
	}
}

func TestHash(t *testing.T) {
	data := []byte("some other data")
	block := NewBlock(data)

	hash, err := mh.Sum(data, mh.SHA2_256, -1)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(block.Multihash(), hash) {
		t.Error("wrong multihash")
	}
}

func TestKey(t *testing.T) {
	data := []byte("yet another data")
	block := NewBlock(data)
	key := block.Key()

	if !bytes.Equal(block.Multihash(), key.ToMultihash()) {
		t.Error("key contains wrong data")
	}
}

func TestManualHash(t *testing.T) {
	oldDebugState := u.Debug
	defer (func() {
		u.Debug = oldDebugState
	})()

	data := []byte("I can't figure out more names .. data")
	hash, err := mh.Sum(data, mh.SHA2_256, -1)
	if err != nil {
		t.Fatal(err)
	}

	u.Debug = false
	block, err := NewBlockWithHash(data, hash)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(block.Multihash(), hash) {
		t.Error("wrong multihash")
	}

	data[5] = byte((uint32(data[5]) + 5) % 256) // Transfrom hash to be different
	block, err = NewBlockWithHash(data, hash)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(block.Multihash(), hash) {
		t.Error("wrong multihash")
	}

	u.Debug = true

	block, err = NewBlockWithHash(data, hash)
	if err != ErrWrongHash {
		t.Fatal(err)
	}

}
