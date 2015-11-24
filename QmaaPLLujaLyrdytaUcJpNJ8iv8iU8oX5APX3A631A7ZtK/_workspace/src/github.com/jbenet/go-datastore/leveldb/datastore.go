package leveldb

import (
	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	dsq "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/query"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/goprocess"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/syndtr/goleveldb/leveldb"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/syndtr/goleveldb/leveldb/util"
)

type datastore struct {
	DB *leveldb.DB
}

type Options opt.Options

func NewDatastore(path string, opts *Options) (*datastore, error) {
	var nopts opt.Options
	if opts != nil {
		nopts = opt.Options(*opts)
	}
	db, err := leveldb.OpenFile(path, &nopts)
	if err != nil {
		return nil, err
	}

	return &datastore{
		DB: db,
	}, nil
}

// Returns ErrInvalidType if value is not of type []byte.
//
// Note: using sync = false.
// see http://godoc.org/github.com/syndtr/goleveldb/leveldb/opt#WriteOptions
func (d *datastore) Put(key ds.Key, value interface{}) (err error) {
	val, ok := value.([]byte)
	if !ok {
		return ds.ErrInvalidType
	}
	return d.DB.Put(key.Bytes(), val, nil)
}

func (d *datastore) Get(key ds.Key) (value interface{}, err error) {
	val, err := d.DB.Get(key.Bytes(), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return nil, ds.ErrNotFound
		}
		return nil, err
	}
	return val, nil
}

func (d *datastore) Has(key ds.Key) (exists bool, err error) {
	return d.DB.Has(key.Bytes(), nil)
}

func (d *datastore) Delete(key ds.Key) (err error) {
	err = d.DB.Delete(key.Bytes(), nil)
	if err == leveldb.ErrNotFound {
		return ds.ErrNotFound
	}
	return err
}

func (d *datastore) Query(q dsq.Query) (dsq.Results, error) {

	// we can use multiple iterators concurrently. see:
	// https://godoc.org/github.com/syndtr/goleveldb/leveldb#DB.NewIterator
	// advance the iterator only if the reader reads
	//
	// run query in own sub-process tied to Results.Process(), so that
	// it waits for us to finish AND so that clients can signal to us
	// that resources should be reclaimed.
	qrb := dsq.NewResultBuilder(q)
	qrb.Process.Go(func(worker goprocess.Process) {
		d.runQuery(worker, qrb)
	})

	// go wait on the worker (without signaling close)
	go qrb.Process.CloseAfterChildren()

	// Now, apply remaining things (filters, order)
	qr := qrb.Results()
	for _, f := range q.Filters {
		qr = dsq.NaiveFilter(qr, f)
	}
	for _, o := range q.Orders {
		qr = dsq.NaiveOrder(qr, o)
	}
	return qr, nil
}

func (d *datastore) runQuery(worker goprocess.Process, qrb *dsq.ResultBuilder) {

	var rnge *util.Range
	if qrb.Query.Prefix != "" {
		rnge = util.BytesPrefix([]byte(qrb.Query.Prefix))
	}
	i := d.DB.NewIterator(rnge, nil)
	defer i.Release()

	// advance iterator for offset
	if qrb.Query.Offset > 0 {
		for j := 0; j < qrb.Query.Offset; j++ {
			i.Next()
		}
	}

	// iterate, and handle limit, too
	for sent := 0; i.Next(); sent++ {
		// end early if we hit the limit
		if qrb.Query.Limit > 0 && sent >= qrb.Query.Limit {
			break
		}

		k := ds.NewKey(string(i.Key())).String()
		e := dsq.Entry{Key: k}

		if !qrb.Query.KeysOnly {
			buf := make([]byte, len(i.Value()))
			copy(buf, i.Value())
			e.Value = buf
		}

		select {
		case qrb.Output <- dsq.Result{Entry: e}: // we sent it out
		case <-worker.Closing(): // client told us to end early.
			break
		}
	}

	if err := i.Error(); err != nil {
		select {
		case qrb.Output <- dsq.Result{Error: err}: // client read our error
		case <-worker.Closing(): // client told us to end.
			return
		}
	}
}

func (d *datastore) Batch() (ds.Batch, error) {
	// TODO: implement batch on leveldb
	return nil, ds.ErrBatchUnsupported
}

// LevelDB needs to be closed.
func (d *datastore) Close() (err error) {
	return d.DB.Close()
}

func (d *datastore) IsThreadSafe() {}
