package filestore

import (
	"gx/ipfs/QmbBhyDKsY4mbY6xsKt3qu9Y7FPvMJ6qbD8AMjYYvPRw1g/goleveldb/leveldb"
	"gx/ipfs/QmbBhyDKsY4mbY6xsKt3qu9Y7FPvMJ6qbD8AMjYYvPRw1g/goleveldb/leveldb/iterator"
	"gx/ipfs/QmbBhyDKsY4mbY6xsKt3qu9Y7FPvMJ6qbD8AMjYYvPRw1g/goleveldb/leveldb/opt"
	"gx/ipfs/QmbBhyDKsY4mbY6xsKt3qu9Y7FPvMJ6qbD8AMjYYvPRw1g/goleveldb/leveldb/util"
)

type readops interface {
	Get(key []byte, ro *opt.ReadOptions) (value []byte, err error)
	Has(key []byte, ro *opt.ReadOptions) (ret bool, err error)
	NewIterator(slice *util.Range, ro *opt.ReadOptions) iterator.Iterator
}

type dbread struct {
	db readops
}

type dbwrap struct {
	dbread
	db *leveldb.DB
}

func Decode(bytes []byte) (*DataObj, error) {
	val := new(DataObj)
	err := val.Unmarshal(bytes)
	if err != nil {
		return nil, err
	}
	return val, nil
}

func (w dbread) GetHash(key []byte) (*DataObj, error) {
	val, err := w.db.Get(key, nil)
	if err != nil {
		return nil, err
	}
	return Decode(val)
}

func (w dbread) Get(key *DbKey) (*DataObj, error) {
	if key.FilePath == "" {
		return w.GetHash(key.Bytes)
	}
	val, err := w.db.Get(key.Bytes, nil)
	if err != nil {
		return nil, err
	}
	dataObj, err := Decode(val)
	if err != nil {
		return nil, err
	}
	dataObj.FilePath = key.FilePath
	dataObj.Offset = uint64(key.Offset)
	return dataObj, err
}

func (d dbread) GetAlternatives(key []byte) *Iterator {
	start := make([]byte, 0, len(key)+1)
	start = append(start, key...)
	start = append(start, byte('/'))
	stop := make([]byte, 0, len(key)+1)
	stop = append(stop, key...)
	stop = append(stop, byte('/')+1)
	return &Iterator{iter: d.db.NewIterator(&util.Range{start, stop}, nil)}
}

func (w dbread) HasHash(key []byte) (bool, error) {
	return w.db.Has(key, nil)
}

func (w dbread) Has(key *DbKey) (bool, error) {
	return w.db.Has(key.Bytes, nil)
}

func marshal(key *DbKey, val *DataObj) ([]byte, error) {
	if key.FilePath != "" {
		val.FilePath = ""
		val.Offset = 0
	}
	return val.Marshal()
}

func (w dbwrap) Put(key *DbKey, val *DataObj) error {
	data, err := marshal(key, val)
	if err != nil {
		return err
	}
	return w.db.Put(key.Bytes, data, nil)
}

func (w dbwrap) Delete(key []byte) error {
	return w.db.Delete(key, nil)
}

func (w dbwrap) Write(b *dbbatch) error {
	return w.db.Write(b.batch, nil)
}

type dbbatch struct {
	batch *leveldb.Batch
}

func (b dbbatch) Put(key *DbKey, val *DataObj) error {
	data, err := marshal(key, val)
	if err != nil {
		return err
	}
	b.batch.Put(key.Bytes, data)
	return nil
}

func (b dbbatch) Delete(key []byte) {
	b.batch.Delete(key)
}

type Iterator struct {
	key   *DbKey
	value *DataObj
	iter  iterator.Iterator
}

func (d dbread) NewIterator() *Iterator {
	return &Iterator{iter: d.db.NewIterator(nil, nil)}
}

func (itr *Iterator) Next() bool {
	itr.key = nil
	itr.value = nil
	return itr.iter.Next()
}

func (itr *Iterator) Key() *DbKey {
	if itr.key == nil {
		bytes := itr.iter.Key()
		itr.key = &DbKey{
			Key:   ParseDsKey(string(bytes)),
			Bytes: bytes,
		}
	}
	return itr.key
}

func (itr *Iterator) Value() (*DataObj, error) {
	if itr.value != nil {
		return itr.value, nil
	}
	bytes := itr.iter.Value()
	if bytes == nil {
		return nil, nil
	}
	var err error
	itr.value, err = Decode(bytes)
	if err != nil {
		return nil, err
	}
	key := itr.Key()
	if key.FilePath != "" {
		itr.value.FilePath = key.FilePath
		itr.value.Offset = uint64(key.Offset)
	}
	return itr.value, nil
}

func (itr *Iterator) Release() {
	itr.iter.Release()
}
