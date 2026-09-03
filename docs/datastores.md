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

The shardFunc is prefixed with `/repo/flatfs/shard/v1` then followed by a descriptor of the sharding strategy. The shard function takes characters of the base32 block key and uses them as the name of the directory the block file goes into. Values used by Kubo:

- `/repo/flatfs/shard/v1/next-to-last/2`
  - Shards on the two next-to-last base32 characters of the key (~1k directories). The default.
- `/repo/flatfs/shard/v1/next-to-last/3`
  - Shards on the three next-to-last base32 characters of the key (~32k directories). See [Choosing a `shardFunc`](#choosing-a-shardfunc-for-large-blockstores).

`prefix/N` and `suffix/N` also parse but spread Kubo's keys badly: every base32 sha2-256 multihash starts with `CIQ`, and the last base32 character carries only 2 of its 5 bits. `next-to-last` skips that last character, which is why it is the default.

```json
{
	"type": "flatfs",
	"path": "<relative path within repo for flatfs root>",
	"shardFunc": "<a descriptor of the sharding scheme>",
	"sync": true|false
}
```

- `sync`: Flush every write to disk before continuing. Setting this to false is safe as kubo will automatically flush writes to disk before and after performing critical operations like pinning. However, you can set this to true to be extra-safe (at the cost of a slowdown when adding files).

> [!WARNING]
> flatfs is a special-purpose store for content-addressed data (CID to block) and is safe only when mounted at `/blocks`. It is not a general-purpose key-value store:
>
> - It assumes a key is the hash of its value, so the same key always carries the same bytes. When several writes target one key at the same time, or within one batch, the first to finish is kept and the rest are skipped without an error. That is correct for blocks and wrong for anything mutable (pins, MFS root, provider records, IPNS records), which needs a store where the last write wins, such as leveldb or pebble.
> - Keys become file names, so only upper-case letters, digits, and the characters `-`, `+`, `_`, `=` are accepted. Namespaced keys such as `/foo/bar` are rejected.
> - Queries by key prefix return nothing; only a query over the whole store works.
>
> This is why every profile that uses flatfs mounts it at `/blocks` only and keeps the remaining keys in leveldb or pebble: the default `flatfs-levelds` profile (alias `flatfs`) pairs it with leveldb, `flatfs-pebbleds` with pebble. Mount flatfs for `/blocks` only, using the [mount](#mount) datastore. See the [go-ds-flatfs restrictions](https://github.com/ipfs/go-ds-flatfs/blob/master/README.md#restrictions) for details.

### Choosing a `shardFunc` for large blockstores

flatfs stores every block as one file and spreads the files over shard directories named by characters of the block key. The shard function fixes how many directories there are, and with it how many files each directory holds as the repo grows. The default `next-to-last/2` uses 1k directories and is right for most nodes. A node that will hold tens of millions of blocks should be created with `next-to-last/3` (32k directories), because the depth cannot be changed afterwards.

Why directory size matters:

- Everything that walks the blockstore (GC, the [`Datastore.BloomFilterSize`](config.md#datastorebloomfiltersize) rebuild at startup, `Provide.Strategy=all` reprovide cycles, `ipfs repo stat`, `ipfs refs local`, `ipfs repo verify`) lists one shard at a time and holds all of that shard's names in memory. The total work is the same at either depth, but the size of each step is not.
- The directories are ordinary directories, and everything else that touches them slows down as they grow: `ls`, `du`, `rsync`, backup agents, and any directory scan on a rotational disk. Keeping a directory to a few thousand entries keeps those tools usable.
- Looking up a single block (`Has`, `Get`, `GetSize`) is one `stat` or `open` of a known path, and filesystems with hashed directories (ext4 `dir_index`, XFS, btrfs, ZFS) do that in constant time whatever the directory size. Shard depth does not change bitswap or gateway latency; for that, size [`Datastore.BlockKeyCacheSize`](config.md#datastoreblockkeycachesize) and [`Datastore.BloomFilterSize`](config.md#datastorebloomfiltersize).

| `shardFunc`      | Directories | Files per directory at 60M blocks | How to get it                                  |
|------------------|------------:|----------------------------------:|------------------------------------------------|
| `next-to-last/2` | 1024        | ~58k                              | default in every flatfs profile                |
| `next-to-last/3` | 32768       | ~1.8k                             | config file passed to `ipfs init` (see below)  |

When to opt in: when the repo is expected to grow past about 10M blocks, the point where the default layout puts more than ~10k files in every directory. With the default 256 KiB chunks that is a few terabytes of data; a repo of small files or small chunks gets there far sooner. `NumObjects` in `ipfs repo stat` is the number to watch. Over the years, several large pinning and gateway operators chose to run `next-to-last/3`, and ipfs-cluster's [production guide](https://ipfscluster.io/documentation/deployment/setup/) gives the same advice as "multi-terabyte repositories", with XFS or ZFS recommended for large flatfs repos. Before you opt in, measure on your own setup: the disk, the size of your actual blocks, and how many of them there are all change the performance curve and come with different tradeoffs, and Kubo has no measurement of the two depths against each other. If none of this means anything to you, keep the default `next-to-last/2`; this is an extreme, low-level optimization.

What it costs: up to 32k directories. On ext4 each is a 4 KiB directory block plus an inode, about 128 MiB once all shards exist. Until the repo holds millions of blocks, most shards hold a handful of files and full listings can be slower, not faster, than at `/2`. Keep the default for anything smaller.

The depth is fixed when the repo is created. Three places must agree: `shardFunc` in `Datastore.Spec`, the repo's `datastore_spec` file, and `blocks/SHARDING`. Kubo refuses to open a repo where they differ, and `ipfs config profile apply` refuses a profile that would change the layout. No profile sets `next-to-last/3`; set it through the config file that `ipfs init` accepts as its argument:

1. Create a throwaway repo with the datastore profile you want. This generates a fresh identity and the full default config:

   ```console
   $ export TMP_REPO=$(mktemp -d)
   $ IPFS_PATH=$TMP_REPO ipfs init --profile=flatfs-pebbleds   # or plain 'ipfs init' for the default layout
   ```

2. Copy its `config` file and change `shardFunc` in the copy:

   ```console
   $ sed 's#next-to-last/2#next-to-last/3#' "$TMP_REPO/config" > init-config.json
   $ rm -rf "$TMP_REPO"
   ```

3. Create the real repo from that file. Do not pass a datastore profile here; datastore profiles replace `Datastore.Spec` and reset `shardFunc` to the default. Other profiles, such as `server`, are fine:

   ```console
   $ ipfs init init-config.json      # or: ipfs init - < init-config.json
   $ cat "$IPFS_PATH/blocks/SHARDING"
   /repo/flatfs/shard/v1/next-to-last/3
   ```

Use the raw `config` file, not the output of `ipfs config show`: that output has no private key, and `ipfs init` refuses it. The identity in the file is used as is, so never reuse another node's `config`. A `shardFunc` that does not parse makes `ipfs init` fail before it writes anything, so the directory stays usable for a corrected attempt.

To change the depth of an existing repo, create a new repo with the wanted layout and move the data there: `ipfs dag export` and `ipfs dag import` for whole DAGs, or `ipfs pin ls -t recursive` on the old node and `ipfs pin add` on the new one. Kubo has no command that re-shards a repo in place.

## levelds

Uses a [leveldb](https://github.com/syndtr/goleveldb) database to store key-value
pairs via [go-ds-leveldb](https://github.com/ipfs/go-ds-leveldb).

```json
{
	"type": "levelds",
	"path": "<location of db inside repo>",
	"compression": "none" | "snappy",
}
```

> [!NOTE]
> LevelDB uses a log-structured merge-tree (LSM) storage engine. When keys are
> deleted, the data is not removed immediately. Instead, a tombstone marker is
> written, and the actual data is removed later by background compaction.
>
> LevelDB's compaction decides what to compact based on file counts (L0) and
> total level size (L1+), without considering how many tombstones a file
> contains. This means that after bulk deletions (such as pin removals or the
> periodic provider keystore sync), disk space may not be reclaimed promptly.
> The `datastore/` directory can grow significantly larger than the live data it
> holds, especially on long-running nodes with many CIDs.
>
> Unlike flatfs (which deletes files immediately) or pebble (which has
> tombstone-aware compaction), LevelDB has no way to prioritize reclaiming
> space from deleted keys. Restarting the daemon may trigger some compaction,
> but this is not guaranteed.
>
> If slow compaction is a problem, consider using the `pebbleds` datastore
> instead (see below), which handles this workload more efficiently.

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

Using a pebble datastore can be set when initializing kubo `ipfs init --profile pebbleds`. To keep blocks in flatfs and use pebble only for the remaining keys, use `ipfs init --profile flatfs-pebbleds`. Both profiles are experimental and opt-in.

#### Use of `formatMajorVersion`

[Pebble's `FormatMajorVersion`](https://github.com/cockroachdb/pebble/tree/master?tab=readme-ov-file#format-major-versions) is a constant controlling the format of persisted data. Backwards incompatible changes to durable formats are gated behind new format major versions.

At any point, a database's format major version may be bumped. However, once a database's format major version is increased, previous versions of Pebble will refuse to open the database.

When IPFS is initialized to use the pebbleds datastore (`ipfs init --profile=pebbleds` or `--profile=flatfs-pebbleds`), the latest pebble database format is configured in the pebble datastore config as `"formatMajorVersion"`. Setting this in the datastore config prevents automatically upgrading to the latest available version when kubo is upgraded. If a later version becomes available, the kubo daemon prints a startup message to indicate this. The user can them update the config to use the latest format when they are certain a downgrade will not be necessary.

Without the `"formatMajorVersion"` in the pebble datastore config, the database format is automatically upgraded to the latest version. If this happens, then it is possible a downgrade back to the previous version of kubo will not work if new format is not compatible with the pebble datastore in the previous version of kubo.

When installing a new version of kubo when `"formatMajorVersion"` is configured, migration does not upgrade this to the latest available version. This is done because a user may have reasons not to upgrade the pebble database format, and may want to be able to downgrade kubo if something else is not working in the new version. If the configured pebble database format in the old kubo is not supported in the new kubo, then the configured version must be updated and the old kubo run, before installing the new kubo.

## badgerds

Uses [badger](https://github.com/dgraph-io/badger) as a key-value store.

> [!CAUTION]
> **Badger v1 datastore is deprecated and will be removed in a future Kubo release.**
>
> This is based on very old badger 1.x, which has not been maintained by its
> upstream maintainers for years and has known bugs (startup timeouts, shutdown
> hangs, file descriptor
> exhaustion, and more). Do not use it for new deployments.
>
> **To migrate:** create a new `IPFS_PATH` with `flatfs`
> (`ipfs init --profile=flatfs`), move pinned data via
> `ipfs dag export/import` or `ipfs pin ls -t recursive|add`, and decommission the
> old badger-based node. When it comes to block storage, use experimental
> `pebbleds` only if you are sure modern `flatfs` does not serve your use case
> (most users will be perfectly fine with `flatfs`; the `flatfs-pebbleds`
> profile keeps `flatfs` for blocks and replaces `leveldb` with `pebble` if
> preferred over `leveldb`).

- `syncWrites`: Flush every write to disk before continuing. Setting this to false is safe as kubo will automatically flush writes to disk before and after performing critical operations like pinning. However, you can set this to true to be extra-safe (at the cost of a 2-3x slowdown when adding files).
- `truncate`: Truncate the DB if a partially written sector is found (defaults to true). There is no good reason to set this to false unless you want to manually recover partially written (and unpinned) blocks if kubo crashes half-way through a write operation.

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

Every operation goes through the wrapper, which adds overhead. The `-measure` profiles (`flatfs-levelds-measure`, `flatfs-pebbleds-measure`, `pebbleds-measure`) are provided for debugging, right-sizing, and testing.

```json
{
	"type": "measure",
	"prefix": "sometag.datastore",
	"child": { datastore being wrapped }
}
```

