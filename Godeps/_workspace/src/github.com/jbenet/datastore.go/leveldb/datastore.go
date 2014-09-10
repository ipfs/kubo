package leveldb

import (
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/syndtr/goleveldb/leveldb"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/syndtr/goleveldb/leveldb/opt"
)

// Datastore uses a standard Go map for internal storage.
type Datastore struct {
	DB *leveldb.DB
}

type Options opt.Options

func NewDatastore(path string, opts *Options) (*Datastore, error) {
	var nopts opt.Options
	if opts != nil {
		nopts = opt.Options(*opts)
	}
	db, err := leveldb.OpenFile(path, &nopts)
	if err != nil {
		return nil, err
	}

	return &Datastore{
		DB: db,
	}, nil
}

// Returns ErrInvalidType if value is not of type []byte.
//
// Note: using sync = false.
// see http://godoc.org/github.com/syndtr/goleveldb/leveldb/opt#WriteOptions
func (d *Datastore) Put(key ds.Key, value interface{}) (err error) {
	val, ok := value.([]byte)
	if !ok {
		return ds.ErrInvalidType
	}
	return d.DB.Put(key.Bytes(), val, nil)
}

func (d *Datastore) Get(key ds.Key) (value interface{}, err error) {
	val, err := d.DB.Get(key.Bytes(), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return nil, ds.ErrNotFound
		}
		return nil, err
	}
	return val, nil
}

func (d *Datastore) Has(key ds.Key) (exists bool, err error) {
	return ds.GetBackedHas(d, key)
}

func (d *Datastore) Delete(key ds.Key) (err error) {
	err = d.DB.Delete(key.Bytes(), nil)
	if err == leveldb.ErrNotFound {
		return ds.ErrNotFound
	}
	return err
}

func (d *Datastore) KeyList() ([]ds.Key, error) {
	i := d.DB.NewIterator(nil, nil)
	var keys []ds.Key
	for ; i.Valid(); i.Next() {
		keys = append(keys, ds.NewKey(string(i.Key())))
	}
	return keys, nil
}

// LevelDB needs to be closed.
func (d *Datastore) Close() (err error) {
	return d.DB.Close()
}
