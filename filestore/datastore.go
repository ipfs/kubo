package filestore

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	//"runtime/debug"
	//"bytes"
	//"time"

	k "github.com/ipfs/go-ipfs/blocks/key"
	ds "gx/ipfs/QmZ6A6P6AMo8SR3jXAwzTuSU6B9R2Y4eqW2yW9VvfUayDN/go-datastore"
	"gx/ipfs/QmZ6A6P6AMo8SR3jXAwzTuSU6B9R2Y4eqW2yW9VvfUayDN/go-datastore/query"
	//mh "gx/ipfs/QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku/go-multihash"
	"gx/ipfs/QmQopLATEYMNg7dVqZRNDfeE2S1yKy8zrRh5xnYiuqeZBn/goprocess"
	b58 "gx/ipfs/QmT8rehPR3F6bmwL6zjUN8XpiDBFFpMP2myPdC6ApsWfJf/go-base58"
	logging "gx/ipfs/QmYtB7Qge8cJpXc4irsEp8zRqfnZMBeB7aTrMEkPk67DRv/go-log"
	dsq "gx/ipfs/QmZ6A6P6AMo8SR3jXAwzTuSU6B9R2Y4eqW2yW9VvfUayDN/go-datastore/query"
	u "gx/ipfs/QmZNVWh8LLjAavuQ2JXuFmuYH3C11xo988vSgp7UQrTRj1/go-ipfs-util"
	"gx/ipfs/QmbBhyDKsY4mbY6xsKt3qu9Y7FPvMJ6qbD8AMjYYvPRw1g/goleveldb/leveldb"
	"gx/ipfs/QmbBhyDKsY4mbY6xsKt3qu9Y7FPvMJ6qbD8AMjYYvPRw1g/goleveldb/leveldb/opt"
	"gx/ipfs/QmbBhyDKsY4mbY6xsKt3qu9Y7FPvMJ6qbD8AMjYYvPRw1g/goleveldb/leveldb/util"
)

var log = logging.Logger("filestore")
var Logger = log

const (
	VerifyNever     = 0
	VerifyIfChanged = 1
	VerifyAlways    = 2
)

type Datastore struct {
	db     *leveldb.DB
	verify int
}

func (d *Datastore) DB() *leveldb.DB {
	return d.db
}

func New(path string, verify int) (*Datastore, error) {
	db, err := leveldb.OpenFile(path, &opt.Options{
		Compression: opt.NoCompression,
	})
	if err != nil {
		return nil, err
	}
	return &Datastore{db, verify}, nil
}

func (d *Datastore) Put(key ds.Key, value interface{}) (err error) {
	dataObj, ok := value.(*DataObj)
	if !ok {
		panic(ds.ErrInvalidType)
	}

	// Make sure the filename is an absolute path
	if !filepath.IsAbs(dataObj.FilePath) {
		return errors.New("datastore put: non-absolute filename: " + dataObj.FilePath)
	}

	// Make sure we can read the file as a sanity check
	file, err := os.Open(dataObj.FilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// See if we have the whole file in the block
	if dataObj.Offset == 0 && !dataObj.WholeFile() {
		// Get the file size
		info, err := file.Stat()
		if err != nil {
			return err
		}
		if dataObj.Size == uint64(info.Size()) {
			dataObj.Flags |= WholeFile
		}
	}

	return d.PutDirect(key, dataObj)
}

func (d *Datastore) PutDirect(key ds.Key, dataObj *DataObj) (err error) {
	data, err := dataObj.Marshal()
	if err != nil {
		return err
	}
	keyBytes := key.Bytes()
	log.Debugf("adding block %s\n", b58.Encode(keyBytes[1:]))
	return d.db.Put(keyBytes, data, nil)
}

func (d *Datastore) Get(key ds.Key) (value interface{}, err error) {
	val, err := d.GetDirect(key)
	if err != nil {
		return nil, err
	}
	return d.GetData(key, val, d.verify, true)
}

// Get the key as a DataObj
func (d *Datastore) GetDirect(key ds.Key) (*DataObj, error) {
	val, err := d.db.Get(key.Bytes(), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return nil, ds.ErrNotFound
		}
		return nil, err
	}
	return Decode(val)
}

