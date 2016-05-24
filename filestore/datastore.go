package filestore

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	//"runtime/debug"
	//"bytes"
	//"time"

	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-datastore"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-datastore/query"
	k "github.com/ipfs/go-ipfs/blocks/key"
	//mh "gx/ipfs/QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku/go-multihash"
	b58 "gx/ipfs/QmT8rehPR3F6bmwL6zjUN8XpiDBFFpMP2myPdC6ApsWfJf/go-base58"
	u "gx/ipfs/QmZNVWh8LLjAavuQ2JXuFmuYH3C11xo988vSgp7UQrTRj1/go-ipfs-util"
	logging "gx/ipfs/QmaDNZ4QMdBdku1YZWBysufYyoQt1negQGNav6PLYarbY8/go-log"
)

var log = logging.Logger("filestore")

const (
	VerifyNever     = 0
	VerifyIfChanged = 1
	VerifyAlways    = 2
)

type Datastore struct {
	ds     ds.Datastore
	verify int
}

func New(d ds.Datastore, fileStorePath string, verify int) (*Datastore, error) {
	return &Datastore{d, verify}, nil
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

	file.Close()

	return d.PutDirect(key, dataObj)
}

func (d *Datastore) PutDirect(key ds.Key, dataObj *DataObj) (err error) {
	data, err := dataObj.Marshal()
	if err != nil {
		return err
	}
	log.Debugf("adding block %s\n", b58.Encode(key.Bytes()[1:]))
	return d.ds.Put(key, data)
}

func (d *Datastore) Get(key ds.Key) (value interface{}, err error) {
	dataObj, err := d.ds.Get(key)
	if err != nil {
		return nil, err
	}
	val, err := d.decode(dataObj)
	if err != nil {
		return nil, err
	}
	return d.GetData(key, val, d.verify, true)
}

// Get the key as a DataObj
func (d *Datastore) GetDirect(key ds.Key) (*DataObj, error) {
	dataObj, err := d.ds.Get(key)
	if err != nil {
		return nil, err
	}
	return d.decode(dataObj)
}

func (d *Datastore) decode(dataObj interface{}) (*DataObj, error) {
	data := dataObj.([]byte)
	val := new(DataObj)
	err := val.Unmarshal(data)
	if err != nil {
		return nil, err
	}
	return val, nil
}

type InvalidBlock struct{}

func (e InvalidBlock) Error() string {
	return "Datastore: Block Verification Failed"
}

const useFastReconstruct = true

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
		_, err = file.Seek(int64(val.Offset), 0)
		if err != nil {
			return nil, err
		}
		var data []byte
		if useFastReconstruct {
			data, err = reconstructDirect(val.Data, file, val.Size)
		} else {
			buf := make([]byte, val.Size)
			_, err = io.ReadFull(file, buf)
			if err != nil {
				return nil, err
			}
			data, err = reconstruct(val.Data, buf)
		}
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
	return d.ds.Has(key)
}

func (d *Datastore) Delete(key ds.Key) error {
	return ds.ErrNotFound
}

func (d *Datastore) DeleteDirect(key ds.Key) error {
	return d.ds.Delete(key)
}

func (d *Datastore) Query(q query.Query) (query.Results, error) {
	res, err := d.ds.Query(q)
	if err != nil {
		return nil, err
	}
	if q.KeysOnly {
		return res, nil
	}
	return nil, errors.New("filestore currently only supports keyonly queries")
	// return &queryResult{res, func(r query.Result) query.Result {
	// 	val, err := d.decode(r.Value)
	// 	if err != nil {
	// 		return query.Result{query.Entry{r.Key, nil}, err}
	// 	}
	// 	// Note: It should not be necessary to reclean the key
	// 	// here (by calling ds.NewKey) just to convert the
	// 	// string back to a ds.Key
	// 	data, err := d.GetData(ds.NewKey(r.Key), val, d.alwaysVerify)
	// 	if err != nil {
	// 		return query.Result{query.Entry{r.Key, nil}, err}
	// 	}
	// 	return query.Result{query.Entry{r.Key, data}, r.Error}
	// }}, nil
}

func (d *Datastore) QueryDirect(q query.Query) (query.Results, error) {
	res, err := d.ds.Query(q)
	if err != nil {
		return nil, err
	}
	if q.KeysOnly {
		return res, nil
	}
	return nil, errors.New("filestore currently only supports keyonly queries")
	// return &queryResult{res, func(r query.Result) query.Result {
	// 	val, err := d.decode(r.Value)
	// 	if err != nil {
	// 		return query.Result{query.Entry{r.Key, nil}, err}
	// 	}
	// 	return query.Result{query.Entry{r.Key, val}, r.Error}
	// }}, nil
}

// type queryResult struct {
// 	query.Results
// 	adjResult func(query.Result) query.Result
// }

// func (q *queryResult) Next() <-chan query.Result {
// 	in := q.Results.Next()
// 	out := make(chan query.Result)
// 	go func() {
// 		res := <-in
// 		if res.Error == nil {
// 			out <- res
// 		}
// 		out <- q.adjResult(res)
// 	}()
// 	return out
// }

// func (q *queryResult) Rest() ([]query.Entry, error) {
// 	res, err := q.Results.Rest()
// 	if err != nil {
// 		return nil, err
// 	}
// 	for _, entry := range res {
// 		newRes := q.adjResult(query.Result{entry, nil})
// 		if newRes.Error != nil {
// 			return nil, newRes.Error
// 		}
// 		entry.Value = newRes.Value
// 	}
// 	return res, nil
// }

func (d *Datastore) Close() error {
	c, ok := d.ds.(io.Closer)
	if ok {
		return c.Close()
	} else {
		return nil
	}
}

func (d *Datastore) Batch() (ds.Batch, error) {
	return ds.NewBasicBatch(d), nil
}
