package simple_test

import (
	"context"
	"math/rand"
	"testing"
	"time"

	cid "github.com/ipfs/go-cid"
	datastore "github.com/ipfs/go-datastore"
	sync "github.com/ipfs/go-datastore/sync"
	blocksutil "github.com/ipfs/go-ipfs-blocksutil"
	peer "github.com/libp2p/go-libp2p-core/peer"

	q "github.com/ipfs/go-ipfs/provider/queue"

	. "github.com/ipfs/go-ipfs/provider/simple"
)

var blockGenerator = blocksutil.NewBlockGenerator()

type mockRouting struct {
	provided chan cid.Cid
}

func (r *mockRouting) Provide(ctx context.Context, cid cid.Cid, recursive bool) error {
	r.provided <- cid
	return nil
}

func (r *mockRouting) FindProvidersAsync(ctx context.Context, cid cid.Cid, timeout int) <-chan peer.AddrInfo {
	return nil
}

func mockContentRouting() *mockRouting {
	r := mockRouting{}
	r.provided = make(chan cid.Cid)
	return &r
}

func TestAnnouncement(t *testing.T) {
	ctx := context.Background()
	defer ctx.Done()

	ds := sync.MutexWrap(datastore.NewMapDatastore())
	queue, err := q.NewQueue(ctx, "test", ds)
	if err != nil {
		t.Fatal(err)
	}

	r := mockContentRouting()

	prov := NewProvider(ctx, queue, r)
	prov.Run()

	cids := cid.NewSet()

	for i := 0; i < 100; i++ {
		c := blockGenerator.Next().Cid()
		cids.Add(c)
	}

	go func() {
		for _, c := range cids.Keys() {
			err = prov.Provide(c)
			// A little goroutine stirring to exercise some different states
			r := rand.Intn(10)
			time.Sleep(time.Microsecond * time.Duration(r))
		}
	}()

	for cids.Len() > 0 {
		select {
		case cp := <-r.provided:
			if !cids.Has(cp) {
				t.Fatal("Wrong CID provided")
			}
			cids.Remove(cp)
		case <-time.After(time.Second * 5):
			t.Fatal("Timeout waiting for cids to be provided.")
		}
	}
}
