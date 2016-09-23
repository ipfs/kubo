package filestore

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	//"runtime"
	//"runtime/debug"
	//"time"

	ds "gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore"
	"gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore/query"
	k "gx/ipfs/Qmce4Y4zg3sYr7xKM5UueS67vhNni6EeWgCRnb7MbLJMew/go-key"
	//mh "gx/ipfs/QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku/go-multihash"
	"gx/ipfs/QmSF8fPo3jgVBAy8fpdjjYqgG87dkJgUprRBHRd2tmfgpP/goprocess"
	logging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
	b58 "gx/ipfs/QmT8rehPR3F6bmwL6zjUN8XpiDBFFpMP2myPdC6ApsWfJf/go-base58"
	u "gx/ipfs/QmZNVWh8LLjAavuQ2JXuFmuYH3C11xo988vSgp7UQrTRj1/go-ipfs-util"
	"gx/ipfs/QmbBhyDKsY4mbY6xsKt3qu9Y7FPvMJ6qbD8AMjYYvPRw1g/goleveldb/leveldb"
	"gx/ipfs/QmbBhyDKsY4mbY6xsKt3qu9Y7FPvMJ6qbD8AMjYYvPRw1g/goleveldb/leveldb/iterator"
	"gx/ipfs/QmbBhyDKsY4mbY6xsKt3qu9Y7FPvMJ6qbD8AMjYYvPRw1g/goleveldb/leveldb/opt"
	"gx/ipfs/QmbBhyDKsY4mbY6xsKt3qu9Y7FPvMJ6qbD8AMjYYvPRw1g/goleveldb/leveldb/util"
	dsq "gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore/query"
)

var log = logging.Logger("filestore")
var Logger = log

type VerifyWhen int

const (
	VerifyNever VerifyWhen = iota
	VerifyIfChanged
	VerifyAlways
)

type readonly interface {
	Get(key []byte, ro *opt.ReadOptions) (value []byte, err error)
	Has(key []byte, ro *opt.ReadOptions) (ret bool, err error)
	NewIterator(slice *util.Range, ro *opt.ReadOptions) iterator.Iterator
}

type Basic struct {
	db readonly
	ds *Datastore
}

type Snapshot struct {
	*Basic
}

func (ss *Snapshot) Defined() bool { return ss.Basic != nil }

type Datastore struct {
	db     *leveldb.DB
	verify VerifyWhen

	// updateLock is designed to only be held for a very short
	// period of time.  It, as it names suggests, is designed to
	// avoid a race condataion when updating a DataObj and is only
	// used by the Update function, which all functions that
	// modify the DB use
	updateLock sync.Mutex

	// A snapshot of the DB the last time it was in a consistent
	// state, if null than there are no outstanding adds
	snapshot Snapshot
	// If the snapshot was used, if not true than Release() can be
	// called to help save space
	snapshotUsed bool

	addLocker addLocker

	// maintenanceLock is designed to be help for a longer period
	// of time.  It, as it names suggests, is designed to be avoid
	// race conditions during maintenance.  Operations that add
	// blocks are expected to already be holding the "read" lock.
	// Maintaince operations will hold an exclusive lock.
	//maintLock  sync.RWMutex
}

func (d *Basic) DB() readonly { return d.db }

func (d *Datastore) DB() *leveldb.DB { return d.db }

func (d *Datastore) AsBasic() *Basic { return &Basic{d.db, d} }

func (d *Basic) AsFull() *Datastore { return d.ds }

func (d *Datastore) GetSnapshot() (Snapshot, error) {
	if d.snapshot.Defined() {
		d.snapshotUsed = true
		return d.snapshot, nil
	}
	ss, err := d.db.GetSnapshot()
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{&Basic{ss, d}}, nil
}

func (d *Datastore) releaseSnapshot() {
	if !d.snapshot.Defined() {
		return
	}
	if !d.snapshotUsed {
		d.snapshot.db.(*leveldb.Snapshot).Release()
	}
	d.snapshot = Snapshot{}
}

func (d *Datastore) updateOnGet() bool {
	return d.verify == VerifyIfChanged
}

func Init(path string) error {
	db, err := leveldb.OpenFile(path, &opt.Options{
		Compression: opt.NoCompression,
	})
	if err != nil {
		return err
	}
	db.Close()
	return nil
}

func New(path string, verify VerifyWhen) (*Datastore, error) {
	db, err := leveldb.OpenFile(path, &opt.Options{
		Compression:    opt.NoCompression,
		ErrorIfMissing: true,
	})
	if err != nil {
		return nil, err
	}
	ds := &Datastore{db: db, verify: verify}
	ds.addLocker.ds = ds
	return ds, nil
}

