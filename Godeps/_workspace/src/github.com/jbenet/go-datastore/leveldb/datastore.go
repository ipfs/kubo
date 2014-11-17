package leveldb

import (
	"io"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/syndtr/goleveldb/leveldb"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/syndtr/goleveldb/leveldb/opt"
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
	return ds.GetBackedHas(d, key)
}

func (d *datastore) Delete(key ds.Key) (err error) {
	err = d.DB.Delete(key.Bytes(), nil)
	if err == leveldb.ErrNotFound {
		return ds.ErrNotFound
	}
	return err
}

func (d *datastore) KeyList() ([]ds.Key, error) {
	i := d.DB.NewIterator(nil, nil)
	var keys []ds.Key
	for i.Next() {
		keys = append(keys, ds.NewKey(string(i.Key())))
	}
	return keys, nil
}

// LevelDB needs to be closed.
func (d *datastore) Close() (err error) {
	return d.DB.Close()
}

func (d *datastore) IsThreadSafe() {}
