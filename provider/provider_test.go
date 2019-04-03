package provider

import (
	"context"
	"math/rand"
	"testing"
	"time"

	cid "github.com/ipfs/go-cid"
	datastore "github.com/ipfs/go-datastore"
	sync "github.com/ipfs/go-datastore/sync"
	blocksutil "github.com/ipfs/go-ipfs-blocksutil"
	pstore "github.com/libp2p/go-libp2p-peerstore"
)

var blockGenerator = blocksutil.NewBlockGenerator()

type mockRouting struct {
	provided chan cid.Cid
}

func (r *mockRouting) Provide(ctx context.Context, cid cid.Cid, recursive bool) error {
	r.provided <- cid
	return nil
}

func (r *mockRouting) FindProvidersAsync(ctx context.Context, cid cid.Cid, timeout int) <-chan pstore.PeerInfo {
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
	queue, err := NewQueue(ctx, "test", ds)
	if err != nil {
		t.Fatal(err)
	}

	tracker := NewTracker(ds)

	r := mockContentRouting()

	provider := NewProvider(ctx, queue, tracker, r)
	provider.Run()

	cids := cid.NewSet()

	for i := 0; i < 100; i++ {
		c := blockGenerator.Next().Cid()
		cids.Add(c)
	}

	go func() {
		for _, c := range cids.Keys() {
			provider.Provide(c)
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

func TestAnnouncementWhenAlreadyAnnounced(t *testing.T) {
	ctx := context.Background()
	defer ctx.Done()

	ds := sync.MutexWrap(datastore.NewMapDatastore())
	queue, err := NewQueue(ctx, "test", ds)
	if err != nil {
		t.Fatal(err)
	}

	tracker := NewTracker(ds)

	r := mockContentRouting()

	provider := NewProvider(ctx, queue, tracker, r)
	provider.Run()

	c := blockGenerator.Next().Cid()

	provider.Provide(c)
	if err != nil {
		t.Fatal(err)
	}

	// give time to provide and track
	time.Sleep(time.Millisecond * 10)

	// provide the same cid again
	provider.Provide(c)
	if err != nil {
		t.Fatal(err)
	}

	cp := <-r.provided
	if c != cp {
		t.Fatalf("expected %s to be provided, but %s was instead", c.String(), cp.String())
	}

	select {
	case rc := <-r.provided:
		t.Fatalf("expected nothing to be provided, but %s was", rc.String())
	case <-time.After(time.Second):
		// this is good, nothing should have been provided
	}
}