func (d *Datastore) Put(key ds.Key, value interface{}) (err error) {
	dataObj, ok := value.(*DataObj)
	if !ok {
		panic(ds.ErrInvalidType)
	}

	if dataObj.FilePath == "" {
		_, err := d.Update(key.Bytes(), nil, dataObj)
		return err
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

	_, err = d.Update(key.Bytes(), nil, dataObj)
	return err
}

// Prevent race condations up update a key while holding a lock, if
// origData is defined and the current value in datastore is not the
// same return false and abort the update, otherwise update the key if
// newData is defined, if it is nil than delete the key.  If an error
// is returned than the return value is undefined.
func (d *Datastore) Update(keyBytes []byte, origData []byte, newData *DataObj) (bool, error) {
	d.updateLock.Lock()
	defer d.updateLock.Unlock()
	if origData != nil {
		val, err := d.db.Get(keyBytes, nil)
		if err != leveldb.ErrNotFound && err != nil {
			return false, err
		}
		if err == leveldb.ErrNotFound && newData == nil {
			// Deleting block already deleted, nothing to
			// worry about.
			log.Debugf("skipping delete of already deleted block %s\n", b58.Encode(keyBytes[1:]))
			return true, nil
		}
		if err == leveldb.ErrNotFound || !bytes.Equal(val, origData) {
			// FIXME: This maybe should at the notice
			// level but there is no "Noticef"!
			log.Infof("skipping update/delete of block %s\n", b58.Encode(keyBytes[1:]))
			return false, nil
		}
	}
	if newData == nil {
		log.Debugf("deleting block %s\n", b58.Encode(keyBytes[1:]))
		return true, d.db.Delete(keyBytes, nil)
	} else {
		data, err := newData.Marshal()
		if err != nil {
			return false, err
		}
		if origData == nil {
			log.Debugf("adding block %s\n", b58.Encode(keyBytes[1:]))
		} else {
			log.Debugf("updating block %s\n", b58.Encode(keyBytes[1:]))
		}
		return true, d.db.Put(keyBytes, data, nil)
	}
}

func (d *Datastore) Get(key ds.Key) (value interface{}, err error) {
	bytes, val, err := d.GetDirect(key)
	if err != nil {
		return nil, err
	}
	return GetData(d, key, bytes, val, d.verify)
}

func (d *Datastore) GetDirect(key ds.Key) ([]byte, *DataObj, error) {
	return d.AsBasic().GetDirect(key)
}

// Get the key as a DataObj
func (d *Basic) GetDirect(key ds.Key) ([]byte, *DataObj, error) {
	val, err := d.db.Get(key.Bytes(), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return nil, nil, ds.ErrNotFound
		}
		return nil, nil, err
	}
	return Decode(val)
}

func Decode(bytes []byte) ([]byte, *DataObj, error) {
	val := new(DataObj)
	err := val.Unmarshal(bytes)
	if err != nil {
		return bytes, nil, err
	}
	return bytes, val, nil
}

type InvalidBlock struct{}

func (e InvalidBlock) Error() string {
	return "filestore: block verification failed"
}

// Verify as much as possible without opening the file, the result is
// a best guess.
func VerifyFast(key ds.Key, val *DataObj) error {
	// There is backing file, nothing to check
	if val.HaveBlockData() {
		return nil
	}

	// block already marked invalid
	if val.Invalid() {
		return InvalidBlock{}
	}

	// get the file's metadata, return on error
	fileInfo, err := os.Stat(val.FilePath)
	if err != nil {
		return err
	}

	// the file has shrunk, the block invalid
	if val.Offset+val.Size > uint64(fileInfo.Size()) {
		return InvalidBlock{}
	}

	// the file mtime has changes, the block is _likely_ invalid
	modtime := FromTime(fileInfo.ModTime())
	if modtime != val.ModTime {
		return InvalidBlock{}
	}

	// the block _seams_ ok
	return nil
}

