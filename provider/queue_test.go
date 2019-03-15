package provider

import (
	"context"
	"testing"
	"time"

	cid "github.com/ipfs/go-cid"
	datastore "github.com/ipfs/go-datastore"
	sync "github.com/ipfs/go-datastore/sync"
)

func makeCids(n int) []cid.Cid {
	cids := make([]cid.Cid, 0, 10)
	for i := 0; i < 10; i++ {
		c := blockGenerator.Next().Cid()
		cids = append(cids, c)
	}
	return cids
}

func assertOrdered(cids []cid.Cid, q *Queue, t *testing.T) {
	for _, c := range cids {
		select {
		case dequeued := <- q.dequeue:
			if c != dequeued.cid {
				t.Fatalf("Error in ordering of CIDs retrieved from queue. Expected: %s, got: %s", c, dequeued.cid)
			}

		case <-time.After(time.Second * 1):
			t.Fatal("Timeout waiting for cids to be provided.")
		}
	}
}

func TestBasicOperation(t *testing.T) {
	ctx := context.Background()
	defer ctx.Done()

	ds := sync.MutexWrap(datastore.NewMapDatastore())
	queue, err := NewQueue(ctx, "test", ds)
	if err != nil {
		t.Fatal(err)
	}
	queue.Run()

	cids := makeCids(10)

	for _, c := range cids {
		err = queue.Enqueue(c)
		if err != nil {
			t.Fatal("Failed to enqueue CID")
		}
	}

	assertOrdered(cids, queue, t)
}

func TestInitialization(t *testing.T) {
	ctx := context.Background()
	defer ctx.Done()

	ds := sync.MutexWrap(datastore.NewMapDatastore())
	queue, err := NewQueue(ctx, "test", ds)
	if err != nil {
		t.Fatal(err)
	}
	queue.Run()

	cids := makeCids(10)

	for _, c := range cids {
		err = queue.Enqueue(c)
		if err != nil {
			t.Fatal("Failed to enqueue CID")
		}
	}

	assertOrdered(cids[:5], queue, t)

	// make a new queue, same data
	queue, err = NewQueue(ctx, "test", ds)
	if err != nil {
		t.Fatal(err)
	}
	queue.Run()

	assertOrdered(cids[5:], queue, t)
}
