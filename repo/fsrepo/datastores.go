package fsrepo

import (
	"encoding/json"
	"fmt"
	"strings"

	repo "github.com/ipfs/go-ipfs/repo"
	config "github.com/ipfs/go-ipfs/repo/config"

	ds "gx/ipfs/QmRWDav6mzWseLWeYfVd5fvUKiVe9xNH29YfMF438fG364/go-datastore"
	mount "gx/ipfs/QmRWDav6mzWseLWeYfVd5fvUKiVe9xNH29YfMF438fG364/go-datastore/syncmount"
	levelds "gx/ipfs/QmaHHmfEozrrotyhyN44omJouyuEtx6ahddqV6W5yRaUSQ/go-ds-leveldb"
	ldb "gx/ipfs/QmbBhyDKsY4mbY6xsKt3qu9Y7FPvMJ6qbD8AMjYYvPRw1g/goleveldb/leveldb/opt"
	"gx/ipfs/Qmbx2KUs8mUbDUiiESzC1ms7mdmh4pRu8X1V1tffC46M4n/go-ds-flatfs"
)

func openDatastore(kind string, params []byte) (repo.Datastore, error) {
	switch kind {
	case "mount":
		var mountmap map[string]*json.RawMessage
		if err := json.Unmarshal(params, &mountmap); err != nil {
			return nil, fmt.Errorf("datastore mount: %v", err)
		}

		return openMountDatastore(mountmap)
	case "flatfs":
		var flatfsparams config.FlatDS
		if err := json.Unmarshal(params, &flatfsparams); err != nil {
			return nil, fmt.Errorf("datastore mount: %v", err)
		}

		return openFlatfsDatastore(&flatfsparams)
	default:
		return nil, fmt.Errorf("unknown datastore type: %s", kind)
	}
}

func openMountDatastore(mountmap map[string]*json.RawMessage) (repo.Datastore, error) {
	var mounts []mount.Mount
	for k, v := range mountmap {
		vals := strings.Split(k, "@")
		if len(vals) != 2 {
			return nil, fmt.Errorf("mount config must be 'type@path'")
		}

		kind := vals[0]
		path := vals[1]

		child, err := openDatastore(kind, []byte(*v))
		if err != nil {
			return nil, err
		}

		mounts = append(mounts, mount.Mount{
			Datastore: child,
			Prefix:    ds.NewKey(path),
		})
	}

	return mount.New(mounts), nil
}

func openFlatfsDatastore(params *config.FlatDS) (repo.Datastore, error) {
	return flatfs.New(params.Path, params.PrefixLen, params.Sync)
}

func openLeveldbDatastore(params *config.LevelDB) (repo.Datastore, error) {
	var compress ldb.Compression
	switch params.Compression {
	case "snappy":
		compress = ldb.SnappyCompression
	case "none":
		compress = ldb.NoCompression
	default:
		compress = ldb.DefaultCompression
	}

	return levelds.NewDatastore(params.Path, &levelds.Options{
		Compression: compress,
	})
}
