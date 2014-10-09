package keytransform

import ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"

// KeyTransform is a function that transforms one key into another.
type KeyTransform func(ds.Key) ds.Key

// Datastore is a keytransform.Datastore
type Datastore interface {
	ds.Shim

	// Transform runs the transformation function
	Transform(ds.Key) ds.Key

	// TransformFunc returns the KeyTransform function
	TransformFunc() KeyTransform
}

// ktds keeps a KeyTransform function
type ktds struct {
	child ds.Datastore
	xform KeyTransform
}

// WrapDatastore wraps a given datastore with a KeyTransform function.
// The resulting wrapped datastore will use the transform on all Datastore
// operations.
func WrapDatastore(child ds.Datastore, f KeyTransform) Datastore {
	if f == nil {
		panic("f (KeyTransform) is nil")
	}

	if child == nil {
		panic("child (ds.Datastore) is nil")
	}

	return &ktds{child, f}
}

// TransformFunc returns the KeyTransform function
func (d *ktds) TransformFunc() KeyTransform {
	return d.xform
}

// Transform runs the KeyTransform function
func (d *ktds) Transform(k ds.Key) ds.Key {
	return d.xform(k)
}

// Children implements ds.Shim
func (d *ktds) Children() []ds.Datastore {
	return []ds.Datastore{d.child}
}

// Put stores the given value, transforming the key first.
func (d *ktds) Put(key ds.Key, value interface{}) (err error) {
	return d.child.Put(d.Transform(key), value)
}

// Get returns the value for given key, transforming the key first.
func (d *ktds) Get(key ds.Key) (value interface{}, err error) {
	return d.child.Get(d.Transform(key))
}

// Has returns whether the datastore has a value for a given key, transforming
// the key first.
func (d *ktds) Has(key ds.Key) (exists bool, err error) {
	return d.child.Has(d.Transform(key))
}

// Delete removes the value for given key
func (d *ktds) Delete(key ds.Key) (err error) {
	return d.child.Delete(d.Transform(key))
}

// KeyList returns a list of all keys in the datastore, transforming keys out.
func (d *ktds) KeyList() ([]ds.Key, error) {

	keys, err := d.child.KeyList()
	if err != nil {
		return nil, err
	}

	for i, k := range keys {
		keys[i] = d.Transform(k)
	}
	return keys, nil
}
