package queue

import (
	"context"
	"testing"
	"time"

	cid "github.com/ipfs/go-cid"
	datastore "github.com/ipfs/go-datastore"
	sync "github.com/ipfs/go-datastore/sync"
	"github.com/ipfs/go-ipfs-blocksutil"
)

var blockGenerator = blocksutil.NewBlockGenerator()

func makeCids(n int) []cid.Cid {
	cids := make([]cid.Cid, 0, n)
	for i := 0; i < n; i++ {
		c := blockGenerator.Next().Cid()
		cids = append(cids, c)
	}
	return cids
}

func assertOrdered(cids []cid.Cid, q *Queue, t *testing.T) {
	for _, c := range cids {
		select {
		case dequeued := <-q.dequeue:
			if c != dequeued {
				t.Fatalf("Error in ordering of CIDs retrieved from queue. Expected: %s, got: %s", c, dequeued)
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

	cids := makeCids(10)

	for _, c := range cids {
		queue.Enqueue(c)
	}

	assertOrdered(cids, queue, t)
}

func TestSparseDatastore(t *testing.T) {
	ctx := context.Background()
	defer ctx.Done()

	ds := sync.MutexWrap(datastore.NewMapDatastore())
	queue, err := NewQueue(ctx, "test", ds)
	if err != nil {
		t.Fatal(err)
	}

	cids := makeCids(10)
	for _, c := range cids {
		queue.Enqueue(c)
	}

	// remove entries in the middle
	err = queue.ds.Delete(queue.queueKey(5))
	if err != nil {
		t.Fatal(err)
	}

	err = queue.ds.Delete(queue.queueKey(6))
	if err != nil {
		t.Fatal(err)
	}

	expected := append(cids[:5], cids[7:]...)
	assertOrdered(expected, queue, t)
}

func TestMangledData(t *testing.T) {
	ctx := context.Background()
	defer ctx.Done()

	ds := sync.MutexWrap(datastore.NewMapDatastore())
	queue, err := NewQueue(ctx, "test", ds)
	if err != nil {
		t.Fatal(err)
	}

	cids := makeCids(10)
	for _, c := range cids {
		queue.Enqueue(c)
	}

	// remove entries in the middle
	err = queue.ds.Put(queue.queueKey(5), []byte("borked"))
	if err != nil {
		t.Fatal(err)
	}

	expected := append(cids[:5], cids[6:]...)
	assertOrdered(expected, queue, t)
}

func TestInitialization(t *testing.T) {
	ctx := context.Background()
	defer ctx.Done()

	ds := sync.MutexWrap(datastore.NewMapDatastore())
	queue, err := NewQueue(ctx, "test", ds)
	if err != nil {
		t.Fatal(err)
	}

	cids := makeCids(10)
	for _, c := range cids {
		queue.Enqueue(c)
	}

	assertOrdered(cids[:5], queue, t)

	// make a new queue, same data
	queue, err = NewQueue(ctx, "test", ds)
	if err != nil {
		t.Fatal(err)
	}

	assertOrdered(cids[5:], queue, t)
}

func TestInitializationWithManyCids(t *testing.T) {
	ctx := context.Background()
	defer ctx.Done()

	ds := sync.MutexWrap(datastore.NewMapDatastore())
	queue, err := NewQueue(ctx, "test", ds)
	if err != nil {
		t.Fatal(err)
	}

	cids := makeCids(25)
	for _, c := range cids {
		queue.Enqueue(c)
	}

	// make a new queue, same data
	queue, err = NewQueue(ctx, "test", ds)
	if err != nil {
		t.Fatal(err)
	}

	assertOrdered(cids, queue, t)
}
