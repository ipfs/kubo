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

// ConfigFromMap creates a new datastore config from a map
type ConfigFromMap func(map[string]interface{}) (DatastoreConfig, error)

type DatastoreConfig interface {
	// DiskId is a unique id representing the Datastore config as stored on disk, runtime config values are not
	// part of this Id.  No length limit.
	//DiskId() string

	// Create instantiate a new datastore from this config
	Create(path string) (repo.Datastore, error)
}

var datastores map[string]ConfigFromMap

func init() {
	datastores = map[string]ConfigFromMap{
		"mount":   MountDatastoreConfig,
		"flatfs":  FlatfsDatastoreConfig,
		"levelds": LeveldsDatastoreConfig,
		"mem":     MemDatastoreConfig,
		"log":     LogDatastoreConfig,
		"measure": MeasureDatastoreConfig,
	}
}

func AnyDatastoreConfig(params map[string]interface{}) (DatastoreConfig, error) {
	which, ok := params["type"].(string)
	if !ok {
		return nil, fmt.Errorf("'type' field missing or not a string")
	}
	fun, ok := datastores[which]
	if !ok {
		return nil, fmt.Errorf("unknown datastore type: %s", which)
	}
	return fun(params)
}

type mountDatastoreConfig struct {
	mounts []premount
}

type premount struct {
	ds     DatastoreConfig
	prefix ds.Key
}

func MountDatastoreConfig(params map[string]interface{}) (DatastoreConfig, error) {
	var res mountDatastoreConfig
	mounts, ok := params["mounts"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("'mounts' field is missing or not an array")
	}
	for _, iface := range mounts {
		cfg, ok := iface.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("expected map for mountpoint")
		}

		child, err := AnyDatastoreConfig(cfg)
		if err != nil {
			return nil, err
		}

		prefix, found := cfg["mountpoint"]
		if !found {
			return nil, fmt.Errorf("no 'mountpoint' on mount")
		}

		res.mounts = append(res.mounts, premount{
			ds:     child,
			prefix: ds.NewKey(prefix.(string)),
		})
	}

	return &res, nil
}

func (c *mountDatastoreConfig) Create(path string) (repo.Datastore, error) {
	mounts := make([]mount.Mount, len(c.mounts))
	for i, m := range c.mounts {
		ds, err := m.ds.Create(path)
		if err != nil {
			return nil, err
		}
		mounts[i].Datastore = ds
		mounts[i].Prefix = m.prefix
	}
	return mount.New(mounts), nil
}

type flatfsDatastoreConfig struct {
	path      string
	shardFun  *flatfs.ShardIdV1
	syncField bool
}

func FlatfsDatastoreConfig(params map[string]interface{}) (DatastoreConfig, error) {
	var c flatfsDatastoreConfig
	var ok bool
	var err error

	c.path, ok = params["path"].(string)
	if !ok {
		return nil, fmt.Errorf("'path' field is missing or not boolean")
	}

	sshardFun, ok := params["shardFunc"].(string)
	if !ok {
		return nil, fmt.Errorf("'shardFunc' field is missing or not a string")
	}
	c.shardFun, err = flatfs.ParseShardFunc(sshardFun)
	if err != nil {
		return nil, err
	}

	c.syncField, ok = params["sync"].(bool)
	if !ok {
		return nil, fmt.Errorf("'sync' field is missing or not boolean")
	}
	return &c, nil
}

func (c *flatfsDatastoreConfig) Create(path string) (repo.Datastore, error) {
	p := c.path
	if !filepath.IsAbs(p) {
		p = filepath.Join(path, p)
	}

	return flatfs.CreateOrOpen(p, c.shardFun, c.syncField)
}

type leveldsDatastoreConfig struct {
	path        string
	compression ldbopts.Compression
}

func LeveldsDatastoreConfig(params map[string]interface{}) (DatastoreConfig, error) {
	var c leveldsDatastoreConfig
	var ok bool

	c.path, ok = params["path"].(string)
	if !ok {
		return nil, fmt.Errorf("'path' field is missing or not string")
	}

	switch params["compression"].(string) {
	case "none":
		c.compression = ldbopts.NoCompression
	case "snappy":
		c.compression = ldbopts.SnappyCompression
	case "":
		fallthrough
	default:
		c.compression = ldbopts.DefaultCompression
	}

	return &c, nil
}

func (c *leveldsDatastoreConfig) Create(path string) (repo.Datastore, error) {
	p := c.path
	if !filepath.IsAbs(p) {
		p = filepath.Join(path, p)
	}

	return levelds.NewDatastore(p, &levelds.Options{
		Compression: c.compression,
	})
}

type memDatastoreConfig struct {
	cfg map[string]interface{}
}

func MemDatastoreConfig(params map[string]interface{}) (DatastoreConfig, error) {
	return &memDatastoreConfig{params}, nil
}

func (c *memDatastoreConfig) Create(string) (repo.Datastore, error) {
	return ds.NewMapDatastore(), nil
}

type logDatastoreConfig struct {
	child DatastoreConfig
	name  string
}

func LogDatastoreConfig(params map[string]interface{}) (DatastoreConfig, error) {
	childField, ok := params["child"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("'child' field is missing or not a map")
	}
	child, err := AnyDatastoreConfig(childField)
	if err != nil {
		return nil, err
	}
	name, ok := params["name"].(string)
	if !ok {
		return nil, fmt.Errorf("'name' field was missing or not a string")
	}
	return &logDatastoreConfig{child, name}, nil

}

func (c *logDatastoreConfig) Create(path string) (repo.Datastore, error) {
	child, err := c.child.Create(path)
	if err != nil {
		return nil, err
	}
	return ds.NewLogDatastore(child, c.name), nil
}

type measureDatastoreConfig struct {
	child  DatastoreConfig
	prefix string
}

func MeasureDatastoreConfig(params map[string]interface{}) (DatastoreConfig, error) {
	childField, ok := params["child"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("'child' field is missing or not a map")
	}
	child, err := AnyDatastoreConfig(childField)
	if err != nil {
		return nil, err
	}
	prefix, ok := params["prefix"].(string)
	if !ok {
		return nil, fmt.Errorf("'prefix' field was missing or not a string")
	}
	return &measureDatastoreConfig{child, prefix}, nil
}

func (c measureDatastoreConfig) Create(path string) (repo.Datastore, error) {
	child, err := c.child.Create(path)
	if err != nil {
		return nil, err
	}
	return measure.New(c.prefix, child), nil
}
