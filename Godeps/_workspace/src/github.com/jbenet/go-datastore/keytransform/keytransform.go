package keytransform

import ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"

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

// KeyList returns a list of all keys in the datastore, transforming keys out.
func (d *ktds) KeyList() ([]ds.Key, error) {

	keys, err := d.child.KeyList()
	if err != nil {
		return nil, err
	}

	for i, k := range keys {
		keys[i] = d.InvertKey(k)
	}
	return keys, nil
}
