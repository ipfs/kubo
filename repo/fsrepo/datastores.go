package fsrepo

import (
	"fmt"
	"path/filepath"

	repo "github.com/ipfs/go-ipfs/repo"

	levelds "gx/ipfs/QmPdvXuXWAR6gtxxqZw42RtSADMwz4ijVmYHGS542b6cMz/go-ds-leveldb"
	measure "gx/ipfs/QmSb95iHExSSb47zpmyn5CyY5PZidVWSjyKyDqgYQrnKor/go-ds-measure"
	flatfs "gx/ipfs/QmUTshC2PP4ZDqkrFfDU4JGJFMWjYnunxPgkQ6ZCA2hGqh/go-ds-flatfs"
	ds "gx/ipfs/QmVSase1JP7cq9QkPT46oNwdp9pT6kBkG3oqS14y3QcZjG/go-datastore"
	mount "gx/ipfs/QmVSase1JP7cq9QkPT46oNwdp9pT6kBkG3oqS14y3QcZjG/go-datastore/syncmount"
	ldbopts "gx/ipfs/QmbBhyDKsY4mbY6xsKt3qu9Y7FPvMJ6qbD8AMjYYvPRw1g/goleveldb/leveldb/opt"
)

func (r *FSRepo) constructDatastore(params map[string]interface{}) (repo.Datastore, error) {
	switch params["type"] {
	case "mount":
		mounts, ok := params["mounts"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("'mounts' field is missing or not an array")
		}

		return r.openMountDatastore(mounts)
	case "flatfs":
		return r.openFlatfsDatastore(params)
	case "mem":
		return ds.NewMapDatastore(), nil
	case "log":
		childField, ok := params["child"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("'child' field is missing or not a map")
		}
		child, err := r.constructDatastore(childField)
		if err != nil {
			return nil, err
		}
		nameField, ok := params["name"].(string)
		if !ok {
			return nil, fmt.Errorf("'name' field was missing or not a string")
		}
		return ds.NewLogDatastore(child, nameField), nil
	case "measure":
		childField, ok := params["child"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("'child' field was missing or not a map")
		}
		child, err := r.constructDatastore(childField)
		if err != nil {
			return nil, err
		}

		prefix, ok := params["prefix"].(string)
		if !ok {
			return nil, fmt.Errorf("'prefix' field was missing or not a string")
		}

		return r.openMeasureDB(prefix, child)

	case "levelds":
		return r.openLeveldbDatastore(params)
	default:
		return nil, fmt.Errorf("unknown datastore type: %s", params["type"])
	}
}

func (r *FSRepo) openMountDatastore(mountcfg []interface{}) (repo.Datastore, error) {
	var mounts []mount.Mount
	for _, iface := range mountcfg {
		cfg := iface.(map[string]interface{})

		child, err := r.constructDatastore(cfg)
		if err != nil {
			return nil, err
		}

		prefix, found := cfg["mountpoint"]
		if !found {
			return nil, fmt.Errorf("no 'mountpoint' on mount")
		}

		mounts = append(mounts, mount.Mount{
			Datastore: child,
			Prefix:    ds.NewKey(prefix.(string)),
		})
	}

	return mount.New(mounts), nil
}

func (r *FSRepo) openFlatfsDatastore(params map[string]interface{}) (repo.Datastore, error) {
	p, ok := params["path"].(string)
	if !ok {
		return nil, fmt.Errorf("'path' field is missing or not boolean")
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(r.path, p)
	}

	sshardFun, ok := params["shardFunc"].(string)
	if !ok {
		return nil, fmt.Errorf("'shardFunc' field is missing or not a string")
	}
	shardFun, err := flatfs.ParseShardFunc(sshardFun)
	if err != nil {
		return nil, err
	}

	syncField, ok := params["sync"].(bool)
	if !ok {
		return nil, fmt.Errorf("'sync' field is missing or not boolean")
	}
	return flatfs.CreateOrOpen(p, shardFun, syncField)
}

func (r *FSRepo) openLeveldbDatastore(params map[string]interface{}) (repo.Datastore, error) {
	p, ok := params["path"].(string)
	if !ok {
		return nil, fmt.Errorf("'path' field is missing or not string")
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(r.path, p)
	}

	var c ldbopts.Compression
	switch params["compression"].(string) {
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
