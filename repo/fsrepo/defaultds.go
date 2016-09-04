package fsrepo

import (
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	repo "github.com/ipfs/go-ipfs/repo"
	config "github.com/ipfs/go-ipfs/repo/config"
	"github.com/ipfs/go-ipfs/thirdparty/dir"

	filestore "github.com/ipfs/go-ipfs/filestore"
	//multi "github.com/ipfs/go-ipfs/repo/multi"
	ds "gx/ipfs/QmTxLSvdhwg68WJimdS6icLPhZi28aTp6b7uihC2Yb47Xk/go-datastore"
	"gx/ipfs/QmTxLSvdhwg68WJimdS6icLPhZi28aTp6b7uihC2Yb47Xk/go-datastore/flatfs"
	levelds "gx/ipfs/QmTxLSvdhwg68WJimdS6icLPhZi28aTp6b7uihC2Yb47Xk/go-datastore/leveldb"
	"gx/ipfs/QmTxLSvdhwg68WJimdS6icLPhZi28aTp6b7uihC2Yb47Xk/go-datastore/measure"
	mount "gx/ipfs/QmTxLSvdhwg68WJimdS6icLPhZi28aTp6b7uihC2Yb47Xk/go-datastore/syncmount"
	ldbopts "gx/ipfs/QmbBhyDKsY4mbY6xsKt3qu9Y7FPvMJ6qbD8AMjYYvPRw1g/goleveldb/leveldb/opt"
)

const (
	leveldbDirectory = "datastore"
	flatfsDirectory  = "blocks"
	fileStoreDir     = "filestore-db"
	fileStoreDataDir = "filestore-data"
)

const (
	RootMount      = "/"
	CacheMount     = "/blocks" // needs to be the same as blockstore.DefaultPrefix
	FilestoreMount = "/filestore"
)

var _ = io.EOF

func openDefaultDatastore(r *FSRepo) (repo.Datastore, []Mount, error) {
	leveldbPath := path.Join(r.path, leveldbDirectory)

	// save leveldb reference so it can be neatly closed afterward
	leveldbDS, err := levelds.NewDatastore(leveldbPath, &levelds.Options{
		Compression: ldbopts.NoCompression,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("unable to open leveldb datastore: %v", err)
	}

	syncfs := !r.config.Datastore.NoSync
	// 5 bytes of prefix gives us 25 bits of freedom, 16 of which are taken by
	// by the Qm prefix. Leaving us with 9 bits, or 512 way sharding
	blocksDS, err := flatfs.New(path.Join(r.path, flatfsDirectory), 5, syncfs)
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

	var mounts []mount.Mount
	var directMounts []Mount

	mounts = append(mounts, mount.Mount{
		Prefix:    ds.NewKey(CacheMount),
		Datastore: metricsBlocks,
	})
	directMounts = append(directMounts, Mount{CacheMount, blocksDS})

	fileStore, err := r.newFilestore()
	if err != nil {
		return nil, nil, err
	}
	if fileStore != nil {
		mounts = append(mounts, mount.Mount{
			Prefix:    ds.NewKey(FilestoreMount),
			Datastore: fileStore,
		})
		directMounts = append(directMounts, Mount{FilestoreMount, fileStore})
	}

	mounts = append(mounts, mount.Mount{
		Prefix:    ds.NewKey(RootMount),
		Datastore: metricsLevelDB,
	})
	directMounts = append(directMounts, Mount{RootMount, leveldbDS})

	mountDS := mount.New(mounts)

	return mountDS, directMounts, nil
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

func InitFilestore(repoPath string) error {
	fileStorePath := path.Join(repoPath, fileStoreDir)
	return filestore.Init(fileStorePath)
}

// will return nil, nil if the filestore is not enabled
func (r *FSRepo) newFilestore() (*filestore.Datastore, error) {
	fileStorePath := path.Join(r.path, fileStoreDir)
	if _, err := os.Stat(fileStorePath); os.IsNotExist(err) {
		return nil, nil
	}
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
