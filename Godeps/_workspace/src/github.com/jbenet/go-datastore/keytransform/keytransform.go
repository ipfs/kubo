package keytransform

import (
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	dsq "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/query"
)

type Pair struct {
	Convert KeyMapping
	Invert  KeyMapping
}

func (t *Pair) ConvertKey(k ds.Key) ds.Key {
	return t.Convert(k)
}

func (t *Pair) InvertKey(k ds.Key) ds.Key {
	return t.Invert(k)
}

// ktds keeps a KeyTransform function
type ktds struct {
	child ds.Datastore

	KeyTransform
}

// Children implements ds.Shim
func (d *ktds) Children() []ds.Datastore {
	return []ds.Datastore{d.child}
}

// Put stores the given value, transforming the key first.
func (d *ktds) Put(key ds.Key, value interface{}) (err error) {
	return d.child.Put(d.ConvertKey(key), value)
}

// Get returns the value for given key, transforming the key first.
func (d *ktds) Get(key ds.Key) (value interface{}, err error) {
	return d.child.Get(d.ConvertKey(key))
}

// Has returns whether the datastore has a value for a given key, transforming
// the key first.
func (d *ktds) Has(key ds.Key) (exists bool, err error) {
	return d.child.Has(d.ConvertKey(key))
}

// Delete removes the value for given key
func (d *ktds) Delete(key ds.Key) (err error) {
	return d.child.Delete(d.ConvertKey(key))
}

// Query implements Query, inverting keys on the way back out.
func (d *ktds) Query(q dsq.Query) (*dsq.Results, error) {

	q2 := q
	q2.Prefix = d.ConvertKey(ds.NewKey(q2.Prefix)).String()
	r, err := d.child.Query(q2)
	if err != nil {
		return nil, err
	}

	ch := make(chan dsq.Entry)
	go func() {
		for e := range r.Entries() {
			e.Key = d.InvertKey(ds.NewKey(e.Key)).String()
			ch <- e
		}
		close(ch)
	}()

	r2 := dsq.ResultsWithEntriesChan(q, ch)
	return r2, nil
}
