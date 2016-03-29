package fsrepo

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	repo "github.com/ipfs/go-ipfs/repo"
	config "github.com/ipfs/go-ipfs/repo/config"

	ds "gx/ipfs/QmRWDav6mzWseLWeYfVd5fvUKiVe9xNH29YfMF438fG364/go-datastore"
	mount "gx/ipfs/QmRWDav6mzWseLWeYfVd5fvUKiVe9xNH29YfMF438fG364/go-datastore/syncmount"
	levelds "gx/ipfs/QmaHHmfEozrrotyhyN44omJouyuEtx6ahddqV6W5yRaUSQ/go-ds-leveldb"
	ldbopts "gx/ipfs/QmbBhyDKsY4mbY6xsKt3qu9Y7FPvMJ6qbD8AMjYYvPRw1g/goleveldb/leveldb/opt"
	"gx/ipfs/QmbUSMTQtK9GRrUbD4ngqJwSzHsquUc8nyDubRWp4vPybH/go-ds-measure"
	"gx/ipfs/Qmbx2KUs8mUbDUiiESzC1ms7mdmh4pRu8X1V1tffC46M4n/go-ds-flatfs"
)

func (r *FSRepo) constructDatastore(kind string, params []byte) (repo.Datastore, error) {
	switch kind {
	case "mount":
		var mounts []*mountConfig
		if err := json.Unmarshal(params, &mounts); err != nil {
			return nil, fmt.Errorf("datastore mount: %v", err)
		}

		return r.openMountDatastore(mounts)
	case "flatfs":
		var flatfsparams config.FlatDS
		if err := json.Unmarshal(params, &flatfsparams); err != nil {
			return nil, fmt.Errorf("datastore flatfs: %v", err)
		}

		return r.openFlatfsDatastore(&flatfsparams)
	case "mem":
		return ds.NewMapDatastore(), nil
	case "log":
		var cfg struct {
			Name      string
			ChildType string
			Child     *json.RawMessage
		}

		if err := json.Unmarshal(params, &cfg); err != nil {
			return nil, fmt.Errorf("datastore measure: %v", err)
		}

		child, err := r.constructDatastore(cfg.ChildType, []byte(*cfg.Child))
		if err != nil {
			return nil, err
		}

		return ds.NewLogDatastore(child, cfg.Name), nil

	case "measure":
		var measureOpts struct {
			Prefix    string
			ChildType string
			Child     *json.RawMessage
		}

		if err := json.Unmarshal(params, &measureOpts); err != nil {
			return nil, fmt.Errorf("datastore measure: %v", err)
		}

		child, err := r.constructDatastore(measureOpts.ChildType, []byte(*measureOpts.Child))
		if err != nil {
			return nil, err
		}

		return r.openMeasureDB(measureOpts.Prefix, child)

	case "levelds":
		var c config.LevelDB
		if err := json.Unmarshal(params, &c); err != nil {
			return nil, fmt.Errorf("datastore levelds: %v", err)
		}

		return r.openLeveldbDatastore(&c)

	default:
		return nil, fmt.Errorf("unknown datastore type: %s", kind)
	}
}

type mountConfig struct {
	Path      string
	ChildType string
	Child     *json.RawMessage
}

func (r *FSRepo) openMountDatastore(mountcfg []*mountConfig) (repo.Datastore, error) {
	var mounts []mount.Mount
	for _, cfg := range mountcfg {

		child, err := r.constructDatastore(cfg.ChildType, []byte(*cfg.Child))
		if err != nil {
			return nil, err
		}

		mounts = append(mounts, mount.Mount{
			Datastore: child,
			Prefix:    ds.NewKey(cfg.Path),
		})
	}

	return mount.New(mounts), nil
}

func (r *FSRepo) openFlatfsDatastore(params *config.FlatDS) (repo.Datastore, error) {
	p := params.Path
	if !filepath.IsAbs(p) {
		p = filepath.Join(r.path, p)
	}
	return flatfs.New(p, params.PrefixLen, params.Sync)
}

func (r *FSRepo) openLeveldbDatastore(params *config.LevelDB) (repo.Datastore, error) {
	p := params.Path
	if !filepath.IsAbs(p) {
		p = filepath.Join(r.path, p)
	}

	var c ldbopts.Compression
	switch params.Compression {
	case "none":
		c = ldbopts.NoCompression
	case "snappy":
		c = ldbopts.SnappyCompression
	case "":
		fallthrough
	default:
		c = ldbopts.DefaultCompression
	}
	return levelds.NewDatastore(p, &levelds.Options{
		Compression: c,
	})
}

func (r *FSRepo) openMeasureDB(prefix string, child repo.Datastore) (repo.Datastore, error) {
	return measure.New(prefix, child), nil
}