// Get the orignal data out of the DataObj
func GetData(d *Datastore, key ds.Key, origData []byte, val *DataObj, verify VerifyWhen) ([]byte, error) {
	if val == nil {
		return nil, errors.New("Nil DataObj")
	}

	// If there is no data to get from a backing file then there
	// is nothing more to do so just return the block data
	if val.HaveBlockData() {
		return val.Data, nil
	}

	update := false
	if d != nil {
		update = d.updateOnGet()
	}

	invalid := val.Invalid()

	// Open the file and seek to the correct position
	file, err := os.Open(val.FilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	_, err = file.Seek(int64(val.Offset), 0)
	if err != nil {
		return nil, err
	}

	// Reconstruct the original block, if we get an EOF
	// than the file shrunk and the block is invalid
	data, _, err := Reconstruct(val.Data, file, val.Size)
	reconstructOk := true
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, err
	} else if err != nil {
		log.Debugf("invalid block: %s: %s\n", asMHash(key), err.Error())
		reconstructOk = false
		invalid = true
	}

	// get the new modtime if needed
	modtime := val.ModTime
	if update || verify == VerifyIfChanged {
		fileInfo, err := file.Stat()
		if err != nil {
			return nil, err
		}
		modtime = FromTime(fileInfo.ModTime())
	}

	// Verify the block contents if required
	if reconstructOk && (verify == VerifyAlways || (verify == VerifyIfChanged && modtime != val.ModTime)) {
		log.Debugf("verifying block %s\n", asMHash(key))
		newKey := k.Key(u.Hash(data)).DsKey()
		invalid = newKey != key
	}

	// Update the block if the metadata has changed
	if update && (invalid != val.Invalid() || modtime != val.ModTime) && origData != nil {
		log.Debugf("updating block %s\n", asMHash(key))
		newVal := *val
		newVal.SetInvalid(invalid)
		newVal.ModTime = modtime
		// ignore errors as they are nonfatal
		_, _ = d.Update(key.Bytes(), origData, &newVal)
	}

	// Finally return the result
	if invalid {
		log.Debugf("invalid block %s\n", asMHash(key))
		return nil, InvalidBlock{}
	} else {
		return data, nil
	}
}

func asMHash(dsKey ds.Key) string {
	key, err := k.KeyFromDsKey(dsKey)
	if err != nil {
		return "??????????????????????????????????????????????"
	}
	return key.B58String()
}

func (d *Datastore) Has(key ds.Key) (exists bool, err error) {
	return d.db.Has(key.Bytes(), nil)
}

func (d *Datastore) Delete(key ds.Key) error {
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
	_, err = d.Update(keyBytes, nil, nil)
	return err
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

type Iterator struct {
	key      ds.Key
	keyBytes []byte
	value    *DataObj
	bytes    []byte
	iter     iterator.Iterator
}

var emptyDsKey = ds.NewKey("")

func (d *Basic) NewIterator() *Iterator {
	return &Iterator{iter: d.db.NewIterator(nil, nil)}
}

func (d *Datastore) NewIterator() *Iterator {
	return &Iterator{iter: d.db.NewIterator(nil, nil)}
}

func (itr *Iterator) Next() bool {
	itr.keyBytes = nil
	itr.value = nil
	return itr.iter.Next()
}

func (itr *Iterator) Key() ds.Key {
	if itr.keyBytes != nil {
		return itr.key
	}
	itr.keyBytes = itr.iter.Key()
	itr.key = ds.NewKey(string(itr.keyBytes))
	return itr.key
}

func (itr *Iterator) KeyBytes() []byte {
	itr.Key()
	return itr.keyBytes
}

func (itr *Iterator) Value() ([]byte, *DataObj, error) {
	if itr.value != nil {
		return itr.bytes, itr.value, nil
	}
	itr.bytes = itr.iter.Value()
	if itr.bytes == nil {
		return nil, nil, nil
	}
	var err error
	_, itr.value, err = Decode(itr.bytes)
	return itr.bytes, itr.value, err
}

func (itr *Iterator) Release() {
	itr.iter.Release()
}

func (d *Datastore) Close() error {
	return d.db.Close()
}

func (d *Datastore) Batch() (ds.Batch, error) {
	return ds.NewBasicBatch(d), nil
}

func NoOpLocker() sync.Locker {
	return noopLocker{}
}

type noopLocker struct{}

func (l noopLocker) Lock() {}

func (l noopLocker) Unlock() {}

type addLocker struct {
	adders int
	lock   sync.Mutex
	ds     *Datastore
}

func (l *addLocker) Lock() {
	l.lock.Lock()
	defer l.lock.Unlock()
	if l.adders == 0 {
		l.ds.releaseSnapshot()
		l.ds.snapshot, _ = l.ds.GetSnapshot()
	}
	l.adders += 1
	log.Debugf("acquired add-lock refcnt now %d\n", l.adders)
}

func (l *addLocker) Unlock() {
	l.lock.Lock()
	defer l.lock.Unlock()
	l.adders -= 1
	if l.adders == 0 {
		l.ds.releaseSnapshot()
	}
	log.Debugf("released add-lock refcnt now %d\n", l.adders)
}

func (d *Datastore) AddLocker() sync.Locker {
	return &d.addLocker
}
