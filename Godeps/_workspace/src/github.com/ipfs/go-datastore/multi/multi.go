// Package mount provides a Datastore that has other Datastores
// mounted at various key prefixes and is threadsafe
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

// Note: The advance datastore is at index 0 so that it is searched first in Get and Has

func New(adv ds.Datastore, normal ds.Datastore, aux []ds.Datastore, roAux []ds.Datastore) *Datastore {
	d := new(Datastore)

	if adv == nil {
		d.normalDSIdx = 0
		d.advanceDSIdx = 0
	} else {
		d.normalDSIdx = 1
		d.advanceDSIdx = 0
	}

	advC := 0
	if adv != nil {
		advC = 1
	}
	d.dss = make([]ds.Datastore, advC+1+len(aux)+len(roAux))
	d.mut = make([]PutDelete, advC+1+len(aux))

	i := 0
	if adv != nil {
		d.dss[i] = adv
		d.mut[i] = adv
		i += 1
	}

	d.dss[i] = normal
	d.mut[i] = normal
	i += 1

	for _, a := range aux {
		d.dss[i] = a
		d.mut[i] = a
		i += 1
	}

	for _, a := range roAux {
		d.dss[i] = a
		i += 1
	}

	return d
}

type params struct {
	normalDSIdx  int
	advanceDSIdx int
}

type Datastore struct {
	params
	dss []ds.Datastore
	mut []PutDelete
}

type PutDelete interface {
	Put(key ds.Key, val interface{}) error
	Delete(key ds.Key) error
}

func (d *Datastore) Put(key ds.Key, value interface{}) error {
	return d.put(d.mut, key, value)
}

func (p *params) put(dss []PutDelete, key ds.Key, value interface{}) error {
	if _, ok := value.([]byte); ok {
		//println("Add Simple")
		return dss[p.normalDSIdx].Put(key, value)
	}
	//println("Add Advance")
	return dss[p.advanceDSIdx].Put(key, value)
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
	return d.delete(d.mut, key)
}

func (d *params) delete(dss []PutDelete, key ds.Key) error {
	var err error = nil
	count := 0
	// always iterate over all datastores to be sure all instances
	// of Key are deleted
	for _, d0 := range dss {
		err0 := d0.Delete(key)
		if err0 == nil {
			count += 1
		} else if err0 != ds.ErrNotFound {
			err = err0
		}
	}
	if err != nil {
		return err
	} else if count == 0 {
		return ds.ErrNotFound
	} else {
		return nil
	}
}

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

	return d.dss[d.normalDSIdx].Query(q)
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

type multiBatch struct {
	params *params
	dss    []PutDelete
}

func (d *Datastore) Batch() (ds.Batch, error) {
	dss := make([]PutDelete, len(d.dss))
	for i, d0 := range d.dss {
		b, ok := d0.(ds.Batching)
		if !ok {
			return nil, ds.ErrBatchUnsupported
		}
		res, err := b.Batch()
		if err != nil {
			return nil, err
		}
		dss[i] = res
	}
	return &multiBatch{&d.params, dss}, nil
}

func (mt *multiBatch) Put(key ds.Key, val interface{}) error {
	return mt.params.put(mt.dss, key, val)
}

func (mt *multiBatch) Delete(key ds.Key) error {
	return mt.params.delete(mt.dss, key)
}

func (mt *multiBatch) Commit() error {
	var err error = nil
	for _, b0 := range mt.dss {
		err0 := b0.(ds.Batch).Commit()
		if err0 != nil {
			err = err0
		}
	}
	return err
}
