package filestore

import (
	"errors"
	"io"
	"os"
	"path/filepath"

	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-datastore"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-datastore/query"
	k "github.com/ipfs/go-ipfs/blocks/key"
	//mh "gx/ipfs/QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku/go-multihash"
	u "gx/ipfs/QmZNVWh8LLjAavuQ2JXuFmuYH3C11xo988vSgp7UQrTRj1/go-ipfs-util"
)

type datastore struct {
	ds           ds.Datastore
	alwaysVerify bool
}

func New(d ds.Datastore, fileStorePath string) (ds.Datastore, error) {
	return &datastore{d, true}, nil
}

func (d *datastore) Put(key ds.Key, value interface{}) (err error) {
	val, ok := value.(*DataWOpts)
	if !ok {
		panic(ds.ErrInvalidType)
	}

	addType, ok := val.AddOpts.(int)
	if !ok {
		panic(ds.ErrInvalidType)
	}
	if addType != AddNoCopy {
		return errors.New("Only \"no-copy\" mode supported for now.")
	}

	dataObj, ok := val.DataObj.(*DataObj)
	if !ok {
		panic(ds.ErrInvalidType)
	}

	// Make sure the filename is an absolute path
	if !filepath.IsAbs(dataObj.FilePath) {
		return errors.New("datastore put: non-absolute filename: " + dataObj.FilePath)
	}

	// Make sure we can read the file as a sanity check
	if file, err := os.Open(dataObj.FilePath); err != nil {
		return err
	} else {
		file.Close()
	}

	data, err := dataObj.Marshal()
	if err != nil {
		return err
	}
	return d.ds.Put(key, data)
}

func (d *datastore) Get(key ds.Key) (value interface{}, err error) {
	dataObj, err := d.ds.Get(key)
	if err != nil {
		return nil, err
	}
	data := dataObj.([]byte)
	val := new(DataObj)
	err = val.Unmarshal(data)
	if err != nil {
		return nil, err
	}
	if val.NoBlockData {
		file, err := os.Open(val.FilePath)
		if err != nil {
			return nil, err
		}
		_, err = file.Seek(int64(val.Offset), 0)
		if err != nil {
			return nil, err
		}
		buf := make([]byte, val.Size)
		_, err = io.ReadFull(file, buf)
		if err != nil {
			return nil, err
		}
		data, err := reconstruct(val.Data, buf)
		if err != nil {
			return nil, err
		}
		if d.alwaysVerify {
			newKey := k.Key(u.Hash(data)).DsKey()
			if newKey != key {
				return nil, errors.New("Filestore: Block Verification Failed")
			}
		}
		return data, nil
	} else {
		return val.Data, nil
	}
}

func (d *datastore) Has(key ds.Key) (exists bool, err error) {
	return d.ds.Has(key)
}

func (d *datastore) Delete(key ds.Key) error {
	return ds.ErrNotFound
}

func (d *datastore) Query(q query.Query) (query.Results, error) {
	return nil, errors.New("queries not supported yet")
}

func (d *datastore) Close() error {
	c, ok := d.ds.(io.Closer)
	if ok {
		return c.Close()
	} else {
		return nil
	}
}

func (d *datastore) Batch() (ds.Batch, error) {
	return ds.NewBasicBatch(d), nil
}
