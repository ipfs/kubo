package fsrepo

import (
	"fmt"
	"io"
	"path"

	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-datastore"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-datastore/flatfs"
	levelds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-datastore/leveldb"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-datastore/measure"
	mount "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-datastore/syncmount"
	ldbopts "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/syndtr/goleveldb/leveldb/opt"
	repo "github.com/ipfs/go-ipfs/repo"
	config "github.com/ipfs/go-ipfs/repo/config"
	"github.com/ipfs/go-ipfs/thirdparty/dir"

	multi "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-datastore/multi"
	filestore "github.com/ipfs/go-ipfs/filestore"
)

const (
	leveldbDirectory = "datastore"
	flatfsDirectory  = "blocks"
	fileStoreDir     = "filestore-db"
	fileStoreDataDir = "filestore-data"
)
const useFileStore = true

var _ = io.EOF

func openDefaultDatastore(r *FSRepo) (repo.Datastore, *filestore.Datastore, error) {
	leveldbPath := path.Join(r.path, leveldbDirectory)

	// save leveldb reference so it can be neatly closed afterward
	leveldbDS, err := levelds.NewDatastore(leveldbPath, &levelds.Options{
		Compression: ldbopts.NoCompression,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("unable to open leveldb datastore: %v", err)
	}

	// 4TB of 256kB objects ~=17M objects, splitting that 256-way
	// leads to ~66k objects per dir, splitting 256*256-way leads to
	// only 256.
	//
	// The keys seen by the block store have predictable prefixes,
	// including "/" from datastore.Key and 2 bytes from multihash. To
	// reach a uniform 256-way split, we need approximately 4 bytes of
	// prefix.
	syncfs := !r.config.Datastore.NoSync
	blocksDS, err := flatfs.New(path.Join(r.path, flatfsDirectory), 4, syncfs)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to open flatfs datastore: %v", err)
	}

	// Add our PeerID to metrics paths to keep them unique
	//
	// As some tests just pass a zero-value Config to fsrepo.Init,
	// cope with missing PeerID.
	id := r.config.Identity.PeerID
	if id == "" {
		// the tests pass in a zero Config; cope with it
		id = fmt.Sprintf("uninitialized_%p", r)
	}
	prefix := "fsrepo." + id + ".datastore."
	metricsBlocks := measure.New(prefix+"blocks", blocksDS)
	metricsLevelDB := measure.New(prefix+"leveldb", leveldbDS)

	var blocksStore ds.Datastore = metricsBlocks

	var fileStore *filestore.Datastore
	if useFileStore {
		fileStorePath := path.Join(r.path, fileStoreDir)
		fileStoreDB, err := levelds.NewDatastore(fileStorePath, &levelds.Options{
			Compression: ldbopts.NoCompression,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("unable to open filestore: %v", err)
		}
		fileStore, _ = filestore.New(fileStoreDB, "")
		//fileStore.(io.Closer).Close()
		blocksStore = multi.New(fileStore, metricsBlocks, nil, nil)
	}

	mountDS := mount.New([]mount.Mount{
		{
			Prefix:    ds.NewKey("/blocks"),
			Datastore: blocksStore,
		},
		{
			Prefix:    ds.NewKey("/"),
			Datastore: metricsLevelDB,
		},
	})

	return mountDS, fileStore, nil
}

func initDefaultDatastore(repoPath string, conf *config.Config) error {
	// The actual datastore contents are initialized lazily when Opened.
	// During Init, we merely check that the directory is writeable.
	leveldbPath := path.Join(repoPath, leveldbDirectory)
	if err := dir.Writable(leveldbPath); err != nil {
		return fmt.Errorf("datastore: %s", err)
	}

	flatfsPath := path.Join(repoPath, flatfsDirectory)
	if err := dir.Writable(flatfsPath); err != nil {
		return fmt.Errorf("datastore: %s", err)
	}
	return nil
}
