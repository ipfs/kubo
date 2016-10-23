package filestore

import (
	//"runtime/debug"
	//"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"

	"gx/ipfs/QmSF8fPo3jgVBAy8fpdjjYqgG87dkJgUprRBHRd2tmfgpP/goprocess"
	logging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
	"gx/ipfs/QmbBhyDKsY4mbY6xsKt3qu9Y7FPvMJ6qbD8AMjYYvPRw1g/goleveldb/leveldb"
	"gx/ipfs/QmbBhyDKsY4mbY6xsKt3qu9Y7FPvMJ6qbD8AMjYYvPRw1g/goleveldb/leveldb/opt"
	"gx/ipfs/QmbBhyDKsY4mbY6xsKt3qu9Y7FPvMJ6qbD8AMjYYvPRw1g/goleveldb/leveldb/util"
	ds "gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore"
	"gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore/query"
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

type Datastore struct {
	db     dbwrap
	verify VerifyWhen

	// updateLock should be held whenever updating the database.  It
	// is designed to only be held for a very short period of time and
	// should not be held when doing potentially expensive operations
	// such as computing a hash or any sort of I/O.
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

type Basic struct {
	db dbread
	ds *Datastore
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

func New(path string, verify VerifyWhen, noCompression bool) (*Datastore, error) {
	dbOpts := &opt.Options{ErrorIfMissing: true}
	if noCompression {
		dbOpts.Compression = opt.NoCompression
	}
	db, err := leveldb.OpenFile(path, dbOpts)
	if err != nil {
		return nil, err
	}
	ds := &Datastore{db: dbwrap{dbread{db}, db}, verify: verify}
	ds.addLocker.ds = ds
	return ds, nil
}

func (d *Datastore) Put(key ds.Key, value interface{}) error {
	dataObj, ok := value.(*DataObj)
	if !ok {
		return ds.ErrInvalidType
	}

	if dataObj.FilePath == "" && dataObj.Size == 0 {
		// special case to handle empty files
		d.updateLock.Lock()
		defer d.updateLock.Unlock()
		return d.db.Put(HashToKey(key.String()), dataObj)
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

	hash := HashToKey(key.String())

	d.updateLock.Lock()
	defer d.updateLock.Unlock()
	return d.db.Put(hash, dataObj)

	//dbKey := NewDbKey(key.String(), dataObj.FilePath, dataObj.Offset, nil)

	//
	// if d.db.Have(hash)

	// foundKey, _, err := d.GetDirect(dbKey)
	// if err != nil && err != ds.ErrNotFound {
	// 	return err
	// }

	// // File already stored, just update the value
	// if err == nil {
	// 	return d.db.Put(foundKey, dataObj)
	// }

	// // File not stored
	// if

	// Check if entry already exists
}

// Might modify the DataObj
// func (d *Datastore) PutDataObj(key Key, dataObj *DataObj) error {
// 	if key.FilePath != "" {
// 		if key.Offset == -1 {
// 			//erro
// 		}
// 		dataObj.FilePath = ""
// 		dataObj.Offset = 0
// 	}
// 	// now store normally
// }

// func (d *Datastore) UpdateGood(key Key, dataObj *DataObj) {
// 	d.updateLock.Lock()
// 	defer d.updateLock.Unlock()
// 	_,bad,err := GetHash(key.Hash)
// 	if err == nil {
// 		return
// 	}
// 	badKey := Key{key.Hash, bad.FilePath, bad.Offset}
// 	_,good,err := GetKey(key)
// 	if err == nil {
// 		return
// 	}
// 	// FIXME: Use batching here
// 	Put(Key{key.Hash,"",-1}, good)
// 	Put(badKey, bad)
// 	d.db.Delete(key.String())
// }

func (d *Datastore) Get(dsKey ds.Key) (value interface{}, err error) {
	key := NewDbKey(dsKey.String(), "", -1, nil)
	_, val, err := d.GetDirect(key)
	if err != nil {
		return nil, err
	}
	data, err := GetData(d, key, val, d.verify)
	if err == nil {
		return data, nil
	}
	if err != InvalidBlock {
		return nil, err
	}

	return nil, err
	// The block failed to validate, check for other blocks

	//UpdateGood(fsKey, dataObj) // ignore errors

	//return val, nil
}

func (d *Datastore) GetDirect(key *DbKey) (*DbKey, *DataObj, error) {
	return d.AsBasic().GetDirect(key)
}

// Get the key as a DataObj. To handle multiple DataObj per Hash a
// block can be retrieved by either by just the hash or the hash
// combined with filename and offset.
//
// In addition to the date GteDirect will return the key the block was
// found under.
func (d *Basic) GetDirect(key *DbKey) (*DbKey, *DataObj, error) {
	if string(key.Bytes) != key.String() {
		panic(string(key.Bytes) + " != " + key.String())
	}
	val, err := d.db.Get(key)
	if err != leveldb.ErrNotFound { // includes the case when err == nil
		return key, val, err
	}

	if key.FilePath == "" {
		return nil, nil, ds.ErrNotFound
	}

	hash := HashToKey(key.Hash)
	return d.getIndirect(hash, key)
}

// We have a key with filename and offset that was not found directly.
// Check to see it it was stored just using the hash.
func (d *Basic) getIndirect(hash *DbKey, key *DbKey) (*DbKey, *DataObj, error) {
	val, err := d.db.GetHash(hash.Bytes)
	if err == leveldb.ErrNotFound {
		return nil, nil, ds.ErrNotFound
	} else if err != nil {
		return nil, nil, err
	}

	if key.FilePath != val.FilePath || uint64(key.Offset) != val.Offset {
		return nil, nil, ds.ErrNotFound
	}

	return hash, val, nil
}

func (d *Datastore) DelDirect(key *DbKey) error {
	if key.FilePath == "" {
		return errors.New("Cannot delete with hash only key")
	}
	d.updateLock.Lock()
	defer d.updateLock.Unlock()
	found, err := d.db.Has(key)
	if err != nil {
		return err
	}
	if found {
		return d.db.Delete(key.Bytes)
	}
	hash := NewDbKey(key.Hash, "", -1, nil)
	_, _, err = d.AsBasic().getIndirect(hash, key)
	if err != nil {
		return err
	}
	return d.db.Delete(hash.Bytes)
}

func (d *Datastore) Update(key *DbKey, val *DataObj) {
	if key.FilePath == "" {
		key = NewDbKey(key.Hash, val.FilePath, int64(val.Offset), nil)
	}
	d.updateLock.Lock()
	defer d.updateLock.Unlock()
	foundKey, _, err := d.GetDirect(key)
	if err != nil {
		return
	}
	d.db.Put(foundKey, val)
}

var InvalidBlock = errors.New("filestore: block verification failed")

// Verify as much as possible without opening the file, the result is
// a best guess.
func VerifyFast(val *DataObj) error {
	// There is backing file, nothing to check
	if val.HaveBlockData() {
		return nil
	}

	// block already marked invalid
	if val.Invalid() {
		return InvalidBlock
	}

	// get the file's metadata, return on error
	fileInfo, err := os.Stat(val.FilePath)
	if err != nil {
		return err
	}

	// the file has shrunk, the block invalid
	if val.Offset+val.Size > uint64(fileInfo.Size()) {
		return InvalidBlock
	}

	// the file mtime has changes, the block is _likely_ invalid
	modtime := FromTime(fileInfo.ModTime())
	if modtime != val.ModTime {
		return InvalidBlock
	}

	// the block _seams_ ok
	return nil
}

// Get the orignal data out of the DataObj
func GetData(d *Datastore, key *DbKey, val *DataObj, verify VerifyWhen) ([]byte, error) {
	if val == nil {
		return nil, errors.New("Nil DataObj")
	}

	// If there is no data to get from a backing file then there
	// is nothing more to do so just return the block data
	if val.HaveBlockData() {
		return val.Data, nil
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
		log.Debugf("invalid block: %s: %s\n", MHash(key), err.Error())
		reconstructOk = false
		invalid = true
	}

	if verify == VerifyNever {
		if invalid {
			return nil, InvalidBlock
		} else {
			return data, nil
		}
	}

	// get the new modtime
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	modtime := FromTime(fileInfo.ModTime())

	// Verify the block contents if required
	if reconstructOk && (verify == VerifyAlways || modtime != val.ModTime) {
		log.Debugf("verifying block %s\n", MHash(key))
		origKey, _ := key.Cid()
		newKey, _ := origKey.Prefix().Sum(data)
		invalid = !origKey.Equals(newKey)
	}

	// Update the block if the metadata has changed
	if invalid != val.Invalid() || modtime != val.ModTime {
		log.Debugf("updating block %s\n", MHash(key))
		newVal := *val
		newVal.SetInvalid(invalid)
		newVal.ModTime = modtime
		// ignore errors as they are nonfatal
		d.Update(key, &newVal)
	}

	// Finally return the result
	if invalid {
		log.Debugf("invalid block %s\n", MHash(key))
		return nil, InvalidBlock
	} else {
		return data, nil
	}
}

func (d *Datastore) Has(key ds.Key) (exists bool, err error) {
	// FIXME: This is too simple
	return d.db.HasHash(key.Bytes())
}

func (d *Datastore) Delete(key ds.Key) error {
	d.updateLock.Lock()
	defer d.updateLock.Unlock()
	return d.db.Delete(key.Bytes())
	//return errors.New("Deleting filestore blocks by hash only is unsupported.")
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
		i := d.db.db.NewIterator(rnge, nil)
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
	return d.db.db.Close()
}

func (d *Datastore) Batch() (ds.Batch, error) {
	return ds.NewBasicBatch(d), nil
}
