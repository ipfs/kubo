package datastore

import (
	"log"

	query "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/query"
)

// Here are some basic datastore implementations.

type keyMap map[Key]interface{}

// MapDatastore uses a standard Go map for internal storage.
type MapDatastore struct {
	values keyMap
}

// NewMapDatastore constructs a MapDatastore
func NewMapDatastore() (d *MapDatastore) {
	return &MapDatastore{
		values: keyMap{},
	}
}

// Put implements Datastore.Put
func (d *MapDatastore) Put(key Key, value interface{}) (err error) {
	d.values[key] = value
	return nil
}

// Get implements Datastore.Get
func (d *MapDatastore) Get(key Key) (value interface{}, err error) {
	val, found := d.values[key]
	if !found {
		return nil, ErrNotFound
	}
	return val, nil
}

// Has implements Datastore.Has
func (d *MapDatastore) Has(key Key) (exists bool, err error) {
	_, found := d.values[key]
	return found, nil
}

// Delete implements Datastore.Delete
func (d *MapDatastore) Delete(key Key) (err error) {
	delete(d.values, key)
	return nil
}

// Query implements Datastore.Query
func (d *MapDatastore) Query(q query.Query) (*query.Results, error) {
	re := make([]query.Entry, 0, len(d.values))
	for k, v := range d.values {
		re = append(re, query.Entry{Key: k.String(), Value: v})
	}
	r := query.ResultsWithEntries(q, re)
	r = q.ApplyTo(r)
	return r, nil
}

// NullDatastore stores nothing, but conforms to the API.
// Useful to test with.
type NullDatastore struct {
}

// NewNullDatastore constructs a null datastoe
func NewNullDatastore() *NullDatastore {
	return &NullDatastore{}
}

// Put implements Datastore.Put
func (d *NullDatastore) Put(key Key, value interface{}) (err error) {
	return nil
}

// Get implements Datastore.Get
func (d *NullDatastore) Get(key Key) (value interface{}, err error) {
	return nil, nil
}

// Has implements Datastore.Has
func (d *NullDatastore) Has(key Key) (exists bool, err error) {
	return false, nil
}

// Delete implements Datastore.Delete
func (d *NullDatastore) Delete(key Key) (err error) {
	return nil
}

// Query implements Datastore.Query
func (d *NullDatastore) Query(q query.Query) (*query.Results, error) {
	return query.ResultsWithEntries(q, nil), nil
}

// LogDatastore logs all accesses through the datastore.
type LogDatastore struct {
	Name  string
	child Datastore
}

// Shim is a datastore which has a child.
type Shim interface {
	Datastore

	Children() []Datastore
}

// NewLogDatastore constructs a log datastore.
func NewLogDatastore(ds Datastore, name string) Shim {
	if len(name) < 1 {
		name = "LogDatastore"
	}
	return &LogDatastore{Name: name, child: ds}
}

// Children implements Shim
func (d *LogDatastore) Children() []Datastore {
	return []Datastore{d.child}
}

// Put implements Datastore.Put
func (d *LogDatastore) Put(key Key, value interface{}) (err error) {
	log.Printf("%s: Put %s\n", d.Name, key)
	// log.Printf("%s: Put %s ```%s```", d.Name, key, value)
	return d.child.Put(key, value)
}

// Get implements Datastore.Get
func (d *LogDatastore) Get(key Key) (value interface{}, err error) {
	log.Printf("%s: Get %s\n", d.Name, key)
	return d.child.Get(key)
}

// Has implements Datastore.Has
func (d *LogDatastore) Has(key Key) (exists bool, err error) {
	log.Printf("%s: Has %s\n", d.Name, key)
	return d.child.Has(key)
}

// Delete implements Datastore.Delete
func (d *LogDatastore) Delete(key Key) (err error) {
	log.Printf("%s: Delete %s\n", d.Name, key)
	return d.child.Delete(key)
}

// Query implements Datastore.Query
func (d *LogDatastore) Query(q query.Query) (*query.Results, error) {
	log.Printf("%s: Query\n", d.Name)
	return d.child.Query(q)
}
