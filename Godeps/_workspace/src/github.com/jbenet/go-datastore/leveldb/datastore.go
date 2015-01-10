package leveldb

import (
	"io"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/syndtr/goleveldb/leveldb"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/syndtr/goleveldb/leveldb/util"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	dsq "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/query"
)

type Datastore interface {
	ds.ThreadSafeDatastore
	io.Closer
}

type datastore struct {
	DB *leveldb.DB
}

type Options opt.Options

func NewDatastore(path string, opts *Options) (Datastore, error) {
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

func (d *datastore) Query(q dsq.Query) (*dsq.Results, error) {
	var rnge *util.Range
	if q.Prefix != "" {
		rnge = util.BytesPrefix([]byte(q.Prefix))
	}
	i := d.DB.NewIterator(rnge, nil)

	// offset
	if q.Offset > 0 {
		for j := 0; j < q.Offset; j++ {
			i.Next()
		}
	}

	var es []dsq.Entry
	for i.Next() {

		// limit
		if q.Limit > 0 && len(es) >= q.Limit {
			break
		}

		k := ds.NewKey(string(i.Key())).String()
		e := dsq.Entry{Key: k}

		if !q.KeysOnly {
			buf := make([]byte, len(i.Value()))
			copy(buf, i.Value())
			e.Value = buf
		}

		es = append(es, e)
	}
	i.Release()
	if err := i.Error(); err != nil {
		return nil, err
	}

	// Now, apply remaining pieces.
	q2 := q
	q2.Offset = 0 // already applied
	q2.Limit = 0  // already applied
	// TODO: make this async with:
	// qr := dsq.ResultsWithEntriesChan(q, ch)
	qr := dsq.ResultsWithEntries(q, es)
	qr = q2.ApplyTo(qr)
	qr.Query = q // set it back
	return qr, nil
}

// LevelDB needs to be closed.
func (d *datastore) Close() (err error) {
	return d.DB.Close()
}

func (d *datastore) IsThreadSafe() {}
