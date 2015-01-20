package component

import (
	"errors"
	"path"
	"path/filepath"
	"sync"

	datastore "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	levelds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/leveldb"
	ldbopts "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/syndtr/goleveldb/leveldb/opt"
	config "github.com/jbenet/go-ipfs/repo/config"
	counter "github.com/jbenet/go-ipfs/repo/fsrepo/counter"
	dir "github.com/jbenet/go-ipfs/repo/fsrepo/dir"
	util "github.com/jbenet/go-ipfs/util"
	ds2 "github.com/jbenet/go-ipfs/util/datastore2"
	debugerror "github.com/jbenet/go-ipfs/util/debugerror"
)

const (
	DefaultDataStoreDirectory = "datastore"
)

var (
	_ Component             = &DatastoreComponent{}
	_ Initializer           = InitDatastoreComponent
	_ InitializationChecker = DatastoreComponentIsInitialized

	dsLock         sync.Mutex // protects openersCounter and datastores
	openersCounter *counter.Openers
	datastores     map[string]ds2.ThreadSafeDatastoreCloser
)

func init() {
	openersCounter = counter.NewOpenersCounter()
	datastores = make(map[string]ds2.ThreadSafeDatastoreCloser)
}

func InitDatastoreComponent(dspath string, conf *config.Config) error {
	// The actual datastore contents are initialized lazily when Opened.
	// During Init, we merely check that the directory is writeable.
	if !filepath.IsAbs(dspath) {
		return debugerror.New("datastore filepath must be absolute") // during initialization (this isn't persisted)
	}
	p := path.Join(dspath, DefaultDataStoreDirectory)
	if err := dir.Writable(p); err != nil {
		return debugerror.Errorf("datastore: %s", err)
	}
	return nil
}

// DatastoreComponentIsInitialized returns true if the datastore dir exists.
func DatastoreComponentIsInitialized(dspath string) bool {
	if !util.FileExists(path.Join(dspath, DefaultDataStoreDirectory)) {
		return false
	}
	return true
}

// DatastoreComponent abstracts the datastore component of the FSRepo.
type DatastoreComponent struct {
	path string                        // required
	ds   ds2.ThreadSafeDatastoreCloser // assigned when repo is opened
}

func (dsc *DatastoreComponent) SetPath(p string) {
	dsc.path = path.Join(p, DefaultDataStoreDirectory)
}

func (dsc *DatastoreComponent) Datastore() datastore.ThreadSafeDatastore { return dsc.ds }

// Open returns an error if the config file is not present.
func (dsc *DatastoreComponent) Open() error {

	dsLock.Lock()
	defer dsLock.Unlock()

	// if no other goroutines have the datastore Open, initialize it and assign
	// it to the package-scoped map for the goroutines that follow.
	if openersCounter.NumOpeners(dsc.path) == 0 {
		ds, err := levelds.NewDatastore(dsc.path, &levelds.Options{
			Compression: ldbopts.NoCompression,
		})
		if err != nil {
			return debugerror.New("unable to open leveldb datastore")
		}
		datastores[dsc.path] = ds
	}

	// get the datastore from the package-scoped map and record self as an
	// opener.
	ds, dsIsPresent := datastores[dsc.path]
	if !dsIsPresent {
		// This indicates a programmer error has occurred.
		return errors.New("datastore should be available, but it isn't")
	}
	dsc.ds = ds
	openersCounter.AddOpener(dsc.path) // only after success
	return nil
}

func (dsc *DatastoreComponent) Close() error {

	dsLock.Lock()
	defer dsLock.Unlock()

	// decrement the Opener count. if this goroutine is the last, also close
	// the underlying datastore (and remove its reference from the map)

	openersCounter.RemoveOpener(dsc.path)

	if openersCounter.NumOpeners(dsc.path) == 0 {
		delete(datastores, dsc.path) // remove the reference
		return dsc.ds.Close()
	}
	return nil
}
