package datastore

import (
	"log"
)

// Here are some basic datastore implementations.

// MapDatastore uses a standard Go map for internal storage.
type keyMap map[Key]interface{}
type MapDatastore struct {
	values keyMap
}

func NewMapDatastore() (d *MapDatastore) {
	return &MapDatastore{
		values: keyMap{},
	}
}

func (d *MapDatastore) Put(key Key, value interface{}) (err error) {
	d.values[key] = value
	return nil
}

func (d *MapDatastore) Get(key Key) (value interface{}, err error) {
	val, found := d.values[key]
	if !found {
		return nil, ErrNotFound
	}
	return val, nil
}

func (d *MapDatastore) Has(key Key) (exists bool, err error) {
	_, found := d.values[key]
	return found, nil
}

func (d *MapDatastore) Delete(key Key) (err error) {
	delete(d.values, key)
	return nil
}

func (d *MapDatastore) KeyList() ([]Key, error) {
	var keys []Key
	for k, _ := range d.values {
		keys = append(keys, k)
	}
	return keys, nil
}

// NullDatastore stores nothing, but conforms to the API.
// Useful to test with.
type NullDatastore struct {
}

func NewNullDatastore() *NullDatastore {
	return &NullDatastore{}
}

func (d *NullDatastore) Put(key Key, value interface{}) (err error) {
	return nil
}

func (d *NullDatastore) Get(key Key) (value interface{}, err error) {
	return nil, nil
}

func (d *NullDatastore) Has(key Key) (exists bool, err error) {
	return false, nil
}

func (d *NullDatastore) Delete(key Key) (err error) {
	return nil
}

func (d *NullDatastore) KeyList() ([]Key, error) {
	return nil, nil
}

// LogDatastore logs all accesses through the datastore.
type LogDatastore struct {
	Name  string
	Child Datastore
}

func NewLogDatastore(ds Datastore, name string) *LogDatastore {
	if len(name) < 1 {
		name = "LogDatastore"
	}
	return &LogDatastore{Name: name, Child: ds}
}

func (d *LogDatastore) Put(key Key, value interface{}) (err error) {
	log.Printf("%s: Put %s", d.Name, key)
	// log.Printf("%s: Put %s ```%s```", d.Name, key, value)
	return d.Child.Put(key, value)
}

func (d *LogDatastore) Get(key Key) (value interface{}, err error) {
	log.Printf("%s: Get %s", d.Name, key)
	return d.Child.Get(key)
}

func (d *LogDatastore) Has(key Key) (exists bool, err error) {
	log.Printf("%s: Has %s", d.Name, key)
	return d.Child.Has(key)
}

func (d *LogDatastore) Delete(key Key) (err error) {
	log.Printf("%s: Delete %s", d.Name, key)
	return d.Child.Delete(key)
}

func (d *LogDatastore) KeyList() ([]Key, error) {
	log.Printf("%s: Get KeyList.", d.Name)
	return d.Child.KeyList()
}
