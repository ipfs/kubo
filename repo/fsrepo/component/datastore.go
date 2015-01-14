package component

import (
	datastore "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	levelds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/leveldb"
	ldbopts "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/syndtr/goleveldb/leveldb/opt"
	config "github.com/jbenet/go-ipfs/repo/config"
	dir "github.com/jbenet/go-ipfs/repo/fsrepo/dir"
	util "github.com/jbenet/go-ipfs/util"
	ds2 "github.com/jbenet/go-ipfs/util/datastore2"
	debugerror "github.com/jbenet/go-ipfs/util/debugerror"
)

var _ Component = &DatastoreComponent{}
var _ Initializer = InitDatastoreComponent
var _ InitializationChecker = DatastoreComponentIsInitialized

func InitDatastoreComponent(path string, conf *config.Config) error {
	// The actual datastore contents are initialized lazily when Opened.
	// During Init, we merely check that the directory is writeable.
	dspath, err := config.DataStorePath(path)
	if err != nil {
		return err
	}
	if err := dir.Writable(dspath); err != nil {
		return debugerror.Errorf("datastore: %s", err)
	}
	return nil
}

// DatastoreComponentIsInitialized returns true if the datastore dir exists.
func DatastoreComponentIsInitialized(path string) bool {
	dspath, err := config.DataStorePath(path)
	if err != nil {
		return false
	}
	if !util.FileExists(dspath) {
		return false
	}
	return true
}

// DatastoreComponent abstracts the datastore component of the FSRepo.
// NB: create with makeDatastoreComponent function.
type DatastoreComponent struct {
	path string
	ds   ds2.ThreadSafeDatastoreCloser
}

// Open returns an error if the config file is not present.
func (dsc *DatastoreComponent) Open() error {
	ds, err := levelds.NewDatastore(dsc.path, &levelds.Options{
		Compression: ldbopts.NoCompression,
	})
	if err != nil {
		return err
	}
	dsc.ds = ds
	return nil
}

func (dsc *DatastoreComponent) Close() error                             { return dsc.ds.Close() }
func (dsc *DatastoreComponent) SetPath(p string)                         { dsc.path = p }
func (dsc *DatastoreComponent) Datastore() datastore.ThreadSafeDatastore { return dsc.ds }
