package fs

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"
)

// Datastore uses a standard Go map for internal storage.
type Datastore struct {
	path string
}

// NewDatastore returns a new fs Datastore at given `path`
func NewDatastore(path string) (ds.Datastore, error) {
	if !isDir(path) {
		return nil, fmt.Errorf("Failed to find directory at: %v (file? perms?)", path)
	}

	return &Datastore{path: path}, nil
}

// KeyFilename returns the filename associated with `key`
func (d *Datastore) KeyFilename(key ds.Key) string {
	return filepath.Join(d.path, key.String(), ".dsobject")
}

// Put stores the given value.
func (d *Datastore) Put(key ds.Key, value interface{}) (err error) {

	// TODO: maybe use io.Readers/Writers?
	// r, err := dsio.CastAsReader(value)
	// if err != nil {
	// 	return err
	// }

	val, ok := value.([]byte)
	if !ok {
		return ds.ErrInvalidType
	}

	fn := d.KeyFilename(key)

	// mkdirall above.
	err = os.MkdirAll(filepath.Dir(fn), 0755)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(fn, val, 0666)
}

// Get returns the value for given key
func (d *Datastore) Get(key ds.Key) (value interface{}, err error) {
	fn := d.KeyFilename(key)
	if !isFile(fn) {
		return nil, ds.ErrNotFound
	}

	return ioutil.ReadFile(fn)
}

// Has returns whether the datastore has a value for a given key
func (d *Datastore) Has(key ds.Key) (exists bool, err error) {
	return ds.GetBackedHas(d, key)
}

// Delete removes the value for given key
func (d *Datastore) Delete(key ds.Key) (err error) {
	fn := d.KeyFilename(key)
	if !isFile(fn) {
		return ds.ErrNotFound
	}

	return os.Remove(fn)
}

// KeyList returns a list of all keys in the datastore
func (d *Datastore) KeyList() ([]ds.Key, error) {

	keys := []ds.Key{}

	walkFn := func(path string, info os.FileInfo, err error) error {
		// remove ds path prefix
		if strings.HasPrefix(path, d.path) {
			path = path[len(d.path):]
		}

		if !info.IsDir() {
			key := ds.NewKey(path)
			keys = append(keys, key)
		}
		return nil
	}

	filepath.Walk(d.path, walkFn)
	return keys, nil
}

// isDir returns whether given path is a directory
func isDir(path string) bool {
	finfo, err := os.Stat(path)
	if err != nil {
		return false
	}

	return finfo.IsDir()
}

// isFile returns whether given path is a file
func isFile(path string) bool {
	finfo, err := os.Stat(path)
	if err != nil {
		return false
	}

	return !finfo.IsDir()
}
