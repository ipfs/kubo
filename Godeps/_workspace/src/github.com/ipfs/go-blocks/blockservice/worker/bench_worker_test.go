package worker

import (
	"testing"

	blocks "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-blocks"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-blocks/blockservice/exchange/offline"
	blockstore "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-blocks/blockstore"
	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	dssync "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
)

func BenchmarkHandle10KBlocks(b *testing.B) {
	bstore := blockstore.NewBlockstore(dssync.MutexWrap(ds.NewMapDatastore()))
	var testdata []*blocks.Block
	for i := 0; i < 10000; i++ {
		testdata = append(testdata, blocks.NewBlock([]byte(string(i))))
	}
	b.ResetTimer()
	b.SetBytes(10000)
	for i := 0; i < b.N; i++ {

		b.StopTimer()
		w := NewWorker(offline.Exchange(bstore), Config{
			NumWorkers:       1,
			ClientBufferSize: 0,
			WorkerBufferSize: 0,
		})
		b.StartTimer()

		for _, block := range testdata {
			if err := w.HasBlock(block); err != nil {
				b.Fatal(err)
			}
		}

		b.StopTimer()
		w.Close()
		b.StartTimer()

	}
}
