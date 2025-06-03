# Datastore Configuration Options

This document describes the different possible values for the `Datastore.Spec`
field in the ipfs configuration file.

- [flatfs](#flatfs)
- [levelds](#levelds)
- [pebbleds](#pebbleds)
- [badgerds](#badgerds)
- [mount](#mount)
- [measure](#measure)

## flatfs

Stores each key-value pair as a file on the filesystem.

The shardFunc is prefixed with `/repo/flatfs/shard/v1` then followed by a descriptor of the sharding strategy. Some example values are:
- `/repo/flatfs/shard/v1/next-to-last/2`
  - Shards on the two next to last characters of the key
- `/repo/flatfs/shard/v1/prefix/2`
  - Shards based on the two-character prefix of the key

```json
{
	"type": "flatfs",
	"path": "<relative path within repo for flatfs root>",
	"shardFunc": "<a descriptor of the sharding scheme>",
	"sync": true|false
}
```

- `sync`: Flush every write to disk before continuing. Setting this to false is safe as kubo will automatically flush writes to disk before and after performing critical operations like pinning. However, you can set this to true to be extra-safe (at the cost of a slowdown when adding files).

NOTE: flatfs must only be used as a block store (mounted at `/blocks`) as it only partially implements the datastore interface. You can mount flatfs for /blocks only using the mount datastore (described below).

## levelds
Uses a leveldb database to store key-value pairs.

```json
{
	"type": "levelds",
	"path": "<location of db inside repo>",
	"compression": "none" | "snappy",
}
```

## pebbleds

Uses [pebble](https://github.com/cockroachdb/pebble) as a key-value store.

```json
{
	"type": "pebbleds",
	"path": "<location of pebble inside repo>",
}
```

The following options are available for tuning pebble.
If they are not configured (or assigned their zero-valued), then default values are used.

* `bytesPerSync`: int, Sync sstables periodically in order to smooth out writes to disk. (default: 512KB)
* `disableWAL`: true|false, Disable the write-ahead log (WAL) at expense of prohibiting crash recovery. (default: false)
* `cacheSize`: Size of pebble's shared block cache. (default: 8MB)
* `formatVersionMajor`: int, Sets the format of pebble on-disk files. If 0 or unset, automatically convert to latest format.
* `l0CompactionThreshold`: int, Count of L0 files necessary to trigger an L0 compaction.
* `l0StopWritesThreshold`: int, Limit on L0 read-amplification, computed as the number of L0 sublevels.
* `lBaseMaxBytes`: int, Maximum number of bytes for LBase. The base level is the level which L0 is compacted into.
* `maxConcurrentCompactions`: int, Maximum number of concurrent compactions. (default: 1)
* `memTableSize`: int, Size of a MemTable in steady state. The actual MemTable size starts at min(256KB, MemTableSize) and doubles for each subsequent MemTable up to MemTableSize (default: 4MB)
* `memTableStopWritesThreshold`: int, Limit on the number of queued of MemTables. (default: 2)
* `walBytesPerSync`: int: Sets the number of bytes to write to a WAL before calling Sync on it in the background. (default: 0, no background syncing)
* `walMinSyncSeconds`: int: Sets the minimum duration between syncs of the WAL. (default: 0)

> [!TIP]
> Start using pebble with only default values and configure tuning items are needed for your needs. For a more complete description of these values, see: `https://pkg.go.dev/github.com/cockroachdb/pebble@vA.B.C#Options` (where `A.B.C` is pebble version from Kubo's `go.mod`).

Using a pebble datastore can be set when initializing kubo `ipfs init --profile pebbleds`.

#### Use of `formatMajorVersion`

[Pebble's `FormatMajorVersion`](https://github.com/cockroachdb/pebble/tree/master?tab=readme-ov-file#format-major-versions) is a constant controlling the format of persisted data. Backwards incompatible changes to durable formats are gated behind new format major versions.

At any point, a database's format major version may be bumped. However, once a database's format major version is increased, previous versions of Pebble will refuse to open the database.

When IPFS is initialized to use the pebbleds datastore (`ipfs init --profile=pebbleds`), the latest pebble database format is configured in the pebble datastore config as `"formatMajorVersion"`. Setting this in the datastore config prevents automatically upgrading to the latest available version when kubo is upgraded. If a later version becomes available, the kubo daemon prints a startup message to indicate this. The user can them update the config to use the latest format when they are certain a downgrade will not be necessary.

Without the `"formatMajorVersion"` in the pebble datastore config, the database format is automatically upgraded to the latest version. If this happens, then it is possible a downgrade back to the previous version of kubo will not work if new format is not compatible with the pebble datastore in the previous version of kubo.

When installing a new version of kubo when `"formatMajorVersion"` is configured, migration does not upgrade this to the latest available version. This is done because a user may have reasons not to upgrade the pebble database format, and may want to be able to downgrade kubo if something else is not working in the new version. If the configured pebble database format in the old kubo is not supported in the new kubo, then the configured version must be updated and the old kubo run, before installing the new kubo.

## badgerds

Uses [badger](https://github.com/dgraph-io/badger) as a key-value store.

> [!CAUTION]
> This is based on very old badger 1.x, which has known bugs and is no longer supported by the upstream team.
> It is provided here only for pre-existing users, allowing them to migrate away to more modern datastore.
> Do not use it for new deployments, unless you really, really know what you are doing.


* `syncWrites`: Flush every write to disk before continuing. Setting this to false is safe as kubo will automatically flush writes to disk before and after performing critical operations like pinning. However, you can set this to true to be extra-safe (at the cost of a 2-3x slowdown when adding files).
* `truncate`: Truncate the DB if a partially written sector is found (defaults to true). There is no good reason to set this to false unless you want to manually recover partially written (and unpinned) blocks if kubo crashes half-way through a write operation.

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

