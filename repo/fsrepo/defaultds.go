package fsrepo

import (
	"fmt"
	"path"

	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/flatfs"
	levelds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/leveldb"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/measure"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/mount"
	ldbopts "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/syndtr/goleveldb/leveldb/opt"
	repo "github.com/ipfs/go-ipfs/repo"
	config "github.com/ipfs/go-ipfs/repo/config"
	"github.com/ipfs/go-ipfs/thirdparty/dir"
)

const (
	leveldbDirectory = "datastore"
	flatfsDirectory  = "blocks"
)

type defaultDatastore struct {
	repo.Datastore

	// tracked separately for use in Close; do not use directly.
	leveldbDS      repo.Datastore
	metricsBlocks  repo.Datastore
	metricsLevelDB repo.Datastore
}

func openDefaultDatastore(r *FSRepo) (repo.Datastore, error) {
	d := &defaultDatastore{}

	leveldbPath := path.Join(r.path, leveldbDirectory)
	var err error
	// save leveldb reference so it can be neatly closed afterward
	d.leveldbDS, err = levelds.NewDatastore(leveldbPath, &levelds.Options{
		Compression: ldbopts.NoCompression,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to open leveldb datastore: %v", err)
	}

	// 4TB of 256kB objects ~=17M objects, splitting that 256-way
	// leads to ~66k objects per dir, splitting 256*256-way leads to
	// only 256.
	//
	// The keys seen by the block store have predictable prefixes,
	// including "/" from datastore.Key and 2 bytes from multihash. To
	// reach a uniform 256-way split, we need approximately 4 bytes of
	// prefix.
	blocksDS, err := flatfs.New(path.Join(r.path, flatfsDirectory), 4)
	if err != nil {
		return nil, fmt.Errorf("unable to open flatfs datastore: %v", err)
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
	d.metricsBlocks = measure.New(prefix+"blocks", blocksDS)
	d.metricsLevelDB = measure.New(prefix+"leveldb", d.leveldbDS)
	mountDS := mount.New([]mount.Mount{
		{
			Prefix:    ds.NewKey("/blocks"),
			Datastore: d.metricsBlocks,
		},
		{
			Prefix:    ds.NewKey("/"),
			Datastore: d.metricsLevelDB,
		},
	})
	// Make sure it's ok to claim the virtual datastore from mount as
	// threadsafe. There's no clean way to make mount itself provide
	// this information without copy-pasting the code into two
	// variants. This is the same dilemma as the `[].byte` attempt at
	// introducing const types to Go.
	d.Datastore = mountDS

	return d, nil
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

var _ repo.Datastore = (*defaultDatastore)(nil)
