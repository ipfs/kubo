package blockstore

import (
	"bytes"
	"fmt"
	"testing"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	ds_sync "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	blocks "github.com/jbenet/go-ipfs/blocks"
	u "github.com/jbenet/go-ipfs/util"
)

// TODO(brian): TestGetReturnsNil

func TestGetWhenKeyNotPresent(t *testing.T) {
	bs := NewBlockstore(ds_sync.MutexWrap(ds.NewMapDatastore()))
	_, err := bs.Get(u.Key("not present"))

	if err != nil {
		t.Log("As expected, block is not present")
		return
	}
	t.Fail()
}

func TestPutThenGetBlock(t *testing.T) {
	bs := NewBlockstore(ds_sync.MutexWrap(ds.NewMapDatastore()))
	block := blocks.NewBlock([]byte("some data"))

	err := bs.Put(block)
	if err != nil {
		t.Fatal(err)
	}

	blockFromBlockstore, err := bs.Get(block.Key())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(block.Data, blockFromBlockstore.Data) {
		t.Fail()
	}
}

func TestAllKeys(t *testing.T) {
	bs := NewBlockstore(ds_sync.MutexWrap(ds.NewMapDatastore()))
	N := 100

	keys := make([]u.Key, N)
	for i := 0; i < N; i++ {
		block := blocks.NewBlock([]byte(fmt.Sprintf("some data %d", i)))
		err := bs.Put(block)
		if err != nil {
			t.Fatal(err)
		}
		keys[i] = block.Key()
	}

	keys2, err := bs.AllKeys(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	// for _, k2 := range keys2 {
	// 	t.Log("found ", k2.Pretty())
	// }

	expectMatches(t, keys, keys2)

	keys3, err := bs.AllKeys(N/3, N/3)
	if err != nil {
		t.Fatal(err)
	}
	for _, k3 := range keys3 {
		t.Log("found ", k3.Pretty())
	}
	if len(keys3) != N/3 {
		t.Errorf("keys3 should be: %d != %d", N/3, len(keys3))
	}

}

func TestValueTypeMismatch(t *testing.T) {
	block := blocks.NewBlock([]byte("some data"))

	datastore := ds.NewMapDatastore()
	k := BlockPrefix.Child(block.Key().DsKey())
	datastore.Put(k, "data that isn't a block!")

	blockstore := NewBlockstore(ds_sync.MutexWrap(datastore))

	_, err := blockstore.Get(block.Key())
	if err != ValueTypeMismatch {
		t.Fatal(err)
	}
}

func expectMatches(t *testing.T, expect, actual []u.Key) {

	if len(expect) != len(actual) {
		t.Errorf("expect and actual differ: %d != %d", len(expect), len(actual))
	}
	for _, ek := range expect {
		found := false
		for _, ak := range actual {
			if ek == ak {
				found = true
			}
		}
		if !found {
			t.Error("expected key not found: ", ek)
		}
	}
}
