package dstest

import (
	"bytes"
	"encoding/base32"
	"testing"

	rand "github.com/dustin/randbo"
	dstore "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
)

func RunBatchTest(t *testing.T, ds dstore.Batching) {
	batch, err := ds.Batch()
	if err != nil {
		t.Fatal(err)
	}

	r := rand.New()
	var blocks [][]byte
	var keys []dstore.Key
	for i := 0; i < 20; i++ {
		blk := make([]byte, 256*1024)
		r.Read(blk)
		blocks = append(blocks, blk)

		key := dstore.NewKey(base32.StdEncoding.EncodeToString(blk[:8]))
		keys = append(keys, key)

		err := batch.Put(key, blk)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Ensure they are not in the datastore before comitting
	for _, k := range keys {
		_, err := ds.Get(k)
		if err == nil {
			t.Fatal("should not have found this block")
		}
	}

	// commit, write them to the datastore
	err = batch.Commit()
	if err != nil {
		t.Fatal(err)
	}

	for i, k := range keys {
		blk, err := ds.Get(k)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(blk.([]byte), blocks[i]) {
			t.Fatal("blocks not correct!")
		}
	}
}

func RunBatchDeleteTest(t *testing.T, ds dstore.Batching) {
	r := rand.New()
	var keys []dstore.Key
	for i := 0; i < 20; i++ {
		blk := make([]byte, 16)
		r.Read(blk)

		key := dstore.NewKey(base32.StdEncoding.EncodeToString(blk[:8]))
		keys = append(keys, key)

		err := ds.Put(key, blk)
		if err != nil {
			t.Fatal(err)
		}
	}

	batch, err := ds.Batch()
	if err != nil {
		t.Fatal(err)
	}

	for _, k := range keys {
		err := batch.Delete(k)
		if err != nil {
			t.Fatal(err)
		}
	}
	err = batch.Commit()
	if err != nil {
		t.Fatal(err)
	}

	for _, k := range keys {
		_, err := ds.Get(k)
		if err == nil {
			t.Fatal("shouldnt have found block")
		}
	}
}
