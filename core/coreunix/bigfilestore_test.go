package coreunix

import (
	"testing"

	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
)

func TestBigFileStorePutThenGet(t *testing.T) {
	bfs := NewBigFileStore(ds_sync.MutexWrap(ds.NewMapDatastore()))

	streamCid, err := cid.Parse("QmWQadpxHe1UgAMdkZ5tm7znzqiixwo5u9XLKCtPGLtdDs")
	if err != nil {
		panic(err)
	}

	cidStrs := []string{
		"QmQPeNsJPyVWPFDVHb77w8G42Fvo15z4bG2X8D2GhfbSXc",
		"QmYff9iHR1Hz6wufVeJodzXqQm4pkK4QNS9ms8tyPKVWm1",
		"QmcvcJRuxFUsM1deMwMzDL7fWB2A7rXhFRNrBAf81KyFuN",
	}
	chunks := make([]*ChunkingManifestChunk, len(cidStrs))

	for i := range cidStrs {
		c, err := cid.Parse(cidStrs[i])
		if err != nil {
			panic(err)
		}
		chunks[i] = &ChunkingManifestChunk{
			ChunkCid: c,
			Offset:   uint64(i * 1024),
			Size:     4096,
		}
	}

	err = bfs.Put(streamCid, chunks)
	if err != nil {
		t.Fatal(err)
	}

	chunks2, err := bfs.Get(streamCid)
	if err != nil {
		t.Fatal(err)
	}

	if len(chunks2) != len(chunks) {
		t.Fatal("wrong number of chunks returned")
	}
	for i := range chunks {
		if !chunks2[i].ChunkCid.Equals(chunks[i].ChunkCid) {
			t.Errorf("chunks2[%d] != chunks[%d]", i, i)
		}
	}
}
