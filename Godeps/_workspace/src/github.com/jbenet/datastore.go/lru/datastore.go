package lru

import (
	"errors"

	lru "github.com/hashicorp/golang-lru"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"
)

// Datastore uses golang-lru for internal storage.
type Datastore struct {
	cache *lru.Cache
}

// NewDatastore constructs a new LRU Datastore with given capacity.
func NewDatastore(capacity int) (*Datastore, error) {
	cache, err := lru.New(capacity)
	if err != nil {
		return nil, err
	}

	return &Datastore{cache: cache}, nil
}

// Put stores the object `value` named by `key`.
func (d *Datastore) Put(key ds.Key, value interface{}) (err error) {
	d.cache.Add(key, value)
	return nil
}

// Get retrieves the object `value` named by `key`.
func (d *Datastore) Get(key ds.Key) (value interface{}, err error) {
	val, ok := d.cache.Get(key)
	if !ok {
		return nil, ds.ErrNotFound
	}
	return val, nil
}

// Has returns whether the `key` is mapped to a `value`.
func (d *Datastore) Has(key ds.Key) (exists bool, err error) {
	return ds.GetBackedHas(d, key)
}

// Delete removes the value for given `key`.
func (d *Datastore) Delete(key ds.Key) (err error) {
	d.cache.Remove(key)
	return nil
}

// KeyList returns a list of keys in the datastore
func (d *Datastore) KeyList() ([]ds.Key, error) {
	return nil, errors.New("KeyList not implemented.")
}
