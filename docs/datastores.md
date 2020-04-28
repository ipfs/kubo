# Datastore Configuration Options

This document describes the different possible values for the `Datastore.Spec`
field in the ipfs configuration file.

## flatfs

Stores each key value pair as a file on the filesystem.

The shardFunc is prefixed with `/repo/flatfs/shard/v1` then followed by a descriptor of the sharding strategy. Some example values are:
- `/repo/flatfs/shard/v1/next-to-last/2`
  - Shards on the two next to last characters of the key
- `/repo/flatfs/shard/v1/prefix/2`
  - Shards based on the two character prefix of the key

```json
{
	"type": "flatfs",
	"path": "<relative path within repo for flatfs root>",
	"shardFunc": "<a descriptor of the sharding scheme>",
	"sync": true|false
}
```

NOTE: flatfs must only be used as a block store (mounted at `/blocks`) as it only partially implements the datastore interface. You can mount flatfs for /blocks only using the mount datastore (described below).

## levelds
Uses a leveldb database to store key value pairs.

```json
{
	"type": "levelds",
	"path": "<location of db inside repo>",
	"compression": "none" | "snappy",
}
```

## badgerds

Uses [badger](https://github.com/dgraph-io/badger) as a key value store.

* `syncWrites`: Flush every write to disk before continuing. Setting this to false is safe as go-ipfs will automatically flush writes to disk before and after performing critical operations like pinning. However, you can set this to true to be extra-safe (at the cost of a 2-3x slowdown when adding files).
* `truncate`: Truncate the DB if a partially written sector is found (defaults to true). There is no good reason to set this to false unless you want to manually recover partially written (and unpinned) blocks if go-ipfs crashes half-way through a adding a file.

```json
{
	"type": "badgerds",
	"path": "<location of badger inside repo>",
	"syncWrites": true|false,
	"truncate": true|false,
}
```

## mount

Allows specified datastores to handle keys prefixed with a given path.
The mountpoints are added as keys within the child datastore definitions.

```json
{
	"type": "mount",
	"mounts": [
		{
			// Insert other datastore definition here, but add the following key:
			"mountpoint": "/path/to/handle"
		},
		{
			// Insert other datastore definition here, but add the following key:
			"mountpoint": "/path/to/handle"
		},
	]
}
```

## measure

This datastore is a wrapper that adds metrics tracking to any datastore.

```json
{
	"type": "measure",
	"prefix": "sometag.datastore",
	"child": { datastore being wrapped }
}
```

