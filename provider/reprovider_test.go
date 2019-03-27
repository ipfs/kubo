package provider

import (
	"context"
	"github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	"testing"
	"time"
)

func TestReAnnouncementTrigger(t *testing.T) {
	ctx := context.Background()
	defer ctx.Done()

	ds := sync.MutexWrap(datastore.NewMapDatastore())
	q, err := NewQueue(ctx, "test", ds)
	if err != nil {
		t.Fatal(err)
	}

	r := mockContentRouting()
	bs := newMapBlockstore()

	tr := NewTracker(ds)

	reprovider := NewReprovider(ctx, q, tr, time.Hour, time.Hour, bs, r)
	reprovider.Run()

	blocks := make(map[cid.Cid]blocks.Block, 0)
	for i := 0; i < 100; i++ {
		b := blockGenerator.Next()
		blocks[b.Cid()] = b

		bs.Put(b)
		tr.Track(b.Cid())
	}

	go reprovider.Trigger(ctx)

	if len(blocks) == 0 {
		t.Fatal("no test blocks to compare to, issue with test data")
	}

	for len(blocks) > 0 {
		select {
		case cp := <-r.provided:
			_, ok := blocks[cp]
			if !ok {
				t.Fatal("Wrong CID provided")
			}
			delete(blocks, cp)
		case <-time.After(time.Second * 5):
			t.Fatal("Timeout waiting for cids to be provided.")
		}
	}
}

func TestReAnnouncementTick(t *testing.T) {
	ctx := context.Background()
	defer ctx.Done()

	ds := sync.MutexWrap(datastore.NewMapDatastore())
	q, err := NewQueue(ctx, "test", ds)
	if err != nil {
		t.Fatal(err)
	}

	r := mockContentRouting()
	bs := newMapBlockstore()

	tr := NewTracker(ds)

	tick := time.Second
	reprovider := NewReprovider(ctx, q, tr, tick, tick, bs, r)
	reprovider.Run()

	blocks := make(map[cid.Cid]blocks.Block, 0)
	for i := 0; i < 100; i++ {
		b := blockGenerator.Next()
		blocks[b.Cid()] = b

		bs.Put(b)
		tr.Track(b.Cid())
	}

	if len(blocks) == 0 {
		t.Fatal("no test blocks to compare to, issue with test data")
	}

	for len(blocks) > 0 {
		select {
		// test waits tick worth of time here
		case cp := <-r.provided:
			b, ok := blocks[cp]
			if !ok {
				t.Fatalf("expected cid %s to be provided, but it was not", b.String())
			}
			delete(blocks, cp)
		case <-time.After(time.Second * 5):
			t.Fatal("Timeout waiting for cids to be provided.")
		}
	}
}

func TestUntracksBlocksNoLongerInBlockstore(t *testing.T) {
	ctx := context.Background()
	defer ctx.Done()

	ds := sync.MutexWrap(datastore.NewMapDatastore())
	q, err := NewQueue(ctx, "test", ds)
	if err != nil {
		t.Fatal(err)
	}

	r := mockContentRouting()
	bs := newMapBlockstore()

	tr := NewTracker(ds)

	tick := time.Hour
	reprovider := NewReprovider(ctx, q, tr, tick, tick, bs, r)
	reprovider.Run()

	blocks := make(map[cid.Cid]blocks.Block, 0)
	for i := 0; i < 100; i++ {
		b := blockGenerator.Next()
		blocks[b.Cid()] = b

		tr.Track(b.Cid())
	}

	reprovider.Trigger(ctx)

	// allow small amount of time for entries to be removed
	time.Sleep(time.Millisecond * 10)

	for k := range blocks {
		isTracking, err := tr.IsTracking(k)
		if err != nil {
			t.Fatal(err)
		}

		if isTracking {
			t.Fatalf("should not track %s, but did", k.String())
		}
	}

	select {
	case <-r.provided:
		t.Fatal("should not have provided anything, but did")
	case <-time.After(time.Second):
		// this is good, nothing was provided
	}
}

// Map based Blockstore for testing

type mapBlockstore struct {
	values map[cid.Cid]blocks.Block
}

func newMapBlockstore() *mapBlockstore {
	return &mapBlockstore{
		values: make(map[cid.Cid]blocks.Block),
	}
}

func (mb *mapBlockstore) DeleteBlock(cid cid.Cid) error {
	delete(mb.values, cid)
	return nil
}

func (mb *mapBlockstore) Has(cid cid.Cid) (bool, error) {
	_, ok := mb.values[cid]
	return ok, nil
}

func (mb *mapBlockstore) Get(cid cid.Cid) (blocks.Block, error) {
	b, _ := mb.values[cid]
	return b, nil
}

func (mb *mapBlockstore) GetSize(cid cid.Cid) (int, error) {
	return 0, nil
}

func (mb *mapBlockstore) Put(block blocks.Block) error {
	mb.values[block.Cid()] = block
	return nil
}

func (mb *mapBlockstore) PutMany(blocks []blocks.Block) error {
	for _, b := range blocks {
		if err := mb.Put(b); err != nil {
			return err
		}
	}
	return nil
}

func (mb *mapBlockstore) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	return make(<-chan cid.Cid), nil
}

func (mb *mapBlockstore) HashOnRead(enabled bool) {
}
