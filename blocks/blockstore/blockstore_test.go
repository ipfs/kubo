package blockstore

import (
	"bytes"
	"fmt"
	"testing"

	ds "gx/ipfs/QmZ6A6P6AMo8SR3jXAwzTuSU6B9R2Y4eqW2yW9VvfUayDN/go-datastore"
	dsq "gx/ipfs/QmZ6A6P6AMo8SR3jXAwzTuSU6B9R2Y4eqW2yW9VvfUayDN/go-datastore/query"
	ds_sync "gx/ipfs/QmZ6A6P6AMo8SR3jXAwzTuSU6B9R2Y4eqW2yW9VvfUayDN/go-datastore/sync"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"

	blocks "github.com/ipfs/go-ipfs/blocks"
	key "github.com/ipfs/go-ipfs/blocks/key"
)

// TODO(brian): TestGetReturnsNil

func TestGetWhenKeyNotPresent(t *testing.T) {
	bs := NewBlockstore(ds_sync.MutexWrap(ds.NewMapDatastore()))
	_, err := bs.Get(key.Key("not present"))

	if err != nil {
		t.Log("As expected, block is not present")
		return
	}
	t.Fail()
}

func TestGetWhenKeyIsEmptyString(t *testing.T) {
	bs := NewBlockstore(ds_sync.MutexWrap(ds.NewMapDatastore()))
	_, err := bs.Get(key.Key(""))
	if err != ErrNotFound {
		t.Fail()
	}
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
	if !bytes.Equal(block.Data(), blockFromBlockstore.Data()) {
		t.Fail()
	}
}

func newBlockStoreWithKeys(t *testing.T, d ds.Datastore, N int) (Blockstore, []key.Key) {
	if d == nil {
		d = ds.NewMapDatastore()
	}
	bs := NewBlockstore(ds_sync.MutexWrap(d))

	keys := make([]key.Key, N)
	for i := 0; i < N; i++ {
		block := blocks.NewBlock([]byte(fmt.Sprintf("some data %d", i)))
		err := bs.Put(block)
		if err != nil {
			t.Fatal(err)
		}
		keys[i] = block.Key()
	}
	return bs, keys
}

func collect(ch <-chan key.Key) []key.Key {
	var keys []key.Key
	for k := range ch {
		keys = append(keys, k)
	}
	return keys
}

func TestAllKeysSimple(t *testing.T) {
	bs, keys := newBlockStoreWithKeys(t, nil, 100)

	ctx := context.Background()
	ch, err := bs.AllKeysChan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	keys2 := collect(ch)

	// for _, k2 := range keys2 {
	// 	t.Log("found ", k2.B58String())
	// }

	expectMatches(t, keys, keys2)
}

func TestAllKeysRespectsContext(t *testing.T) {
	N := 100

	d := &queryTestDS{ds: ds.NewMapDatastore()}
	bs, _ := newBlockStoreWithKeys(t, d, N)

	started := make(chan struct{}, 1)
	done := make(chan struct{}, 1)
	errors := make(chan error, 100)

	getKeys := func(ctx context.Context) {
		started <- struct{}{}
		ch, err := bs.AllKeysChan(ctx) // once without cancelling
		if err != nil {
			errors <- err
		}
		_ = collect(ch)
		done <- struct{}{}
		errors <- nil // a nil one to signal break
	}

	// Once without context, to make sure it all works
	{
		var results dsq.Results
		var resultsmu = make(chan struct{})
		resultChan := make(chan dsq.Result)
		d.SetFunc(func(q dsq.Query) (dsq.Results, error) {
			results = dsq.ResultsWithChan(q, resultChan)
			resultsmu <- struct{}{}
			return results, nil
		})

		go getKeys(context.Background())

		// make sure it's waiting.
		<-started
		<-resultsmu
		select {
		case <-done:
			t.Fatal("sync is wrong")
		case <-results.Process().Closing():
			t.Fatal("should not be closing")
		case <-results.Process().Closed():
			t.Fatal("should not be closed")
		default:
		}

		e := dsq.Entry{Key: BlockPrefix.ChildString("foo").String()}
		resultChan <- dsq.Result{Entry: e} // let it go.
		close(resultChan)
		<-done                       // should be done now.
		<-results.Process().Closed() // should be closed now

		// print any errors
		for err := range errors {
			if err == nil {
				break
			}
			t.Error(err)
		}
	}

	// Once with
	{
		var results dsq.Results
		var resultsmu = make(chan struct{})
		resultChan := make(chan dsq.Result)
		d.SetFunc(func(q dsq.Query) (dsq.Results, error) {
			results = dsq.ResultsWithChan(q, resultChan)
			resultsmu <- struct{}{}
			return results, nil
		})

		ctx, cancel := context.WithCancel(context.Background())
		go getKeys(ctx)

		// make sure it's waiting.
		<-started
		<-resultsmu
		select {
		case <-done:
			t.Fatal("sync is wrong")
		case <-results.Process().Closing():
			t.Fatal("should not be closing")
		case <-results.Process().Closed():
			t.Fatal("should not be closed")
		default:
		}

		cancel() // let it go.

		select {
		case <-done:
			t.Fatal("sync is wrong")
		case <-results.Process().Closed():
			t.Fatal("should not be closed") // should not be closed yet.
		case <-results.Process().Closing():
			// should be closing now!
			t.Log("closing correctly at this point.")
		}

		close(resultChan)
		<-done                       // should be done now.
		<-results.Process().Closed() // should be closed now

		// print any errors
		for err := range errors {
			if err == nil {
				break
			}
			t.Error(err)
		}
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

func expectMatches(t *testing.T, expect, actual []key.Key) {

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

type queryTestDS struct {
	cb func(q dsq.Query) (dsq.Results, error)
	ds ds.Datastore
}

func (c *queryTestDS) SetFunc(f func(dsq.Query) (dsq.Results, error)) { c.cb = f }

func (c *queryTestDS) Put(key ds.Key, value interface{}) (err error) {
	return c.ds.Put(key, value)
}

func (c *queryTestDS) Get(key ds.Key) (value interface{}, err error) {
	return c.ds.Get(key)
}

func (c *queryTestDS) Has(key ds.Key) (exists bool, err error) {
	return c.ds.Has(key)
}

func (c *queryTestDS) Delete(key ds.Key) (err error) {
	return c.ds.Delete(key)
}

func (c *queryTestDS) Query(q dsq.Query) (dsq.Results, error) {
	if c.cb != nil {
		return c.cb(q)
	}
	return c.ds.Query(q)
}

func (c *queryTestDS) Batch() (ds.Batch, error) {
	return ds.NewBasicBatch(c), nil
}
