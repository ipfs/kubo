package fsrepo

import (
	"fmt"
	"io"
	"path"
	"strings"

	repo "github.com/ipfs/go-ipfs/repo"
	config "github.com/ipfs/go-ipfs/repo/config"
	"github.com/ipfs/go-ipfs/thirdparty/dir"

	multi "github.com/ipfs/go-ipfs/repo/multi"
	filestore "github.com/ipfs/go-ipfs/filestore"
	ds "gx/ipfs/QmZ6A6P6AMo8SR3jXAwzTuSU6B9R2Y4eqW2yW9VvfUayDN/go-datastore"
	"gx/ipfs/QmZ6A6P6AMo8SR3jXAwzTuSU6B9R2Y4eqW2yW9VvfUayDN/go-datastore/flatfs"
	levelds "gx/ipfs/QmZ6A6P6AMo8SR3jXAwzTuSU6B9R2Y4eqW2yW9VvfUayDN/go-datastore/leveldb"
	"gx/ipfs/QmZ6A6P6AMo8SR3jXAwzTuSU6B9R2Y4eqW2yW9VvfUayDN/go-datastore/measure"
	mount "gx/ipfs/QmZ6A6P6AMo8SR3jXAwzTuSU6B9R2Y4eqW2yW9VvfUayDN/go-datastore/syncmount"
	ldbopts "gx/ipfs/QmbBhyDKsY4mbY6xsKt3qu9Y7FPvMJ6qbD8AMjYYvPRw1g/goleveldb/leveldb/opt"
)

const (
	leveldbDirectory = "datastore"
	flatfsDirectory  = "blocks"
	fileStoreDir     = "filestore-db"
	fileStoreDataDir = "filestore-data"
)
const useFileStore = true

var _ = io.EOF

func openDefaultDatastore(r *FSRepo) (repo.Datastore, error) {
	leveldbPath := path.Join(r.path, leveldbDirectory)

	// save leveldb reference so it can be neatly closed afterward
	leveldbDS, err := levelds.NewDatastore(leveldbPath, &levelds.Options{
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
	syncfs := !r.config.Datastore.NoSync
	blocksDS, err := flatfs.New(path.Join(r.path, flatfsDirectory), 4, syncfs)
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
	metricsBlocks := measure.New(prefix+"blocks", blocksDS)
	metricsLevelDB := measure.New(prefix+"leveldb", leveldbDS)

	r.subDss[RepoCache] = metricsBlocks

	var blocksStore ds.Datastore = metricsBlocks

	if useFileStore {
		fileStore, err := r.newFilestore()
		if err != nil {
			return nil, err
		}
		r.subDss[RepoFilestore] = fileStore
		blocksStore = multi.New(metricsBlocks, fileStore)
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

	return mountDS, nil
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

func (r *FSRepo) newFilestore() (*filestore.Datastore, error) {
	fileStorePath := path.Join(r.path, fileStoreDir)
	verify := filestore.VerifyIfChanged
	switch strings.ToLower(r.config.Filestore.Verify) {
	case "never":
		verify = filestore.VerifyNever
	case "":
	case "ifchanged":
	case "if changed":
		verify = filestore.VerifyIfChanged
	case "always":
		verify = filestore.VerifyAlways
	default:
		return nil, fmt.Errorf("invalid value for Filestore.Verify: %s", r.config.Filestore.Verify)
	}
	return filestore.New(fileStorePath, verify)
}
