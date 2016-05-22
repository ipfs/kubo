// A very simple multi-datastore that analogous to unionfs
// Put and Del only go to the first datastore
// All others are considered readonly
package multi

import (
	"errors"
	"io"

	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-datastore"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-datastore/query"
)

var (
	ErrNoMount = errors.New("no datastore mounted for this key")
)

func New(dss ...ds.Datastore) *Datastore {
	return &Datastore{dss}
}

type Datastore struct {
	dss []ds.Datastore
}

func (d *Datastore) Put(key ds.Key, value interface{}) error {
	return d.dss[0].Put(key, value)
}

func (d *Datastore) Get(key ds.Key) (value interface{}, err error) {
	for _, d0 := range d.dss {
		value, err = d0.Get(key)
		if err == nil || err != ds.ErrNotFound {
			return
		}
	}
	return nil, ds.ErrNotFound
}

func (d *Datastore) Has(key ds.Key) (exists bool, err error) {
	for _, d0 := range d.dss {
		exists, err = d0.Has(key)
		if exists && err == nil {
			return
		}
	}
	return false, err
}

func (d *Datastore) Delete(key ds.Key) error {
	return d.dss[0].Delete(key)
}

// FIXME: Should Query each datastore in term and combine the results
func (d *Datastore) Query(q query.Query) (query.Results, error) {
	if len(q.Filters) > 0 ||
		len(q.Orders) > 0 ||
		q.Limit > 0 ||
		q.Offset > 0 ||
		q.Prefix != "/" {
		// TODO this is overly simplistic, but the only caller is
		// `ipfs refs local` for now, and this gets us moving.
		return nil, errors.New("multi only supports listing all keys in random order")
	}

	return d.dss[0].Query(q)
}

func (d *Datastore) Close() error {
	var err error = nil
	for _, d0 := range d.dss {
		c, ok := d0.(io.Closer)
		if !ok {
			continue
		}
		err0 := c.Close()
		if err0 != nil {
			err = err0
		}
	}
	return err
}

func (d *Datastore) Batch() (ds.Batch, error) {
	b, ok := d.dss[0].(ds.Batching)
	if ok {
		return b.Batch()
	} else {
		return nil, ds.ErrBatchUnsupported
	}
}

