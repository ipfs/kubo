package datastore

import "log"

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

// KeyList implements Datastore.KeyList
func (d *MapDatastore) KeyList() ([]Key, error) {
	var keys []Key
	for k := range d.values {
		keys = append(keys, k)
	}
	return keys, nil
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

// KeyList implements Datastore.KeyList
func (d *NullDatastore) KeyList() ([]Key, error) {
	return nil, nil
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

// KeyList implements Datastore.KeyList
func (d *LogDatastore) KeyList() ([]Key, error) {
	log.Printf("%s: Get KeyList\n", d.Name)
	return d.child.KeyList()
}