func Decode(data []byte) (*DataObj, error) {
	val := new(DataObj)
	err := val.Unmarshal(data)
	if err != nil {
		return nil, err
	}
	return val, nil
}

type InvalidBlock struct{}

func (e InvalidBlock) Error() string {
	return "datastore: block verification failed"
}

// Get the orignal data out of the DataObj
func (d *Datastore) GetData(key ds.Key, val *DataObj, verify int, update bool) ([]byte, error) {
	if val == nil {
		return nil, errors.New("Nil DataObj")
	} else if val.NoBlockData() {
		if verify != VerifyIfChanged {
			update = false
		}
		file, err := os.Open(val.FilePath)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		_, err = file.Seek(int64(val.Offset), 0)
		if err != nil {
			return nil, err
		}
		data, _, err := Reconstruct(val.Data, file, val.Size)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return nil, err
		}
		modtime := val.ModTime
		if update || verify == VerifyIfChanged {
			fileInfo, err := file.Stat()
			if err != nil {
				return nil, err
			}
			modtime = FromTime(fileInfo.ModTime())
		}
		if err != nil {
			log.Debugf("invalid block: %s: %s\n", b58.Encode(key.Bytes()[1:]), err.Error())
		}
		invalid := val.Invalid() || err != nil
		if err == nil && (verify == VerifyAlways || (verify == VerifyIfChanged && modtime != val.ModTime)) {
			log.Debugf("verifying block %s\n", b58.Encode(key.Bytes()[1:]))
			newKey := k.Key(u.Hash(data)).DsKey()
			invalid = newKey != key
		}
		if update && (invalid != val.Invalid() || modtime != val.ModTime) {
			log.Debugf("updating block %s\n", b58.Encode(key.Bytes()[1:]))
			newVal := *val
			newVal.SetInvalid(invalid)
			newVal.ModTime = modtime
			// ignore errors as they are nonfatal
			_ = d.PutDirect(key, &newVal)
		}
		if invalid {
			if err != nil {
				log.Debugf("invalid block %s: %s\n", b58.Encode(key.Bytes()[1:]), err.Error())
			} else {
				log.Debugf("invalid block %s\n", b58.Encode(key.Bytes()[1:]))
			}
			return nil, InvalidBlock{}
		} else {
			return data, nil
		}
	} else {
		return val.Data, nil
	}
}

func (d *Datastore) Has(key ds.Key) (exists bool, err error) {
	return d.db.Has(key.Bytes(), nil)
}

func (d *Datastore) Delete(key ds.Key) error {
	return ds.ErrNotFound
}

func (d *Datastore) DeleteDirect(key ds.Key) error {
	// leveldb Delete will not return an error if the key doesn't
	// exist (see https://github.com/syndtr/goleveldb/issues/109),
	// so check that the key exists first and if not return an
	// error
	keyBytes := key.Bytes()
	exists, err := d.db.Has(keyBytes, nil)
	if !exists {
		return ds.ErrNotFound
	} else if err != nil {
		return err
	}
	return d.db.Delete(keyBytes, nil)
}

func (d *Datastore) Query(q query.Query) (query.Results, error) {
	if (q.Prefix != "" && q.Prefix != "/") ||
		len(q.Filters) > 0 ||
		len(q.Orders) > 0 ||
		q.Limit > 0 ||
		q.Offset > 0 ||
		!q.KeysOnly {
		// TODO this is overly simplistic, but the only caller is
		// `ipfs refs local` for now, and this gets us moving.
		return nil, errors.New("filestore only supports listing all keys in random order")
	}
	qrb := dsq.NewResultBuilder(q)
	qrb.Process.Go(func(worker goprocess.Process) {
		var rnge *util.Range
		i := d.db.NewIterator(rnge, nil)
		defer i.Release()
		for i.Next() {
			k := ds.NewKey(string(i.Key())).String()
			e := dsq.Entry{Key: k}
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
	})
	go qrb.Process.CloseAfterChildren()
	return qrb.Results(), nil
}

func (d *Datastore) Close() error {
	return d.db.Close()
}

func (d *Datastore) Batch() (ds.Batch, error) {
	return ds.NewBasicBatch(d), nil
}
