package blockstore

import (
	"bytes"
	"testing"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"
	u "github.com/jbenet/go-ipfs/util"
	testutil "github.com/jbenet/go-ipfs/util/testutil"
)

// TODO(brian): TestGetReturnsNil

func TestGetWhenKeyNotPresent(t *testing.T) {
	bs := NewBlockstore(ds.NewMapDatastore())
	_, err := bs.Get(u.Key("not present"))

	if err != nil {
		t.Log("As expected, block is not present")
		return
	}
	t.Fail()
}

func TestPutThenGetBlock(t *testing.T) {
	bs := NewBlockstore(ds.NewMapDatastore())
	block := testutil.NewBlockOrFail(t, "some data")

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

func TestValueTypeMismatch(t *testing.T) {
	block := testutil.NewBlockOrFail(t, "some data")

	datastore := ds.NewMapDatastore()
	datastore.Put(block.Key().DsKey(), "data that isn't a block!")

	blockstore := NewBlockstore(datastore)

	_, err := blockstore.Get(block.Key())
	if err != ValueTypeMismatch {
		t.Fatal(err)
	}
}
