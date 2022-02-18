# go-ipfs changelog

## v0.12.0 2022-02-17

We're happy to announce go-ipfs 0.12.0. This release switches the storage of IPLD blocks to be keyed by multihash instead of CID.

As usual, this release includes important fixes, some of which may be critical for security. Unless the fix addresses a bug being exploited in the wild, the fix will _not_ be called out in the release notes. Please make sure to update ASAP. See our [release process](https://github.com/ipfs/go-ipfs/tree/master/docs/releases.md#security-fix-policy) for details.

### 🛠 BREAKING CHANGES

- `ipfs refs local` will now list all blocks as if they were [raw]() CIDv1 instead of with whatever CID version and IPLD codecs they were stored with. All other functionality should remain the same.

Keep reading to learn more details.

#### 🔦 Highlights

There is only one change since 0.11:

##### Blockstore migration from full CID to Multihash keys

We are switching the default low level [datastore](https://docs.ipfs.io/concepts/glossary/#datastore) to be keyed only by the [Multihash](https://docs.ipfs.io/concepts/glossary/#multihash) part of the [CID](https://docs.ipfs.io/concepts/glossary/#cid), and deduplicate some [blocks](https://docs.ipfs.io/concepts/glossary/#block) in the process. The blockstore will become [codec](https://docs.ipfs.io/concepts/glossary/#codec)-agnostic.

###### Rationale

The blockstore/datastore layers are not concerned with data interpretation, only with storage of binary blocks and verification that the Multihash they are addressed with (which comes from the CID), matches the block. In fact, different CIDs, with different codecs prefixes, may be carrying the same multihash, and referencing the same block. Carrying the CID abstraction so low on the stack means potentially fetching and storing the same blocks multiple times just because they are referenced by different CIDs. Prior to this change, a CIDv1 with a `dag-cbor` codec and a CIDv1 with a `raw` codec, both containing the same multihash, would result in two identical blocks stored. A CIDv0 and CIDv1 both being the same `dag-pb` block would also result in two copies.

###### How migration works

In order to perform the switch, and start referencing all blocks by their multihash, a migration will occur on update. This migration will take the repository version from 11 (current) to 12.

One thing to note is that any content addressed CIDv0 (all the hashes that start with `Qm...`, the current default in go-ipfs), does not need any migration, as CIDv0 are raw multihashes already. This means the migration will be very lightweight for the majority of users.

The migration process will take care of re-keying any CIDv1 block so that it is only addressed by its multihash. Large nodes with lots of CIDv1-addressed content will need to go through a heavier process as the migration happens. This is how the migration works:

1. Phase 1: The migration script will perform a pass for every block in the datastore and will add all CIDv1s found to a file named `11-to-12-cids.txt`, in the go-ipfs configuration folder. Nothing is written in this first phase and it only serves to identify keys that will be migrated in phase 2.
2. Phase 2: The migration script will perform a second pass where every CIDv1 block will be read and re-written with its raw-multihash as key. There is 1 worker performing this task, although more can be configured. Every 100MiB-worth of blocks (this is configurable), each worker will trigger a datastore "sync" (to ensure all written data is flushed to disk) and delete the CIDv1-addressed blocks that were just renamed. This provides a good compromise between speed and resources needed to run the migration.

At every sync, the migration emits a log message showing how many blocks need to be rewritten and how far the process is.

####### FlatFS specific migration

For those using a single FlatFS datastore as their backing blockstore (i.e. the default behavior), the migration (but not reversion) will take advantage of the ability to easily move/rename the blocks to improve migration performance.

Unfortunately, other common datastores do not support renames which is what makes this FlatFS specific. If you are running a large custom datastore that supports renames you may want to consider running a fork of [fs-repo-11-to-12](https://github.com/ipfs/fs-repo-migrations/tree/master/fs-repo-11-to-12) specific to your datastore.

If you want to disable this behavior, set the environment variable `IPFS_FS_MIGRATION_11_TO_12_ENABLE_FLATFS_FASTPATH` to `false`.

####### Migration configuration

For those who want to tune the migration more precisely for their setups, there are two environment variables to configure:

- `IPFS_FS_MIGRATION_11_TO_12_NWORKERS` : an integer describing the number of migration workers - defaults to 1
- `IPFS_FS_MIGRATION_11_TO_12_SYNC_SIZE_BYTES` : an integer describing the number of bytes after which migration workers will sync - defaults to 104857600 (i.e. 100MiB)

###### Migration caveats

Large repositories with very large numbers of CIDv1s should be mindful of the migration process:

* We recommend ensuring that IPFS runs with an appropriate (high) file-descriptor limit, particularly when Badger is use as datastore backend. Badger is known to open many tables when experiencing a high number of writes, which may trigger "too many files open" type of errors during the migrations. If this happens, the migration can be retried with a higher FD limit (see below).
* Migrations using the Badger datastore may not immediately reclaim the space freed by the deletion of migrated blocks, thus space requirements may grow considerably. A periodic Badger-GC is run every 2 minutes, which will reclaim space used by deleted and de-duplicated blocks. The last portion of the space will only be reclaimed after go-ipfs starts (the Badger-GC cycle will trigger after 15 minutes).
* While there is a revert process detailed below, we recommend keeping a backup of the repository, particularly for very large ones, in case an issue happens, so that the revert can happen immediately and cases of repository corruption due to crashes or unexpected circumstances are not catastrophic.

###### Migration interruptions and retries

If a problem occurs during the migration, it is be possible to simply re-start and retry it:

1. Phase 1 will never overwrite the `11-to-12-cids.txt` file, but only append to it (so that a list of things we were supposed to have migrated during our first attempt is not lost - this is important for reverts, see below).
2. Phase 2 will proceed to continue re-keying blocks that were not re-keyed during previous attempts.

###### Migration reverts

It is also possible to revert the migration after it has succeeded, for example to go to a previous go-ipfs version (<=[0.11](https://github.com/ipfs/go-ipfs/releases/tag/v0.11.0)), even after starting and using go-ipfs in the new version (>=0.12). The revert process works as follows:

1. The `11-to-12-cids.txt` file is read, which has the list of all the CIDv1s that had to be rewritten for the migration.
2. A CIDv1-addressed block is written for every item on the list. This work is performed by 1 worker (configurable), syncing every 100MiB (configurable).
3. It is ensured that every CIDv1 pin, and every CIDv1 reference in MFS, are also written as CIDV1-addressed blocks, regardless of whether they were part of the original migration or were added later.

The revert process does not delete any blocks--it only makes sure that blocks that were accessible with CIDv1s before the migration are again keyed with CIDv1s. This may result in a datastore becoming twice as large (i.e. if all the blocks were CIDv1-addressed before the migration). This is however done this way to cover corner cases: user can add CIDv1s after migration, which may reference blocks that existed as CIDv0 before migration. The revert aims to ensure that no data becomes unavailable on downgrade.

While go-ipfs will auto-run the migration for you, it will not run the reversion. To do so you can download the [latest migration binary](https://dist.ipfs.io/fs-repo-11-to-12) or use [ipfs-update](https://dist.ipfs.io/#ipfs-update).

###### Custom datastores

As with previous migrations if you work with custom datastores and want to leverage the migration you can run a fork of [fs-repo-11-to-12](https://github.com/ipfs/fs-repo-migrations/tree/master/fs-repo-11-to-12) specific to your datastore. The repo includes instructions on building for different datastores.

For this migration, if your datastore has fast renames you may want to consider writing some code to leverage the particular efficiencies of your datastore similar to what was done for FlatFS.

### Changelog

- github.com/ipfs/go-ipfs:
  - Release v0.12.0
  - docs: v0.12.0 release notes
  - chore: bump migrations dist.ipfs.io CID to contain fs-repo-11-to-12 v1.0.2
  - feat: refactor Fetcher interface used for downloading migrations (#8728) ([ipfs/go-ipfs#8728](https://github.com/ipfs/go-ipfs/pull/8728))
  - feat: log multifetcher errors
  - Release v0.12.0-rc1
  - chore: bump Go version to 1.16.12
  - feat: switch to raw multihashes for blocks ([ipfs/go-ipfs#6816](https://github.com/ipfs/go-ipfs/pull/6816))
  - chore: add release template snippet for fetching artifact tarball
  - chore: bump Go version to 1.16.11
  - chore: add release steps for upgrading Go
  - Merge branch 'release'
  - fix(corehttp): adjust peer counting metrics (#8577) ([ipfs/go-ipfs#8577](https://github.com/ipfs/go-ipfs/pull/8577))
  - chore: update version to v0.12.0-dev
- github.com/ipfs/go-filestore (v0.1.0 -> v1.1.0):
  - Version 1.1.0
  - feat: plumb through context changes
  - sync: update CI config files (#54) ([ipfs/go-filestore#54](https://github.com/ipfs/go-filestore/pull/54))
  - fix staticcheck ([ipfs/go-filestore#48](https://github.com/ipfs/go-filestore/pull/48))
  - Work with Multihashes directly (#21) ([ipfs/go-filestore#21](https://github.com/ipfs/go-filestore/pull/21))
- github.com/ipfs/go-ipfs-blockstore (v0.2.1 -> v1.1.2):
  - Release v1.1.2
  - feat: per-cid locking
  - Version 1.1.1
  - fix: remove context from HashOnRead
  - Version 1.1.0 (#91) ([ipfs/go-ipfs-blockstore#91](https://github.com/ipfs/go-ipfs-blockstore/pull/91))
  - feat: add context to interfaces (#90) ([ipfs/go-ipfs-blockstore#90](https://github.com/ipfs/go-ipfs-blockstore/pull/90))
  - sync: update CI config files (#88) ([ipfs/go-ipfs-blockstore#88](https://github.com/ipfs/go-ipfs-blockstore/pull/88))
  - add constructor that doesnt mess with datastore keys ([ipfs/go-ipfs-blockstore#83](https://github.com/ipfs/go-ipfs-blockstore/pull/83))
  - Use bloom filter in GetSize
  - fix staticcheck ([ipfs/go-ipfs-blockstore#73](https://github.com/ipfs/go-ipfs-blockstore/pull/73))
  - add BenchmarkARCCacheConcurrentOps ([ipfs/go-ipfs-blockstore#70](https://github.com/ipfs/go-ipfs-blockstore/pull/70))
  - fix(arc): striped locking on last byte of CID ([ipfs/go-ipfs-blockstore#67](https://github.com/ipfs/go-ipfs-blockstore/pull/67))
  - make idstore implement io.Closer. (#60) ([ipfs/go-ipfs-blockstore#60](https://github.com/ipfs/go-ipfs-blockstore/pull/60))
  - add View() to all the various blockstores. (#59) ([ipfs/go-ipfs-blockstore#59](https://github.com/ipfs/go-ipfs-blockstore/pull/59))
  - Optimize id store ([ipfs/go-ipfs-blockstore#56](https://github.com/ipfs/go-ipfs-blockstore/pull/56))
  - add race fix for HashOnRead ([ipfs/go-ipfs-blockstore#50](https://github.com/ipfs/go-ipfs-blockstore/pull/50))
  - Add test to maintain Put contract of calling Has first ([ipfs/go-ipfs-blockstore#47](https://github.com/ipfs/go-ipfs-blockstore/pull/47))
  - Update readme and license ([ipfs/go-ipfs-blockstore#44](https://github.com/ipfs/go-ipfs-blockstore/pull/44))
  - feat: switch to raw multihashes for blocks ([ipfs/go-ipfs-blockstore#38](https://github.com/ipfs/go-ipfs-blockstore/pull/38))
- github.com/ipfs/go-ipfs-ds-help (v0.1.1 -> v1.1.0):
  - Update version.json (#38) ([ipfs/go-ipfs-ds-help#38](https://github.com/ipfs/go-ipfs-ds-help/pull/38))
  - sync: update CI config files (#37) ([ipfs/go-ipfs-ds-help#37](https://github.com/ipfs/go-ipfs-ds-help/pull/37))
  - feat: switch to raw multihashes for blocks ([ipfs/go-ipfs-ds-help#18](https://github.com/ipfs/go-ipfs-ds-help/pull/18))

### ❤️ Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Gus Eggert | 10 | +333/-321 | 24 |
| Steven Allen | 7 | +289/-190 | 13 |
| Hector Sanjuan | 9 | +134/-109 | 18 |
| Adin Schmahmann | 11 | +179/-55 | 21 |
| Raúl Kripalani | 2 | +152/-42 | 5 |
| Daniel Martí | 1 | +120/-1 | 1 |
| frrist | 1 | +95/-13 | 2 |
| Alex Trottier | 2 | +22/-11 | 4 |
| Andrey Petrov | 1 | +32/-0 | 1 |
| Lucas Molas | 1 | +18/-7 | 2 |
| Marten Seemann | 2 | +11/-7 | 3 |
| whyrusleeping | 1 | +10/-0 | 1 |
| web3-bot | 3 | +9/-0 | 3 |
| postables | 1 | +5/-3 | 1 |
| Dr Ian Preston | 1 | +4/-0 | 1 |

## v0.11.0 2021-12-08

We're happy to announce go-ipfs 0.11.0. This release comes with improvements to the UnixFS Sharding and PubSub experiments as well as support for Circuit-Relay v2 which sets the network up for decentralized hole punching support.

As usual, this release includes important fixes, some of which may be critical for security. Unless the fix addresses a bug being exploited in the wild, the fix will _not_ be called out in the release notes. Please make sure to update ASAP. See our [release process](https://github.com/ipfs/go-ipfs/tree/master/docs/releases.md#security-fix-policy) for details.

### 🛠 BREAKING CHANGES

-  UnixFS sharding is now automatic and enabled by default
   - HAMT-based sharding is applied to large directories (i.e. those that would serialize into [block](https://docs.ipfs.io/concepts/glossary/#block) larger than ~256KiB)s. This means importing data via commands like `ipfs add -r <directory>` may result in different [CID](https://docs.ipfs.io/concepts/glossary/#cid)s due to the different [DAG](https://docs.ipfs.io/concepts/glossary/#dag) representations.
   - Support for `Experimental.ShardingEnabled` is removed.
- go-ipfs can no longer act as a [Circuit Relay](https://docs.ipfs.io/concepts/glossary/#circuit-relay) v1
  - Node will refuse to start if `Swarm.EnableRelayHop` is set to `true`
  -  If you depend on v1 relay service provider, see "Removal of v1 relay service" section for available migration options.
- HTTP RPC wire format for experimental commands at  `/api/v0/pubsub` changed.
  - If you use [js-ipfs-http-client](https://www.npmjs.com/package/ipfs-http-client) or [go-ipfs-http-client](https://github.com/ipfs/go-ipfs-http-client), just update to their latest version.
  - If you use something else, see "Multibase in PubSub" section below for migration details.

Keep reading to learn more details.

### 🔦 Highlights

#### 🗃 Automatic UnixFS sharding

Truly big directories can have so many items, that the root block with all of their names is too big to be exchanged with other peers. This was partially solved by [HAMT-sharding](https://docs.ipfs.io/concepts/glossary/#hamt-sharding), which was introduced a while ago as opt-in. The main downside of the implementation was that it was a global flag that sharded all imported directories (big and small).

This release solves that inconvenience by making UnixFS sharding smarter and applies it only to larger directories (i.e. directories that would be at least ~256KiB). This is now the default behavior in `ipfs add` and `ipfs files` commands, where UnixFS sharding works out-of-the-box.

#### 🔁 Circuit Relay v2

This release adds support for the [circuit relay v2](https://github.com/libp2p/specs/blob/master/relay/circuit-v2.md) protocol based on the reference implementation from [go-libp2p 0.16](https://github.com/libp2p/go-libp2p/releases/tag/v0.16.0).

This is the cornerstone for maximizing p2p connections between IPFS peers. Every publicly dialable peer can now act as a limited relay v2, which can be used for [hole punching](https://docs.ipfs.io/concepts/glossary/#hole-punching) and other decentralized signaling protocols.

##### Limited relay v2 configuration options

go-ipfs can now be configured to act as a [`RelayClient`](https://github.com/ipfs/go-ipfs/blob/master/docs/config.md#swarmrelayclient)  that uses other peers for autorelay functionality when behind a NAT, or provide a limited [`RelayService`](https://github.com/ipfs/go-ipfs/blob/master/docs/config.md#swarmrelayservice) to other peers on the network.

Starting with go-ipfs v0.11 every publicly dialable go-ipfs (based on AutoNAT determination) will start a limited `RelayService`.  `RelayClient` remains disabled by default for now, as we want the network to update and get enough v2 service providers first.

Note: the limited Circuit Relay v2 provided with this release only allows low-bandwidth protocols (identify, ping, holepunch) over transient connections. If you want to relay things like bitswap sessions, you need to set up a v1 relay by some other means. See details below.

##### Removal of unlimited v1 relay service provider

Switching to v2 of the relay spec means removal or deprecation of configuration keys that were specific to v1.

- Relay transport and client support circuit-relay v2:
  - `Swarm.EnableAutoRelay` was replaced by `Swarm.RelayClient.Enable`.
  - `Swarm.DisableRelay` is deprecated, relay transport can be now disabled globally (both client and service) by setting `Swarm.Transports.Network.Relay` to `false`
- Relay v1 service provider was replaced by v2:
  - `Swarm.EnableRelayHop` no longer starts an unlimited v1 relay. If you have it set to `true` the node will refuse to start and display an error message.
  - Existing users who choose to continue running a v1 relay should migrate their setups to relay v1 based on js-ipfs running in node, or the standalone [libp2p-relay-daemon](https://dist.ipfs.io/#libp2p-relay-daemon) [configured](https://github.com/libp2p/go-libp2p-relay-daemon/#configuration) with `RelayV1.Enabled` set to `true`. Be mindful that v1 relays are unlimited, and one may want to set up some ACL based either on PeerIDs or Subnets.

#### 🕳 Decentralized Hole Punching (DCUtR protocol client)

We are working towards enabling hole punching for NAT traversal when port forwarding is not possible.

[go-libp2p 0.16](https://github.com/libp2p/go-libp2p/releases/tag/v0.16.0) provides an implementation of the [DCUtR (decentralized hole punching)](https://github.com/libp2p/specs/blob/master/relay/DCUtR.md) protocol. It is hidden behind the `Swarm.EnableHolePunching` configuration flag.

When enabled, go-ipfs will coordinate with the counterparty using a [relayed v2 connection](https://github.com/libp2p/specs/blob/master/relay/circuit-v2.md), to [upgrade to a direct connection](https://github.com/libp2p/specs/blob/master/relay/DCUtR.md) through a NAT/firewall whenever possible.

This feature is disabled by default in this release, but we hope to enable it by default as soon the network updates to go-ipfs v0.11 and gains a healthy set of limited v2 relays.

#### 💬 Multibase in PubSub HTTP RPC API

This release fixed some edge cases that were reported by users of the PubSub experiment, getting it closer to becoming a stable feature of go-ipfs. Some PubSub users will notice that the plaintext limitation is lifted: one can now use line breaks in messages published to non-ascii topic names, or even publish arbitrary bytes to arbitrary topics.  It required a change to the wire format used when pubsub commands are executed over the HTTP RPC API at `/api/v0/pubsub/*`, and also modified the behavior of the `ipfs pubsub pub` command, which now is publishing only a single pubsub message with data read from a file or stdin.

##### PubSub client migration tips

If you use the HTTP RPC API with the [go-ipfs-http-client](https://github.com/ipfs/go-ipfs-http-client) library, make sure to update to the latest version. The next version of [js-ipfs-http-client](https://www.npmjs.com/package/ipfs-http-client) will use the new wire format as well, so you don't need to do anything.

If you use `/api/v0/pubsub/*` directly or maintain your own client library, you must adjust your HTTP client code. Byte fields and URL args are now encoded in `base64url` [Multibase](https://docs.ipfs.io/concepts/glossary/#multibase). Encode/decode bytes using the `ipfs multibase --help` commands, or use the multiformats libraries ([js-multiformats](https://github.com/multiformats/js-multiformats#readme), [go-multibase](https://github.com/multiformats/go-multibase)).

Low level changes:
- `topic` passed as URL `arg` in requests to `/api/v0/pubsub/*` must be encoded in URL-safe multibase (`base64url`)
- `data`, `from`, `seqno` and `topicIDs` returned in JSON responses are now encoded in multibase
- Peer IDs returned in `from` now use the same default text representation from go-libp2p and peerid encoder/decoder from libp2p. This means the same text representation as in as in `swarm peers`, which makes it possible to compare them without decoding multibase.
-  `/api/v0/pubsub/pub`  no longer accepts `data` to be passed as URL, it has to be sent as `multipart/form-data`. This removes size limitations based on URL length, and enables regular HTTP clients to publish data to PubSub topics. For example, to publish `some-file` to topic named `test-topic` using vanilla `curl`, one would execute: `curl -X POST -v -F "stdin=@some-file" 'http://127.0.0.1:5001/api/v0/pubsub/pub?arg=$(echo -n test-topic | ipfs multibase encode -b base64url)'`
- `ipfs pubsub pub` on the command line no longer accepts variadic `data` arguments. Instead, it expects a single file input or stream of bytes from stdin. This ensures arbitrary stream of bytes can be published, removing limitation around messages that include `\n` or `\r\n`.

#### ⚙ New configuration flags

- [`Addresses.AppendAnnounce`](https://github.com/ipfs/go-ipfs/blob/master/docs/config.md#addressesappendannounce)  is an array of multiaddrs, similar to  `Addresses.Announce`, except it does not override inferred swarm addresses, but appends custom ones to the list.
- Pubsub experiments can now  be enabled via config, removing the need for CLI flag to be passed every time daemon starts:
  - [`Pubsub.Enabled`](https://github.com/ipfs/go-ipfs/blob/master/docs/config.md#pubsubenabled) enables the pubsub system.
  - [`Ipns.UsePubsub`](https://github.com/ipfs/go-ipfs/blob/master/docs/config.md#ipnsusepubsub) enables IPFS over pubsub experiment for publishing IPNS records in real time.

#### 🔐 Support for DAG-JOSE IPLD codec

JOSE is a [standard](https://datatracker.ietf.org/wg/jose/documents/) for signing and encrypting JSON objects. [DAG-JOSE](https://ipld.io/specs/codecs/dag-jose/spec/) is an IPLD codec based on JOSE and represented in CBOR. Upon encountering the `dag-jose` multicodec indicator, implementations can expect that the block contains dag-cbor encoded data which matches the IPLD schema from the [DAG-JOSE spec](https://ipld.io/specs/codecs/dag-jose/spec/).

This work was [contributed](https://github.com/ipfs/go-ipfs/pull/8569) by [Ceramic](https://ceramic.network/) and acts as a template for future IPFS improvements driven by the real world needs of the IPFS community.

### Changelog

- github.com/ipfs/go-ipfs:
  - docs: update changelog for v0.11.0
  - Release v0.11.0-rc2
  - fix(corehttp): adjust peer counting metrics (#8577) ([ipfs/go-ipfs#8577](https://github.com/ipfs/go-ipfs/pull/8577))
  - Release v0.11.0-rc1
  - feat: Swarm.EnableHolePunching flag (#8562) ([ipfs/go-ipfs#8562](https://github.com/ipfs/go-ipfs/pull/8562))
  - feat: enabling pubsub and ipns-pubsub via config flags (#8510) ([ipfs/go-ipfs#8510](https://github.com/ipfs/go-ipfs/pull/8510))
  - Integrate go-dag-jose plugin (#8569) ([ipfs/go-ipfs#8569](https://github.com/ipfs/go-ipfs/pull/8569))
  - feat: Addresses.AppendAnnounce (#8177) ([ipfs/go-ipfs#8177](https://github.com/ipfs/go-ipfs/pull/8177))
  - fix: multibase in pubsub http rpc (#8183) ([ipfs/go-ipfs#8183](https://github.com/ipfs/go-ipfs/pull/8183))
  - refactor: remove dir-index-html submodule   ([ipfs/go-ipfs#8555](https://github.com/ipfs/go-ipfs/pull/8555))
  - feat: hard deprecation of IPFS_REUSEPORT
  - feat: go-libp2p 0.16, UnixFS autosharding and go-datastore with contexts (#8563) ([ipfs/go-ipfs#8563](https://github.com/ipfs/go-ipfs/pull/8563))
  - chore: fix link in README.md (#8551) ([ipfs/go-ipfs#8551](https://github.com/ipfs/go-ipfs/pull/8551))
  - Updating release template based off some 0.10 learnings (#8491) ([ipfs/go-ipfs#8491](https://github.com/ipfs/go-ipfs/pull/8491))
  - fix: multiple subdomain gateways on same domain (#8556) ([ipfs/go-ipfs#8556](https://github.com/ipfs/go-ipfs/pull/8556))
  - Fix typos (#8548) ([ipfs/go-ipfs#8548](https://github.com/ipfs/go-ipfs/pull/8548))
  - Add support for multiple files to `ipfs files rm`.
  - add a docker-compose file (#8387) ([ipfs/go-ipfs#8387](https://github.com/ipfs/go-ipfs/pull/8387))
  - fix(sharness): use -Q option instead of pipe to tail cmd
  - Add Homebrew installation method. ([ipfs/go-ipfs#8545](https://github.com/ipfs/go-ipfs/pull/8545))
  - docs: fix ipfs files cp examples (#8533) ([ipfs/go-ipfs#8533](https://github.com/ipfs/go-ipfs/pull/8533))
  - fix(unixfs): check for errors before dereferencing the link ([ipfs/go-ipfs#8508](https://github.com/ipfs/go-ipfs/pull/8508))
  - chore: replace go-merkledag walk with go-ipld-prime traversal for dag export (#8506) ([ipfs/go-ipfs#8506](https://github.com/ipfs/go-ipfs/pull/8506))
  - test: add sharness test for reading ADLs with FUSE
  - fix: allow the levelds compression level to be unspecified
  -  ([ipfs/go-ipfs#8457](https://github.com/ipfs/go-ipfs/pull/8457))
  -  ([ipfs/go-ipfs#8482](https://github.com/ipfs/go-ipfs/pull/8482))
  - Added the missing heathcheck for the container (#8429) ([ipfs/go-ipfs#8429](https://github.com/ipfs/go-ipfs/pull/8429))
  - chore: update dir-index-html to v1.2.2
  - Update RELEASE_ISSUE_TEMPLATE.md
  - Update RELEASE_ISSUE_TEMPLATE.md
  - add more logging to flaky TestPeersTotal
  - Update RELEASE_ISSUE_TEMPLATE.md
  - Update RELEASE_ISSUE_TEMPLATE.md
  - Updating chocolatey to reference go-ipfs
  - chore: update changelog for v0.10.0
  - add testground plans to bitswap on CI
  - ci: move Docker image build to Actions (#8467) ([ipfs/go-ipfs#8467](https://github.com/ipfs/go-ipfs/pull/8467))
  - fix(cli): object add-link: do not allow blocks over BS limit (#8414) ([ipfs/go-ipfs#8414](https://github.com/ipfs/go-ipfs/pull/8414))
  - fuse: load unixfs adls as their dagpb substrates
  - enable the legacy mDNS implementation
  - change ipfs dag get flag name from format to output-codec ([ipfs/go-ipfs#8440](https://github.com/ipfs/go-ipfs/pull/8440))
  - change names of ipfs dag put flags to make changes clearer ([ipfs/go-ipfs#8439](https://github.com/ipfs/go-ipfs/pull/8439))
  - test: check behavior of loading UnixFS sharded directories with missing shards
  -  ([ipfs/go-ipfs#8432](https://github.com/ipfs/go-ipfs/pull/8432))
  - feat: dag import --stats (#8237) ([ipfs/go-ipfs#8237](https://github.com/ipfs/go-ipfs/pull/8237))
  - feat: ipfs-webui v2.13.0 (#8430) ([ipfs/go-ipfs#8430](https://github.com/ipfs/go-ipfs/pull/8430))
  - feat(cli): add daemon option --agent-version-suffix (#8419) ([ipfs/go-ipfs#8419](https://github.com/ipfs/go-ipfs/pull/8419))
  - feat: multibase transcode command (#8403) ([ipfs/go-ipfs#8403](https://github.com/ipfs/go-ipfs/pull/8403))
  - fix: take the lock while listing peers
  - feature: 'ipfs swarm peering' command (#8147) ([ipfs/go-ipfs#8147](https://github.com/ipfs/go-ipfs/pull/8147))
  - chore: update IPFS Desktop testing steps (#8393) ([ipfs/go-ipfs#8393](https://github.com/ipfs/go-ipfs/pull/8393))
  - add more buttons; remove some sections covered in the docs; general cleanup ([ipfs/go-ipfs#8274](https://github.com/ipfs/go-ipfs/pull/8274))
  - Cosmetic fixups in examples (#8325) ([ipfs/go-ipfs#8325](https://github.com/ipfs/go-ipfs/pull/8325))
  - perf: use performance-enhancing FUSE mount options
  - ci: publish Docker images for bifrost-* branches
  - chore: add comments to peerlog plugin about being unsupported
  - test: add unit tests for peerlog config parsing
  - ci: preload peerlog plugin, disable by default
  - fix(mkreleaselog): specify the parent commit when diffing
  - update version to v0.11.0-dev
- github.com/ipfs/go-bitswap (v0.4.0 -> v0.5.1):
  - Version 0.5.1
  - Change incorrect function name in README (#541) ([ipfs/go-bitswap#541](https://github.com/ipfs/go-bitswap/pull/541))
  - Version 0.5.0 (#540) ([ipfs/go-bitswap#540](https://github.com/ipfs/go-bitswap/pull/540))
  - feat: plumb through contexts (#539) ([ipfs/go-bitswap#539](https://github.com/ipfs/go-bitswap/pull/539))
  - sync: update CI config files (#538) ([ipfs/go-bitswap#538](https://github.com/ipfs/go-bitswap/pull/538))
  - fix: optimize handling for peers with lots of tasks ([ipfs/go-bitswap#537](https://github.com/ipfs/go-bitswap/pull/537))
  - Enable custom task prioritization logic ([ipfs/go-bitswap#535](https://github.com/ipfs/go-bitswap/pull/535))
  - feat: cache the materialized wantlist ([ipfs/go-bitswap#530](https://github.com/ipfs/go-bitswap/pull/530))
  - fix: reduce receive contention ([ipfs/go-bitswap#536](https://github.com/ipfs/go-bitswap/pull/536))
  - Fix ProviderQueryManager test timings ([ipfs/go-bitswap#534](https://github.com/ipfs/go-bitswap/pull/534))
  - fix: rename wiretap to tracer ([ipfs/go-bitswap#531](https://github.com/ipfs/go-bitswap/pull/531))
  - fix: fix race on "responsive" check ([ipfs/go-bitswap#528](https://github.com/ipfs/go-bitswap/pull/528))
  - fix: reduce log verbosity
- github.com/ipfs/go-blockservice (v0.1.7 -> v0.2.1):
  - Version 0.2.1
  - Version 0.2.0 (#87) ([ipfs/go-blockservice#87](https://github.com/ipfs/go-blockservice/pull/87))
  - feat: add context to interfaces (#86) ([ipfs/go-blockservice#86](https://github.com/ipfs/go-blockservice/pull/86))
  - sync: update CI config files (#85) ([ipfs/go-blockservice#85](https://github.com/ipfs/go-blockservice/pull/85))
  - chore: update log ([ipfs/go-blockservice#84](https://github.com/ipfs/go-blockservice/pull/84))
- github.com/ipfs/go-cid (v0.0.7 -> v0.1.0):
  - amend the CidFromReader slice extension math
  - implement CidFromReader
  - chore: fixups from running go vet, go fmt and staticcheck ([ipfs/go-cid#122](https://github.com/ipfs/go-cid/pull/122))
  - s/characters/bytes
  - Fix inaccurate comment for uvarint
  - coverage: more tests for cid
  - coverage: more tests for varint
  - coverage: more tests for builder
  - fix: make tests run with Go 1.15
  - Add the dagjose multiformat
- github.com/ipfs/go-datastore (v0.4.6 -> v0.5.1):
  - Release v0.5.1
  - chore: add lots of interface assertions
  - fix: make NullDatastore satisfy the Batching interface again
  - Update version.json (#183) ([ipfs/go-datastore#183](https://github.com/ipfs/go-datastore/pull/183))
  - feat: add context to interfaces (#181) ([ipfs/go-datastore#181](https://github.com/ipfs/go-datastore/pull/181))
  - sync: update CI config files ([ipfs/go-datastore#182](https://github.com/ipfs/go-datastore/pull/182))
- github.com/ipfs/go-ds-badger (v0.2.7 -> v0.3.0):
  - feat: plumb through contexts (#119) ([ipfs/go-ds-badger#119](https://github.com/ipfs/go-ds-badger/pull/119))
- github.com/ipfs/go-ds-flatfs (v0.4.5 -> v0.5.1):
  - Update version.json
  - fix: add context to DiskUsage()
  - Version 0.5.0 (#99) ([ipfs/go-ds-flatfs#99](https://github.com/ipfs/go-ds-flatfs/pull/99))
  - feat: add contexts on datastore methods (#98) ([ipfs/go-ds-flatfs#98](https://github.com/ipfs/go-ds-flatfs/pull/98))
  - sync: update CI config files (#97) ([ipfs/go-ds-flatfs#97](https://github.com/ipfs/go-ds-flatfs/pull/97))
  - sync: update CI config files ([ipfs/go-ds-flatfs#96](https://github.com/ipfs/go-ds-flatfs/pull/96))
  - fix staticcheck ([ipfs/go-ds-flatfs#92](https://github.com/ipfs/go-ds-flatfs/pull/92))
  - fix typo in readme.go ([ipfs/go-ds-flatfs#89](https://github.com/ipfs/go-ds-flatfs/pull/89))
- github.com/ipfs/go-ds-leveldb (v0.4.2 -> v0.5.0):
  - Version 0.5.0 (#58) ([ipfs/go-ds-leveldb#58](https://github.com/ipfs/go-ds-leveldb/pull/58))
  - feat: plumb through contexts (#57) ([ipfs/go-ds-leveldb#57](https://github.com/ipfs/go-ds-leveldb/pull/57))
  - sync: update CI config files (#56) ([ipfs/go-ds-leveldb#56](https://github.com/ipfs/go-ds-leveldb/pull/56))
  - fix closing of datastore in tests ([ipfs/go-ds-leveldb#52](https://github.com/ipfs/go-ds-leveldb/pull/52))
  - fix staticcheck ([ipfs/go-ds-leveldb#49](https://github.com/ipfs/go-ds-leveldb/pull/49))
  - fix typo in function documentation ([ipfs/go-ds-leveldb#46](https://github.com/ipfs/go-ds-leveldb/pull/46))
- github.com/ipfs/go-ds-measure (v0.1.0 -> v0.2.0):
  - Version 0.2.0 (#39) ([ipfs/go-ds-measure#39](https://github.com/ipfs/go-ds-measure/pull/39))
  - feat: add contexts on datastore methods (#38) ([ipfs/go-ds-measure#38](https://github.com/ipfs/go-ds-measure/pull/38))
  - sync: update CI config files (#37) ([ipfs/go-ds-measure#37](https://github.com/ipfs/go-ds-measure/pull/37))
- github.com/ipfs/go-fetcher (v1.5.0 -> v1.6.1):
  - Version 1.6.1
  - Version 1.6.0 (#29) ([ipfs/go-fetcher#29](https://github.com/ipfs/go-fetcher/pull/29))
  - feat: plumb through context changes (#28) ([ipfs/go-fetcher#28](https://github.com/ipfs/go-fetcher/pull/28))
  - sync: update CI config files (#27) ([ipfs/go-fetcher#27](https://github.com/ipfs/go-fetcher/pull/27))
  - add a fetcher constructor for the case where we already have a session ([ipfs/go-fetcher#26](https://github.com/ipfs/go-fetcher/pull/26))
- github.com/ipfs/go-filestore (v0.0.3 -> v0.1.0):
  - feat: plumb through context changes (#56) ([ipfs/go-filestore#56](https://github.com/ipfs/go-filestore/pull/56))
- github.com/ipfs/go-graphsync (v0.8.0 -> v0.11.0):
  - docs(CHANGELOG): update for v0.11.0 release
  - Merge branch 'release/v0.10.6'
  - update to context datastores (#275) ([ipfs/go-graphsync#275](https://github.com/ipfs/go-graphsync/pull/275))
  - feat!(requestmanager): remove request allocation backpressure (#272) ([ipfs/go-graphsync#272](https://github.com/ipfs/go-graphsync/pull/272))
  - message/pb: stop using gogo/protobuf (#277) ([ipfs/go-graphsync#277](https://github.com/ipfs/go-graphsync/pull/277))
  - mark all test helper funcs via t.Helper (#276) ([ipfs/go-graphsync#276](https://github.com/ipfs/go-graphsync/pull/276))
  - chore(queryexecutor): remove unused RunTraversal
  - chore(responsemanager): remove unused workSignal
  - chore(queryexecutor): fix tests for runtraversal refactor + clean up
  - feat(queryexecutor): merge RunTraversal into QueryExecutor
  - feat(responsemanager): QueryExecutor to separate module - use TaskQueue, add tests
  - Merge branch 'release/v0.10.5'
  - fix(responseassembler): dont hold block data reference in passed on subscribed block link (#268) ([ipfs/go-graphsync#268](https://github.com/ipfs/go-graphsync/pull/268))
  - sync: update CI config files (#266) ([ipfs/go-graphsync#266](https://github.com/ipfs/go-graphsync/pull/266))
  - Check IPLD context cancellation error type instead of string comparison
  - Use `context.CancelFunc` instead of `func()` (#257) ([ipfs/go-graphsync#257](https://github.com/ipfs/go-graphsync/pull/257))
  - fix: bail properly when budget exceeded
  - feat(requestmanager): report inProgressRequestCount on OutgoingRequests event
  - fix(requestmanager): remove failing racy test select block
  - feat(requestmanager): add OutgoingRequeustProcessingListener
  - Merge branch 'release/v0.10.4'
  - fix(allocator): prevent buffer overflow (#248) ([ipfs/go-graphsync#248](https://github.com/ipfs/go-graphsync/pull/248))
  - Merge branch 'release/v0.10.3'
  - Configure message parameters (#247) ([ipfs/go-graphsync#247](https://github.com/ipfs/go-graphsync/pull/247))
  - Stats! (#246) ([ipfs/go-graphsync#246](https://github.com/ipfs/go-graphsync/pull/246))
  - Limit simultaneous incoming requests on a per peer basis (#245) ([ipfs/go-graphsync#245](https://github.com/ipfs/go-graphsync/pull/245))
  - sync: update CI config files (#191) ([ipfs/go-graphsync#191](https://github.com/ipfs/go-graphsync/pull/191))
  - Merge branch 'release/v0.10.2'
  - test(responsemanager): fix flakiness TestCancellationViaCommand (#243) ([ipfs/go-graphsync#243](https://github.com/ipfs/go-graphsync/pull/243))
  - Fix deadlock on notifications (#242) ([ipfs/go-graphsync#242](https://github.com/ipfs/go-graphsync/pull/242))
  - Merge branch 'release/v0.10.1'
  - Free memory on request finish (#240) ([ipfs/go-graphsync#240](https://github.com/ipfs/go-graphsync/pull/240))
  - release: v1.10.0 ([ipfs/go-graphsync#238](https://github.com/ipfs/go-graphsync/pull/238))
  - Add support for IPLD prime's budgets feature in selectors (#235) ([ipfs/go-graphsync#235](https://github.com/ipfs/go-graphsync/pull/235))
  - feat(graphsync): add an index for blocks in the on new block hook (#234) ([ipfs/go-graphsync#234](https://github.com/ipfs/go-graphsync/pull/234))
  - Do not send first blocks extension (#230) ([ipfs/go-graphsync#230](https://github.com/ipfs/go-graphsync/pull/230))
  - Protect Libp2p Connections (#229) ([ipfs/go-graphsync#229](https://github.com/ipfs/go-graphsync/pull/229))
  - test(responsemanager): remove check (#228) ([ipfs/go-graphsync#228](https://github.com/ipfs/go-graphsync/pull/228))
  - feat(graphsync): give missing blocks a named error (#227) ([ipfs/go-graphsync#227](https://github.com/ipfs/go-graphsync/pull/227))
  - Add request limits (#224) ([ipfs/go-graphsync#224](https://github.com/ipfs/go-graphsync/pull/224))
  - Tech Debt Cleanup and Docs Update (#219) ([ipfs/go-graphsync#219](https://github.com/ipfs/go-graphsync/pull/219))
  - Release/v0.9.3 ([ipfs/go-graphsync#218](https://github.com/ipfs/go-graphsync/pull/218))
  - 0.9.2 release ([ipfs/go-graphsync#217](https://github.com/ipfs/go-graphsync/pull/217))
  - fix(requestmanager): remove main thread block on allocation (#216) ([ipfs/go-graphsync#216](https://github.com/ipfs/go-graphsync/pull/216))
  - feat(allocator): add debug logging (#213) ([ipfs/go-graphsync#213](https://github.com/ipfs/go-graphsync/pull/213))
  - fix: spurious warn log (#210) ([ipfs/go-graphsync#210](https://github.com/ipfs/go-graphsync/pull/210))
  - docs(CHANGELOG): update for v0.9.1 release (#212) ([ipfs/go-graphsync#212](https://github.com/ipfs/go-graphsync/pull/212))
  - fix(message): fix dropping of response extensions (#211) ([ipfs/go-graphsync#211](https://github.com/ipfs/go-graphsync/pull/211))
  - docs(CHANGELOG): update change log ([ipfs/go-graphsync#208](https://github.com/ipfs/go-graphsync/pull/208))
  - docs(README): add notice about branch rename
  - fix(graphsync): make sure linkcontext is passed (#207) ([ipfs/go-graphsync#207](https://github.com/ipfs/go-graphsync/pull/207))
  - Merge final v0.6.x commit history, and 0.8.0 changelog (#205) ([ipfs/go-graphsync#205](https://github.com/ipfs/go-graphsync/pull/205))
  - Fix broken link to IPLD selector documentation (#189) ([ipfs/go-graphsync#189](https://github.com/ipfs/go-graphsync/pull/189))
  - fix: check errors before defering a close (#200) ([ipfs/go-graphsync#200](https://github.com/ipfs/go-graphsync/pull/200))
  - chore: fix checks (#197) ([ipfs/go-graphsync#197](https://github.com/ipfs/go-graphsync/pull/197))
  - Merge the v0.6.x commit history (#190) ([ipfs/go-graphsync#190](https://github.com/ipfs/go-graphsync/pull/190))
  - Ready for universal CI (#187) ([ipfs/go-graphsync#187](https://github.com/ipfs/go-graphsync/pull/187))
  - fix(requestmanager): pass through linksystem (#166) ([ipfs/go-graphsync#166](https://github.com/ipfs/go-graphsync/pull/166))
  - fix missing word in section title (#179) ([ipfs/go-graphsync#179](https://github.com/ipfs/go-graphsync/pull/179))
- github.com/ipfs/go-ipfs-blockstore (v0.1.6 -> v0.2.1):
  - fix: revert back to go-ipfs-ds-help@v0.1.1 (#92) ([ipfs/go-ipfs-blockstore#92](https://github.com/ipfs/go-ipfs-blockstore/pull/92))
  - feat: add context to interfaces & plumb through datastore contexts (#89) ([ipfs/go-ipfs-blockstore#89](https://github.com/ipfs/go-ipfs-blockstore/pull/89))
- github.com/ipfs/go-ipfs-config (v0.16.0 -> v0.18.0):
  - Release v0.18.0 (#159) ([ipfs/go-ipfs-config#159](https://github.com/ipfs/go-ipfs-config/pull/159))
  - feat: add Addresses.AppendAnnounce (#135) ([ipfs/go-ipfs-config#135](https://github.com/ipfs/go-ipfs-config/pull/135))
  - feat: omitempty Swarm.EnableRelayHop for circuit v1 migration (#157) ([ipfs/go-ipfs-config#157](https://github.com/ipfs/go-ipfs-config/pull/157))
  - chore: omitempty Experimental.ShardingEnabled (#158) ([ipfs/go-ipfs-config#158](https://github.com/ipfs/go-ipfs-config/pull/158))
  - chore: update comment to match struct
  - Release v0.17.0 (#156) ([ipfs/go-ipfs-config#156](https://github.com/ipfs/go-ipfs-config/pull/156))
  - feat: add a flag to enable the hole punching service (#155) ([ipfs/go-ipfs-config#155](https://github.com/ipfs/go-ipfs-config/pull/155))
  - improve AutoRelay configuration, add config option for static relays ([ipfs/go-ipfs-config#154](https://github.com/ipfs/go-ipfs-config/pull/154))
  - feat: Swarm.RelayService (circuit v2) (#146) ([ipfs/go-ipfs-config#146](https://github.com/ipfs/go-ipfs-config/pull/146))
  - fix: String method on the OptionalString (#153) ([ipfs/go-ipfs-config#153](https://github.com/ipfs/go-ipfs-config/pull/153))
  - sync: update CI config files (#152) ([ipfs/go-ipfs-config#152](https://github.com/ipfs/go-ipfs-config/pull/152))
  - feat: OptionalString type and UnixFSShardingSizeThreshold (#149) ([ipfs/go-ipfs-config#149](https://github.com/ipfs/go-ipfs-config/pull/149))
  - feat: pubsub and ipns pubsub flags (#145) ([ipfs/go-ipfs-config#145](https://github.com/ipfs/go-ipfs-config/pull/145))
  - feat: add an OptionalDuration type (#148) ([ipfs/go-ipfs-config#148](https://github.com/ipfs/go-ipfs-config/pull/148))
- github.com/ipfs/go-ipfs-exchange-interface (v0.0.1 -> v0.1.0):
  - Update version.json (#20) ([ipfs/go-ipfs-exchange-interface#20](https://github.com/ipfs/go-ipfs-exchange-interface/pull/20))
  - sync: update CI config files (#19) ([ipfs/go-ipfs-exchange-interface#19](https://github.com/ipfs/go-ipfs-exchange-interface/pull/19))
  - feat: add context to interface (#18) ([ipfs/go-ipfs-exchange-interface#18](https://github.com/ipfs/go-ipfs-exchange-interface/pull/18))
  - doc: add a lead maintainer
- github.com/ipfs/go-ipfs-exchange-offline (v0.0.1 -> v0.1.1):
  - Version 0.1.1
  - Version 0.1.0 (#43) ([ipfs/go-ipfs-exchange-offline#43](https://github.com/ipfs/go-ipfs-exchange-offline/pull/43))
  - feat: plumb through contexts (#42) ([ipfs/go-ipfs-exchange-offline#42](https://github.com/ipfs/go-ipfs-exchange-offline/pull/42))
  - sync: update CI config files (#41) ([ipfs/go-ipfs-exchange-offline#41](https://github.com/ipfs/go-ipfs-exchange-offline/pull/41))
  - fix staticcheck ([ipfs/go-ipfs-exchange-offline#35](https://github.com/ipfs/go-ipfs-exchange-offline/pull/35))
  - chore(gx): remove gx
- github.com/ipfs/go-ipfs-files (v0.0.8 -> v0.0.9):
  - sync: update CI config files ([ipfs/go-ipfs-files#40](https://github.com/ipfs/go-ipfs-files/pull/40))
  - fix: manually parse the content disposition to preserve directories ([ipfs/go-ipfs-files#42](https://github.com/ipfs/go-ipfs-files/pull/42))
  - fix: round timestamps down by truncating them to seconds ([ipfs/go-ipfs-files#41](https://github.com/ipfs/go-ipfs-files/pull/41))
  - sync: update CI config files ([ipfs/go-ipfs-files#34](https://github.com/ipfs/go-ipfs-files/pull/34))
  - Fix test failure on Windows caused by nil `sys` in mock `FileInfo` ([ipfs/go-ipfs-files#39](https://github.com/ipfs/go-ipfs-files/pull/39))
  - fix staticcheck ([ipfs/go-ipfs-files#35](https://github.com/ipfs/go-ipfs-files/pull/35))
  - fix linters ([ipfs/go-ipfs-files#33](https://github.com/ipfs/go-ipfs-files/pull/33))
- github.com/ipfs/go-ipfs-pinner (v0.1.2 -> v0.2.1):
  - feat: plumb through context changes (#18) ([ipfs/go-ipfs-pinner#18](https://github.com/ipfs/go-ipfs-pinner/pull/18))
- github.com/ipfs/go-ipfs-provider (v0.6.1 -> v0.7.1):
  - Fix go vet and staticcheck ([ipfs/go-ipfs-provider#40](https://github.com/ipfs/go-ipfs-provider/pull/40))
  - feat: plumb through datastore contexts (#39) ([ipfs/go-ipfs-provider#39](https://github.com/ipfs/go-ipfs-provider/pull/39))
- github.com/ipfs/go-ipfs-routing (v0.1.0 -> v0.2.1):
  - Version 0.2.1
  - Bump version to 0.2.0 (#29) ([ipfs/go-ipfs-routing#29](https://github.com/ipfs/go-ipfs-routing/pull/29))
  - feat: plumb through context changes (#28) ([ipfs/go-ipfs-routing#28](https://github.com/ipfs/go-ipfs-routing/pull/28))
  - sync: update CI config files (#27) ([ipfs/go-ipfs-routing#27](https://github.com/ipfs/go-ipfs-routing/pull/27))
  - fix staticcheck ([ipfs/go-ipfs-routing#24](https://github.com/ipfs/go-ipfs-routing/pull/24))
- github.com/ipfs/go-merkledag (v0.4.0 -> v0.5.1):
  - Version 0.5.1
  - Version 0.5.0 (#79) ([ipfs/go-merkledag#79](https://github.com/ipfs/go-merkledag/pull/79))
  - feat: plumb through contexts (#78) ([ipfs/go-merkledag#78](https://github.com/ipfs/go-merkledag/pull/78))
  - sync: update CI config files (#77) ([ipfs/go-merkledag#77](https://github.com/ipfs/go-merkledag/pull/77))
  - expose session construction to other callers
  - fix RawNode incomplete stats
- github.com/ipfs/go-mfs (v0.1.2 -> v0.2.1):
  - Version 0.2.1
  - Version 0.2.0 (#96) ([ipfs/go-mfs#96](https://github.com/ipfs/go-mfs/pull/96))
  - support threshold based automatic sharding and unsharding of directories (#88) ([ipfs/go-mfs#88](https://github.com/ipfs/go-mfs/pull/88))
  - sync: update CI config files (#94) ([ipfs/go-mfs#94](https://github.com/ipfs/go-mfs/pull/94))
  - Fix lint errors ([ipfs/go-mfs#90](https://github.com/ipfs/go-mfs/pull/90))
  - remove Makefile ([ipfs/go-mfs#89](https://github.com/ipfs/go-mfs/pull/89))
- github.com/ipfs/go-namesys (v0.3.1 -> v0.4.0):
  - Release v0.4.0
  - feat: plumb through datastore contexts
  - sync: update CI config files (#23) ([ipfs/go-namesys#23](https://github.com/ipfs/go-namesys/pull/23))
- github.com/ipfs/go-path (v0.1.2 -> v0.2.1):
  - Version 0.2.1
  - Version 0.2.0 (#48) ([ipfs/go-path#48](https://github.com/ipfs/go-path/pull/48))
  - feat: plumb through context changes (#47) ([ipfs/go-path#47](https://github.com/ipfs/go-path/pull/47))
  - sync: update CI config files (#46) ([ipfs/go-path#46](https://github.com/ipfs/go-path/pull/46))
  - Revert "feat: plumb through context changes"
  - feat: plumb through context changes
- github.com/ipfs/go-peertaskqueue (v0.4.0 -> v0.7.0):
  - feat: optimize checking if a new task is "better" ([ipfs/go-peertaskqueue#19](https://github.com/ipfs/go-peertaskqueue/pull/19))
  - Adds customizable prioritization logic for peertracker and peertaskqueue ([ipfs/go-peertaskqueue#17](https://github.com/ipfs/go-peertaskqueue/pull/17))
  - When priority is equal, use FIFO ([ipfs/go-peertaskqueue#16](https://github.com/ipfs/go-peertaskqueue/pull/16))
- github.com/ipfs/go-unixfs (v0.2.5 -> v0.3.1):
  - Version 0.3.1
  - Version 0.3.0 (#114) ([ipfs/go-unixfs#114](https://github.com/ipfs/go-unixfs/pull/114))
  - feat: plumb through datastore context changes
  - Size-based unsharding (#94) ([ipfs/go-unixfs#94](https://github.com/ipfs/go-unixfs/pull/94))
  - sync: update CI config files (#112) ([ipfs/go-unixfs#112](https://github.com/ipfs/go-unixfs/pull/112))
  - chore(deps): move bitfield to ipfs org ([ipfs/go-unixfs#98](https://github.com/ipfs/go-unixfs/pull/98))
  - fix staticcheck ([ipfs/go-unixfs#95](https://github.com/ipfs/go-unixfs/pull/95))
  - fix(directory): initialize size when computing it ([ipfs/go-unixfs#93](https://github.com/ipfs/go-unixfs/pull/93))
  - fix: always return upgradeable instead of basic dir (#92) ([ipfs/go-unixfs#92](https://github.com/ipfs/go-unixfs/pull/92))
  - feat: switch to HAMT based on size (#91) ([ipfs/go-unixfs#91](https://github.com/ipfs/go-unixfs/pull/91))
  - go fmt
  - fix: add pointer receiver
  - add test
  - feat: add UpgradeableDirectory
- github.com/ipfs/interface-go-ipfs-core (v0.5.1 -> v0.5.2):
  - fix: check errors by string ([ipfs/interface-go-ipfs-core#76](https://github.com/ipfs/interface-go-ipfs-core/pull/76))
- github.com/ipfs/tar-utils (v0.0.1 -> v0.0.2):
  - Release v0.0.2 (#8) ([ipfs/tar-utils#8](https://github.com/ipfs/tar-utils/pull/8))
  - sync: update CI config files ([ipfs/tar-utils#7](https://github.com/ipfs/tar-utils/pull/7))
  - sync: update CI config files (#6) ([ipfs/tar-utils#6](https://github.com/ipfs/tar-utils/pull/6))
  - allow .. in file and directory names ([ipfs/tar-utils#5](https://github.com/ipfs/tar-utils/pull/5))
- github.com/ipld/go-car (v0.3.1 -> v0.3.2):
  - Expose selector traversal options for SelectiveCar ([ipld/go-car#251](https://github.com/ipld/go-car/pull/251))
  - Implement API to allow replacing root CIDs in a CARv1 or CARv2
  - blockstore: OpenReadWrite should not modify if it refuses to resume
  - clarify the relation between StoreIdentityCIDs and SetFullyIndexed
  - Implement options to handle `IDENTITY` CIDs gracefully
  - Combine API options for simplicity and logical coherence
  - Add test script for car verify (#236) ([ipld/go-car#236](https://github.com/ipld/go-car/pull/236))
  - cmd/car: add first testscript tests
  - integrate `car/` cli into `cmd/car` (#233) ([ipld/go-car#233](https://github.com/ipld/go-car/pull/233))
  - Add `car get-dag` command (#232) ([ipld/go-car#232](https://github.com/ipld/go-car/pull/232))
  - Separate CLI to separate module (#231) ([ipld/go-car#231](https://github.com/ipld/go-car/pull/231))
  - add `get block` to car cli (#230) ([ipld/go-car#230](https://github.com/ipld/go-car/pull/230))
  - use file size when loading from v1 car (#229) ([ipld/go-car#229](https://github.com/ipld/go-car/pull/229))
  - add interface describing iteration (#228) ([ipld/go-car#228](https://github.com/ipld/go-car/pull/228))
  - Add `list` and `filter` commands (#227) ([ipld/go-car#227](https://github.com/ipld/go-car/pull/227))
  - Add `car split` command (#226) ([ipld/go-car#226](https://github.com/ipld/go-car/pull/226))
  - Make `MultihashIndexSorted` the default index codec for CARv2
  - Add carve utility for updating the index of a car{v1,v2} file (#219) ([ipld/go-car#219](https://github.com/ipld/go-car/pull/219))
  - Ignore records with `IDENTITY` CID in `IndexSorted`
  - Fix index GetAll infinite loop if function always returns `true`
  - Expose the ability to iterate over records in `MultihasIndexSorted`
  - avoid another alloc per read byte
  - avoid allocating on every byte read
  - Implement new index type that also includes mutltihash code
  - Return `nil` as Index reader when reading indexless CARv2
  - Assert `OpenReader` from file does not panic after closure
  - Document performance caveats of `ExtractV1File` and address comments
  - Implement utility to extract CARv1 from a CARv2
  - v2/blockstore: add ReadWrite.Discard
  - update LICENSE files to point to the new gateway
  - re-add root LICENSE file
  - v2: stop using a symlink for LICENSE.md
  - Update the readme with link to examples
  - update package godocs and root level README for v2
  - blockstore: stop embedding ReadOnly in ReadWrite
  - Implement version agnostic streaming CAR block iterator
  - blockstore: use errors when API contracts are broken
  - add the first read-only benchmarks
  - Implement reader block iterator over CARv1 or CARv2
  - Propagate async `blockstore.AllKeysChan` errors via context
  - Add zero-length sections as EOF option to internal CARv1 reader
  - Improve error handing in tests
  - Allow `ReadOption`s to be set when getting or generating index
  - Use `ioutil.TempFile` to simplify file creation in index example
  - Avoid writing to files in testdata
  - blockstore: implement UseWholeCIDs
  - Merge wip-v2 into master (#178) ([ipld/go-car#178](https://github.com/ipld/go-car/pull/178))
- github.com/ipld/go-ipld-prime (v0.12.2 -> v0.14.2):
  - dagcbor: coerce undef to null. ([ipld/go-ipld-prime#308](https://github.com/ipld/go-ipld-prime/pull/308))
  - fluent: add toInterface (#304) ([ipld/go-ipld-prime#304](https://github.com/ipld/go-ipld-prime/pull/304))
  - traversal: s/Walk/WalkLocal/
  - traversal: add a primitive walk function.
  - Remove dependency to `go-wish`
  - mark v0.14.0
  -  ([ipld/go-ipld-prime#279](https://github.com/ipld/go-ipld-prime/pull/279))
  - Port `traversal` package tests to quicktest
  - Port `codec` package tests to quicktest
  - changelog: backfill.
  - Gracefully handle `TypedNode` with `nil` type of kind `Map`
  - Gracefully print typed nodes with `nil` type
  - Implement handling of `Link` and `[]byte` in `printer` (#294) ([ipld/go-ipld-prime#294](https://github.com/ipld/go-ipld-prime/pull/294))
  - changelog: backfill for the v0.12.x series.
  - readme: introduce a migration guide.
  - Port `fluent` package tests to quicktest
  - Port `datamodel` package tests to quicktest
  - Port `adl` package tests to quicktest
  - Port `node` package tests to quicktest
  - node/bindnode: support links in ProduceGoTypes
  - bump CI to Go 1.16 and 1.17
  - node/bindnode: support links in schema-type verification
  - node/bindnode: export ProduceGoTypes
  - all: fix "an" typos after the ipld->datamodel refactor
  - node/bindnode: fix test code after two PR merges
  - add LoadSchema APIs to the root package
  - storage: add 'Has' feature. ([ipld/go-ipld-prime#276](https://github.com/ipld/go-ipld-prime/pull/276))
  - node/bindnode: start verifying schema compatibility
  - linking: add LoadRaw and LoadPlusRaw functions to LinkSystem. ([ipld/go-ipld-prime#267](https://github.com/ipld/go-ipld-prime/pull/267))
  - node/bindnode: add support for lists behind kinded unions
  - node/bindnode: also run TestPrototype with just schemas
  - node/bindnode: polish a few TODO panics away
  - node/bindnode: add support for all scalars behind kinded unions
  - node/bindnode: get closer to passing the Links schema tests
  - start using Rod's schema tests from ipld/ipld
  - fully support parsing, encoding, and decoding the schema-schema
  - node/bindnode: add native support for cid.Cid
  - A more Featureful Approach to Storage APIs ([ipld/go-ipld-prime#265](https://github.com/ipld/go-ipld-prime/pull/265))
  - Add a cidlink.Memory storage option (#266) ([ipld/go-ipld-prime#266](https://github.com/ipld/go-ipld-prime/pull/266))
  - Improve docs for AssignNode; and datamodel.Copy function. ([ipld/go-ipld-prime#264](https://github.com/ipld/go-ipld-prime/pull/264))
  - schemadsl: assign the struct representation.
  - schema,tests,gen/go: more tests, gen union fixes. ([ipld/go-ipld-prime#257](https://github.com/ipld/go-ipld-prime/pull/257))
  - fix: deal with LinkRevisit->LinkVisitOnlyOnce change
  - traversal: the link-visit-only-once behavior should require opt-in, rather than defaulting to on.
  - chore: add LinkRevisit:false traversal test
  - traversal: track seen links, and revisit only if configured to do so.
  - fix: use datamodel.Node selectors
  - Revert encode round-trip to leave unencoded node test intact
  - Add more walk tests, including tests for use of SkipMe
  - Round-trip test nodes through custom codec to ensure stability
  - Don't abort block processing when encountering SkipMe
  - traversal: implement monotonically decrementing budgets. ([ipld/go-ipld-prime#260](https://github.com/ipld/go-ipld-prime/pull/260))
  - Use datamodel.Node for "Common" selector variants
  - schema/dmt: first pass at a parser ([ipld/go-ipld-prime#253](https://github.com/ipld/go-ipld-prime/pull/253))
  - drop codectools.
  - drop jst codec.  It lives in https://github.com/warpfork/go-jst/ now.
  - drop dagjson2.
  - fix(traversal): properly wrap errors
  - printer: empty maps and lists and structs should stay on one line.
  - schema: turn TypeName into an alias
  - schema/dmt: sync with schema-schema changes, finish Compile
  - schema: add ways to set and access the ImplicitValue for a struct field.
  - schema: accessor for TypeEnum.Representation.
  - schema: finish minimum viable support for describing enum types.
- github.com/libp2p/go-conn-security-multistream (v0.2.1 -> v0.3.0):
  - use the new SecureTransport and SecureMuxer interfaces (#36) ([libp2p/go-conn-security-multistream#36](https://github.com/libp2p/go-conn-security-multistream/pull/36))
  - fix go vet and staticcheck ([libp2p/go-conn-security-multistream#33](https://github.com/libp2p/go-conn-security-multistream/pull/33))
- github.com/libp2p/go-libp2p (v0.15.0 -> v0.16.0):
  - release v0.16.0 ([libp2p/go-libp2p#1246](https://github.com/libp2p/go-libp2p/pull/1246))
  - allow the ping protocol on transient connections ([libp2p/go-libp2p#1244](https://github.com/libp2p/go-libp2p/pull/1244))
  - make the Type field required in the HolePunch protobuf ([libp2p/go-libp2p#1241](https://github.com/libp2p/go-libp2p/pull/1241))
  - reject hole punching attempts when we don't have any public addresses ([libp2p/go-libp2p#1214](https://github.com/libp2p/go-libp2p/pull/1214))
  - refactor the AutoRelay code ([libp2p/go-libp2p#1240](https://github.com/libp2p/go-libp2p/pull/1240))
  - remove dead API link in README ([libp2p/go-libp2p#1233](https://github.com/libp2p/go-libp2p/pull/1233))
  - pass static relays to EnableAutoRelay, deprecate libp2p.StaticRelays and libp2p.DefaultStaticRelays ([libp2p/go-libp2p#1239](https://github.com/libp2p/go-libp2p/pull/1239))
  - feat: plumb through peerstore context changes (#1237) ([libp2p/go-libp2p#1237](https://github.com/libp2p/go-libp2p/pull/1237))
  - emit the EvtPeerConnectednessChanged event ([libp2p/go-libp2p#1230](https://github.com/libp2p/go-libp2p/pull/1230))
  - update go-libp2p-swarm to v0.7.0 ([libp2p/go-libp2p#1226](https://github.com/libp2p/go-libp2p/pull/1226))
  - sync: update CI config files (#1225) ([libp2p/go-libp2p#1225](https://github.com/libp2p/go-libp2p/pull/1225))
  - simplify circuitv2 package structure ([libp2p/go-libp2p#1224](https://github.com/libp2p/go-libp2p/pull/1224))
  - use a random string for the mDNS peer-name ([libp2p/go-libp2p#1222](https://github.com/libp2p/go-libp2p/pull/1222))
  - remove {Un}RegisterNotifee functions from mDNS service ([libp2p/go-libp2p#1220](https://github.com/libp2p/go-libp2p/pull/1220))
  - fix structured logging in holepunch coordination ([libp2p/go-libp2p#1213](https://github.com/libp2p/go-libp2p/pull/1213))
  - fix flaky TestStBackpressureStreamWrite test ([libp2p/go-libp2p#1212](https://github.com/libp2p/go-libp2p/pull/1212))
  - properly close hosts in mDNS tests ([libp2p/go-libp2p#1216](https://github.com/libp2p/go-libp2p/pull/1216))
  - close the ObserverAddrManager when the ID service is closed ([libp2p/go-libp2p#1218](https://github.com/libp2p/go-libp2p/pull/1218))
  - make it possible to pass options to a transport constructor ([libp2p/go-libp2p#1205](https://github.com/libp2p/go-libp2p/pull/1205))
  - remove goprocess from the NATManager ([libp2p/go-libp2p#1193](https://github.com/libp2p/go-libp2p/pull/1193))
  - add an option to start the relay v2 ([libp2p/go-libp2p#1197](https://github.com/libp2p/go-libp2p/pull/1197))
  - fix flaky TestFastDisconnect identify test ([libp2p/go-libp2p#1200](https://github.com/libp2p/go-libp2p/pull/1200))
  - chore: update go-tcp-transport to v0.3.0 ([libp2p/go-libp2p#1203](https://github.com/libp2p/go-libp2p/pull/1203))
  - fix: skip variadic params in constructors ([libp2p/go-libp2p#1204](https://github.com/libp2p/go-libp2p/pull/1204))
  - fix flaky BasicHost tests ([libp2p/go-libp2p#1202](https://github.com/libp2p/go-libp2p/pull/1202))
  - remove dependency on github.com/ipfs/go-detect-race ([libp2p/go-libp2p#1201](https://github.com/libp2p/go-libp2p/pull/1201))
  - fix flaky TestEndToEndSimConnect holepunching test ([libp2p/go-libp2p#1191](https://github.com/libp2p/go-libp2p/pull/1191))
  - autorelay support for circuitv2 relays (#1198) ([libp2p/go-libp2p#1198](https://github.com/libp2p/go-libp2p/pull/1198))
  - reject circuitv2 reservations with nonsensical expiration times ([libp2p/go-libp2p#1199](https://github.com/libp2p/go-libp2p/pull/1199))
  - Tag relay hops in relay implementations ([libp2p/go-libp2p#1188](https://github.com/libp2p/go-libp2p/pull/1188))
  - Add standalone implementation of v1 Relay (#1186) ([libp2p/go-libp2p#1186](https://github.com/libp2p/go-libp2p/pull/1186))
  - remove the context from the libp2p and the Host constructor ([libp2p/go-libp2p#1190](https://github.com/libp2p/go-libp2p/pull/1190))
  - don't use a context to shut down the circuitv2 ([libp2p/go-libp2p#1185](https://github.com/libp2p/go-libp2p/pull/1185))
  - fix: remove v1 go-log dep ([libp2p/go-libp2p#1189](https://github.com/libp2p/go-libp2p/pull/1189))
  - don't use the context to shut down the relay ([libp2p/go-libp2p#1184](https://github.com/libp2p/go-libp2p/pull/1184))
  - Use circuitv2 code (#1183) ([libp2p/go-libp2p#1183](https://github.com/libp2p/go-libp2p/pull/1183))
  - clean up badges in README ([libp2p/go-libp2p#1179](https://github.com/libp2p/go-libp2p/pull/1179))
  - remove recommendation about Go module proxy from README ([libp2p/go-libp2p#1180](https://github.com/libp2p/go-libp2p/pull/1180))
  - merge branch 'hole-punching'
  - don't use a context for closing the ObservedAddrManager ([libp2p/go-libp2p#1175](https://github.com/libp2p/go-libp2p/pull/1175))
  - move the circuit v2 code here ([libp2p/go-libp2p#1174](https://github.com/libp2p/go-libp2p/pull/1174))
  - make QUIC a default transport ([libp2p/go-libp2p#1128](https://github.com/libp2p/go-libp2p/pull/1128))
  - stop using jbenet/go-cienv ([libp2p/go-libp2p#1176](https://github.com/libp2p/go-libp2p/pull/1176))
  - fix flaky TestObsAddrSet test ([libp2p/go-libp2p#1172](https://github.com/libp2p/go-libp2p/pull/1172))
  - clean up messy defer logic in IDService.sendIdentifyResp ([libp2p/go-libp2p#1169](https://github.com/libp2p/go-libp2p/pull/1169))
  - remove secio from README, add noise ([libp2p/go-libp2p#1165](https://github.com/libp2p/go-libp2p/pull/1165))
- github.com/libp2p/go-libp2p-asn-util (v0.0.0-20200825225859-85005c6cf052 -> v0.1.0):
  - Update from upstream and make regeneration easier (#17) ([libp2p/go-libp2p-asn-util#17](https://github.com/libp2p/go-libp2p-asn-util/pull/17))
  - add license file so it can be found by go-licenses ([libp2p/go-libp2p-asn-util#10](https://github.com/libp2p/go-libp2p-asn-util/pull/10))
  - refactor: rename ASN table files ([libp2p/go-libp2p-asn-util#9](https://github.com/libp2p/go-libp2p-asn-util/pull/9))
  - Library for IP -> ASN mapping ([libp2p/go-libp2p-asn-util#1](https://github.com/libp2p/go-libp2p-asn-util/pull/1))
- github.com/libp2p/go-libp2p-autonat (v0.4.2 -> v0.6.0):
  - Version 0.6.0 (#112) ([libp2p/go-libp2p-autonat#112](https://github.com/libp2p/go-libp2p-autonat/pull/112))
  - feat: plumb through contexts from peerstore (#111) ([libp2p/go-libp2p-autonat#111](https://github.com/libp2p/go-libp2p-autonat/pull/111))
  - sync: update CI config files (#110) ([libp2p/go-libp2p-autonat#110](https://github.com/libp2p/go-libp2p-autonat/pull/110))
  - remove context from constructor, implement a proper Close method ([libp2p/go-libp2p-autonat#109](https://github.com/libp2p/go-libp2p-autonat/pull/109))
  - fix stream deadlines ([libp2p/go-libp2p-autonat#107](https://github.com/libp2p/go-libp2p-autonat/pull/107))
  - disable failing integration test ([libp2p/go-libp2p-autonat#108](https://github.com/libp2p/go-libp2p-autonat/pull/108))
  - fix staticcheck ([libp2p/go-libp2p-autonat#103](https://github.com/libp2p/go-libp2p-autonat/pull/103))
- github.com/libp2p/go-libp2p-core (v0.9.0 -> v0.11.0):
  - release v0.11.0 (#217) ([libp2p/go-libp2p-core#217](https://github.com/libp2p/go-libp2p-core/pull/217))
  - remove the ConnHandler (#214) ([libp2p/go-libp2p-core#214](https://github.com/libp2p/go-libp2p-core/pull/214))
  - sync: update CI config files (#216) ([libp2p/go-libp2p-core#216](https://github.com/libp2p/go-libp2p-core/pull/216))
  - remove the Process from the Network interface (#212) ([libp2p/go-libp2p-core#212](https://github.com/libp2p/go-libp2p-core/pull/212))
  - pass the peer ID to SecureInbound in the SecureTransport and SecureMuxer (#211) ([libp2p/go-libp2p-core#211](https://github.com/libp2p/go-libp2p-core/pull/211))
  - save the role (client, server) in the simultaneous connect context (#210) ([libp2p/go-libp2p-core#210](https://github.com/libp2p/go-libp2p-core/pull/210))
  - sync: update CI config files (#209) ([libp2p/go-libp2p-core#209](https://github.com/libp2p/go-libp2p-core/pull/209))
- github.com/libp2p/go-libp2p-discovery (v0.5.1 -> v0.6.0):
  - feat: plumb peerstore contexts changes through (#75) ([libp2p/go-libp2p-discovery#75](https://github.com/libp2p/go-libp2p-discovery/pull/75))
  - remove deprecated types ([libp2p/go-libp2p-discovery#73](https://github.com/libp2p/go-libp2p-discovery/pull/73))
- github.com/libp2p/go-libp2p-kad-dht (v0.13.1 -> v0.15.0):
  - Bump version to 0.15.0 (#755) ([libp2p/go-libp2p-kad-dht#755](https://github.com/libp2p/go-libp2p-kad-dht/pull/755))
  - sync: update CI config files (#754) ([libp2p/go-libp2p-kad-dht#754](https://github.com/libp2p/go-libp2p-kad-dht/pull/754))
  - feat: plumb through datastore contexts (#753) ([libp2p/go-libp2p-kad-dht#753](https://github.com/libp2p/go-libp2p-kad-dht/pull/753))
  - custom ProviderManager that brokers AddrInfos (#751) ([libp2p/go-libp2p-kad-dht#751](https://github.com/libp2p/go-libp2p-kad-dht/pull/751))
  - feat: make compatible with go-libp2p 0.15 ([libp2p/go-libp2p-kad-dht#747](https://github.com/libp2p/go-libp2p-kad-dht/pull/747))
  - sync: update CI config files ([libp2p/go-libp2p-kad-dht#743](https://github.com/libp2p/go-libp2p-kad-dht/pull/743))
  - Disallow GetPublicKey when DisableValues is passed ([libp2p/go-libp2p-kad-dht#604](https://github.com/libp2p/go-libp2p-kad-dht/pull/604))
- github.com/libp2p/go-libp2p-nat (v0.0.6 -> v0.1.0):
  - remove Codecov config file ([libp2p/go-libp2p-nat#39](https://github.com/libp2p/go-libp2p-nat/pull/39))
  - stop using goprocess for shutdown ([libp2p/go-libp2p-nat#38](https://github.com/libp2p/go-libp2p-nat/pull/38))
  - chore: update go-log ([libp2p/go-libp2p-nat#37](https://github.com/libp2p/go-libp2p-nat/pull/37))
  - remove unused field permanent from mapping ([libp2p/go-libp2p-nat#33](https://github.com/libp2p/go-libp2p-nat/pull/33))
- github.com/libp2p/go-libp2p-noise (v0.2.2 -> v0.3.0):
  - add the peer ID to SecureInbound ([libp2p/go-libp2p-noise#104](https://github.com/libp2p/go-libp2p-noise/pull/104))
  - update go-libp2p-core, remove integration test ([libp2p/go-libp2p-noise#102](https://github.com/libp2p/go-libp2p-noise/pull/102))
- github.com/libp2p/go-libp2p-peerstore (v0.2.8 -> v0.4.0):
  - Update version.json (#178) ([libp2p/go-libp2p-peerstore#178](https://github.com/libp2p/go-libp2p-peerstore/pull/178))
  - limit the number of protocols we store per peer ([libp2p/go-libp2p-peerstore#172](https://github.com/libp2p/go-libp2p-peerstore/pull/172))
  - sync: update CI config files (#177) ([libp2p/go-libp2p-peerstore#177](https://github.com/libp2p/go-libp2p-peerstore/pull/177))
  - feat: plumb through datastore contexts (#176) ([libp2p/go-libp2p-peerstore#176](https://github.com/libp2p/go-libp2p-peerstore/pull/176))
  - remove leftover peerstore implementation in the root package ([libp2p/go-libp2p-peerstore#173](https://github.com/libp2p/go-libp2p-peerstore/pull/173))
  - fix: replace deprecated call ([libp2p/go-libp2p-peerstore#168](https://github.com/libp2p/go-libp2p-peerstore/pull/168))
  - feat: remove queue ([libp2p/go-libp2p-peerstore#166](https://github.com/libp2p/go-libp2p-peerstore/pull/166))
  - remove deprecated types ([libp2p/go-libp2p-peerstore#165](https://github.com/libp2p/go-libp2p-peerstore/pull/165))
- github.com/libp2p/go-libp2p-pubsub (v0.5.4 -> v0.6.0):
  - feat: plumb through context changes (#459) ([libp2p/go-libp2p-pubsub#459](https://github.com/libp2p/go-libp2p-pubsub/pull/459))
  - support MinTopicSize without a discovery mechanism
  - clear peerPromises map when fullfilling a promise
  - README: remove obsolete notice, fix example code for tracing.
  - remove peer filter check from subscriptions (#453) ([libp2p/go-libp2p-pubsub#453](https://github.com/libp2p/go-libp2p-pubsub/pull/453))
  - Create peer filter option
- github.com/libp2p/go-libp2p-pubsub-router (v0.4.0 -> v0.5.0):
  - Version 0.5.0
  - feat: plumb through datastore contexts
  - sync: update CI config files (#86) ([libp2p/go-libp2p-pubsub-router#86](https://github.com/libp2p/go-libp2p-pubsub-router/pull/86))
  - Remove arbitrary sleeps from tests ([libp2p/go-libp2p-pubsub-router#87](https://github.com/libp2p/go-libp2p-pubsub-router/pull/87))
  - cleanup: fix staticcheck failures ([libp2p/go-libp2p-pubsub-router#84](https://github.com/libp2p/go-libp2p-pubsub-router/pull/84))
  - Add WithDatastore option. ([libp2p/go-libp2p-pubsub-router#82](https://github.com/libp2p/go-libp2p-pubsub-router/pull/82))
- github.com/libp2p/go-libp2p-quic-transport (v0.12.0 -> v0.15.0):
  - release v0.15.0 (#241) ([libp2p/go-libp2p-quic-transport#241](https://github.com/libp2p/go-libp2p-quic-transport/pull/241))
  - reuse the same router until we change listeners ([libp2p/go-libp2p-quic-transport#240](https://github.com/libp2p/go-libp2p-quic-transport/pull/240))
  - release v0.14.0 ([libp2p/go-libp2p-quic-transport#237](https://github.com/libp2p/go-libp2p-quic-transport/pull/237))
  - fix error assertions in the tracer ([libp2p/go-libp2p-quic-transport#234](https://github.com/libp2p/go-libp2p-quic-transport/pull/234))
  - sync: update CI config files (#235) ([libp2p/go-libp2p-quic-transport#235](https://github.com/libp2p/go-libp2p-quic-transport/pull/235))
  - read the client option from the simultaneous connect context ([libp2p/go-libp2p-quic-transport#230](https://github.com/libp2p/go-libp2p-quic-transport/pull/230))
- github.com/libp2p/go-libp2p-swarm (v0.5.3 -> v0.8.0):
  - Version 0.8.0 (#292) ([libp2p/go-libp2p-swarm#292](https://github.com/libp2p/go-libp2p-swarm/pull/292))
  - feat: plumb contexts through from peerstore (#290) ([libp2p/go-libp2p-swarm#290](https://github.com/libp2p/go-libp2p-swarm/pull/290))
  - release v0.7.0 ([libp2p/go-libp2p-swarm#289](https://github.com/libp2p/go-libp2p-swarm/pull/289))
  - update go-tcp-transport to v0.4.0 ([libp2p/go-libp2p-swarm#287](https://github.com/libp2p/go-libp2p-swarm/pull/287))
  - remove the ConnHandler ([libp2p/go-libp2p-swarm#286](https://github.com/libp2p/go-libp2p-swarm/pull/286))
  - sync: update CI config files (#288) ([libp2p/go-libp2p-swarm#288](https://github.com/libp2p/go-libp2p-swarm/pull/288))
  - remove a lot of incorrect statements from the README ([libp2p/go-libp2p-swarm#284](https://github.com/libp2p/go-libp2p-swarm/pull/284))
  - unexport the DialSync ([libp2p/go-libp2p-swarm#281](https://github.com/libp2p/go-libp2p-swarm/pull/281))
  - add an error return value to the constructor ([libp2p/go-libp2p-swarm#280](https://github.com/libp2p/go-libp2p-swarm/pull/280))
  - use functional options to configure the swarm ([libp2p/go-libp2p-swarm#279](https://github.com/libp2p/go-libp2p-swarm/pull/279))
  - stop using goprocess to control teardown ([libp2p/go-libp2p-swarm#278](https://github.com/libp2p/go-libp2p-swarm/pull/278))
  - read and use the direction from the simultaneous connect context ([libp2p/go-libp2p-swarm#277](https://github.com/libp2p/go-libp2p-swarm/pull/277))
  - simplify the DialSync code ([libp2p/go-libp2p-swarm#272](https://github.com/libp2p/go-libp2p-swarm/pull/272))
  - remove redundant self-dialing check, simplify starting of dialWorkerLoop ([libp2p/go-libp2p-swarm#273](https://github.com/libp2p/go-libp2p-swarm/pull/273))
  - add a test case for the testing package ([libp2p/go-libp2p-swarm#276](https://github.com/libp2p/go-libp2p-swarm/pull/276))
  - simplify limiter by removing the injected isFdConsumingFnc ([libp2p/go-libp2p-swarm#274](https://github.com/libp2p/go-libp2p-swarm/pull/274))
  - update badges ([libp2p/go-libp2p-swarm#271](https://github.com/libp2p/go-libp2p-swarm/pull/271))
  - remove unused context in Swarm.dialWorkerLoop ([libp2p/go-libp2p-swarm#268](https://github.com/libp2p/go-libp2p-swarm/pull/268))
  - remove Codecov config ([libp2p/go-libp2p-swarm#270](https://github.com/libp2p/go-libp2p-swarm/pull/270))
  - fix race condition in TestFailFirst ([libp2p/go-libp2p-swarm#269](https://github.com/libp2p/go-libp2p-swarm/pull/269))
- github.com/libp2p/go-libp2p-testing (v0.4.2 -> v0.5.0):
  - chore: update go-libp2p-core to v0.10.0 ([libp2p/go-libp2p-testing#38](https://github.com/libp2p/go-libp2p-testing/pull/38))
  - sync: update CI config files (#37) ([libp2p/go-libp2p-testing#37](https://github.com/libp2p/go-libp2p-testing/pull/37))
- github.com/libp2p/go-libp2p-tls (v0.2.0 -> v0.3.1):
  - release v0.3.1 ([libp2p/go-libp2p-tls#101](https://github.com/libp2p/go-libp2p-tls/pull/101))
  - set a random certificate subject ([libp2p/go-libp2p-tls#100](https://github.com/libp2p/go-libp2p-tls/pull/100))
  - sync: update CI config files (#96) ([libp2p/go-libp2p-tls#96](https://github.com/libp2p/go-libp2p-tls/pull/96))
  - add the peer ID to SecureInbound ([libp2p/go-libp2p-tls#94](https://github.com/libp2p/go-libp2p-tls/pull/94))
  - sync: update CI config files ([libp2p/go-libp2p-tls#91](https://github.com/libp2p/go-libp2p-tls/pull/91))
- github.com/libp2p/go-libp2p-transport-upgrader (v0.4.6 -> v0.5.0):
  - increase timeout in TestConnectionsClosedIfNotAccepted on CI ([libp2p/go-libp2p-transport-upgrader#85](https://github.com/libp2p/go-libp2p-transport-upgrader/pull/85))
  - add the peer ID to SecureInbound ([libp2p/go-libp2p-transport-upgrader#83](https://github.com/libp2p/go-libp2p-transport-upgrader/pull/83))
- github.com/libp2p/go-msgio (v0.0.6 -> v0.1.0):
  - sync: update CI config files (#27) ([libp2p/go-msgio#27](https://github.com/libp2p/go-msgio/pull/27))
  - remove .gxignore file ([libp2p/go-msgio#24](https://github.com/libp2p/go-msgio/pull/24))
  - remove Codecov config ([libp2p/go-msgio#26](https://github.com/libp2p/go-msgio/pull/26))
  - remove "Chan" type ([libp2p/go-msgio#23](https://github.com/libp2p/go-msgio/pull/23))
- github.com/libp2p/go-nat (v0.0.5 -> v0.1.0):
  - pass a context to DiscoverGateway ([libp2p/go-nat#23](https://github.com/libp2p/go-nat/pull/23))
- github.com/libp2p/go-reuseport (v0.0.2 -> v0.1.0):
  - stop using github.com/pkg/errors ([libp2p/go-reuseport#85](https://github.com/libp2p/go-reuseport/pull/85))
  - sync: update CI config files (#84) ([libp2p/go-reuseport#84](https://github.com/libp2p/go-reuseport/pull/84))
- github.com/libp2p/go-reuseport-transport (v0.0.5 -> v0.1.0):
  - remove Codecov config file ([libp2p/go-reuseport-transport#36](https://github.com/libp2p/go-reuseport-transport/pull/36))
  - chore: update go-log to v2 ([libp2p/go-reuseport-transport#35](https://github.com/libp2p/go-reuseport-transport/pull/35))
  - sync: update CI config files ([libp2p/go-reuseport-transport#31](https://github.com/libp2p/go-reuseport-transport/pull/31))
- github.com/libp2p/go-tcp-transport (v0.2.8 -> v0.4.0):
  - release v0.4.0 ([libp2p/go-tcp-transport#108](https://github.com/libp2p/go-tcp-transport/pull/108))
  - sync: update CI config files (#107) ([libp2p/go-tcp-transport#107](https://github.com/libp2p/go-tcp-transport/pull/107))
  - remove the deprecated IPFS_REUSEPORT command line flag ([libp2p/go-tcp-transport#104](https://github.com/libp2p/go-tcp-transport/pull/104))
  - add options to the constructor ([libp2p/go-tcp-transport#99](https://github.com/libp2p/go-tcp-transport/pull/99))
  - remove the context from the libp2p constructor in README ([libp2p/go-tcp-transport#101](https://github.com/libp2p/go-tcp-transport/pull/101))
  - don't use libp2p.ChainOption in README ([libp2p/go-tcp-transport#102](https://github.com/libp2p/go-tcp-transport/pull/102))
  - remove incorrect statement about dns addresses in README ([libp2p/go-tcp-transport#100](https://github.com/libp2p/go-tcp-transport/pull/100))
  - use the assigned role when upgrading a sim open connection ([libp2p/go-tcp-transport#95](https://github.com/libp2p/go-tcp-transport/pull/95))
  - chore: update go-log to v2 ([libp2p/go-tcp-transport#97](https://github.com/libp2p/go-tcp-transport/pull/97))
  - simplify dial timeout context ([libp2p/go-tcp-transport#94](https://github.com/libp2p/go-tcp-transport/pull/94))
- github.com/libp2p/go-yamux/v2 (v2.2.0 -> v2.3.0):
  - limit the number of concurrent incoming streams ([libp2p/go-yamux#66](https://github.com/libp2p/go-yamux/pull/66))
  - drastically reduce allocations in ring buffer implementation (#64) ([libp2p/go-yamux#64](https://github.com/libp2p/go-yamux/pull/64))
  - sync: update CI config files (#63) ([libp2p/go-yamux#63](https://github.com/libp2p/go-yamux/pull/63))
  - remove call to asyncNotify in Stream.Read
- github.com/libp2p/zeroconf/v2 (v2.0.0 -> v2.1.1):
  - fix flaky TTL test ([libp2p/zeroconf#18](https://github.com/libp2p/zeroconf/pull/18))
  - implement a clean shutdown of the probe method ([libp2p/zeroconf#16](https://github.com/libp2p/zeroconf/pull/16))
  - remove dependency on the backoff library ([libp2p/zeroconf#17](https://github.com/libp2p/zeroconf/pull/17))
  - Don't stop browsing after ~15min ([libp2p/zeroconf#13](https://github.com/libp2p/zeroconf/pull/13))
  - fix delays when sending initial probe packets ([libp2p/zeroconf#14](https://github.com/libp2p/zeroconf/pull/14))
  - improve starting of mDNS service in tests, stop using pkg/errors ([libp2p/zeroconf#15](https://github.com/libp2p/zeroconf/pull/15))
  - update import path to include v2 in README ([libp2p/zeroconf#11](https://github.com/libp2p/zeroconf/pull/11))
- github.com/lucas-clemente/quic-go (v0.23.0 -> v0.24.0):
  - don't unlock the receive stream mutex for copying from STREAM frames ([lucas-clemente/quic-go#3290](https://github.com/lucas-clemente/quic-go/pull/3290))
  - List projects using quic-go ([lucas-clemente/quic-go#3266](https://github.com/lucas-clemente/quic-go/pull/3266))
  - disable Path MTU Discovery on Windows ([lucas-clemente/quic-go#3276](https://github.com/lucas-clemente/quic-go/pull/3276))
  - enter the regular run loop if no undecryptable packet was processed ([lucas-clemente/quic-go#3268](https://github.com/lucas-clemente/quic-go/pull/3268))
  - Allow use of custom port value in Alt-Svc header. ([lucas-clemente/quic-go#3272](https://github.com/lucas-clemente/quic-go/pull/3272))
  - disable the goconst linter ([lucas-clemente/quic-go#3286](https://github.com/lucas-clemente/quic-go/pull/3286))
  - use x/net/ipv{4,6} to construct oob info when writing packets (#3278) ([lucas-clemente/quic-go#3278](https://github.com/lucas-clemente/quic-go/pull/3278))
  - run gofmt to add the new go:build tags ([lucas-clemente/quic-go#3277](https://github.com/lucas-clemente/quic-go/pull/3277))
  - fix log string in client example ([lucas-clemente/quic-go#3264](https://github.com/lucas-clemente/quic-go/pull/3264))
- github.com/multiformats/go-multiaddr (v0.4.0 -> v0.4.1):
  - add the plaintextv2 protocol ([multiformats/go-multiaddr#165](https://github.com/multiformats/go-multiaddr/pull/165))
- github.com/multiformats/go-multihash (v0.0.15 -> v0.1.0):
  - bump version to v0.1.0 ([multiformats/go-multihash#151](https://github.com/multiformats/go-multihash/pull/151))
  - add version.json per tooling convention.
  - murmur3 support (#150) ([multiformats/go-multihash#150](https://github.com/multiformats/go-multihash/pull/150))
  - Add variations of sha2 ([multiformats/go-multihash#149](https://github.com/multiformats/go-multihash/pull/149))
  - don't use pointers for Multihash.String
  - Add blake3 hash and sharness tests ([multiformats/go-multihash#147](https://github.com/multiformats/go-multihash/pull/147))
  - remove Makefile ([multiformats/go-multihash#142](https://github.com/multiformats/go-multihash/pull/142))
  - fix staticcheck ([multiformats/go-multihash#141](https://github.com/multiformats/go-multihash/pull/141))
  - New SumStream function reads from io.Reader ([multiformats/go-multihash#138](https://github.com/multiformats/go-multihash/pull/138))
- github.com/warpfork/go-testmark (v0.3.0 -> v0.9.0):
  - testexec: will now always set up tmpdirs.
  - testexec: fix typo in error message.
  - testexec: subtest ("then-*") feature ([warpfork/go-testmark#7](https://github.com/warpfork/go-testmark/pull/7))
  - testexec: quote error from child; attribution better via more t.Helper.
  - Improve documentation of format.
  - Rename Hunk.BlockTag -> InfoString.
  - testexec: will now create tmpdirs and files for you if you have an 'fs' entry tree.
  - testexec: getting exit codes correctly. ([warpfork/go-testmark#6](https://github.com/warpfork/go-testmark/pull/6))
  - fix parsing CRLF files, part 3 ([warpfork/go-testmark#5](https://github.com/warpfork/go-testmark/pull/5))
  - fix parsing CRLF files, part 2 ([warpfork/go-testmark#4](https://github.com/warpfork/go-testmark/pull/4))
  - testexec: support both simple sequence and script mode.
  - Proper tests for read function.
  - avoid creeping extra linebreaks at the end of a patched document.
  - refrain from making double linebreaks when patching with content that ends in a linebreak.
  - Merge branch 'testexec'
  - add support for parsing CRLF line endings ([warpfork/go-testmark#3](https://github.com/warpfork/go-testmark/pull/3))
  - link to patch example code
  - More readme; and, parsing recommendations document.
  - Further improve readme.

### ❤️ Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Will | 13 | +73226/-130481 | 43 |
| Masih H. Derkani | 99 | +10549/-5799 | 489 |
| hannahhoward | 43 | +5515/-3293 | 233 |
| Daniel Martí | 60 | +5312/-2883 | 208 |
| Marten Seemann | 175 | +4839/-3254 | 396 |
| Eric Myhre | 73 | +3924/-3328 | 175 |
| Jessica Schilling | 52 | +2709/-2386 | 75 |
| Rod Vagg | 30 | +2719/-1703 | 79 |
| vyzo | 10 | +3516/-177 | 87 |
| Gus Eggert | 64 | +1677/-1416 | 147 |
| Adin Schmahmann | 23 | +1708/-381 | 95 |
| Lucas Molas | 14 | +1557/-365 | 48 |
| Will Scott | 7 | +1846/-15 | 34 |
| Steven Allen | 32 | +537/-897 | 56 |
| Cory Schwartz | 3 | +614/-109 | 12 |
| rht | 3 | +576/-4 | 7 |
| Simon Zhu | 9 | +352/-51 | 16 |
| Petar Maymounkov | 7 | +173/-167 | 23 |
| RubenKelevra | 1 | +107/-188 | 1 |
| jwh | 2 | +212/-80 | 7 |
| longfeiW | 1 | +4/-249 | 10 |
| guseggert | 5 | +230/-21 | 11 |
| Kevin Neaton | 8 | +137/-80 | 13 |
| Takashi Matsuda | 1 | +199/-0 | 5 |
| Andrey Kostakov | 1 | +107/-49 | 2 |
| Jesse Bouwman | 1 | +151/-0 | 7 |
| web3-bot | 39 | +136/-3 | 52 |
| Marcin Rataj | 16 | +62/-57 | 25 |
| Marco Munizaga | 1 | +118/-0 | 2 |
| Aaron Riekenberg | 4 | +64/-52 | 6 |
| Ian Davis | 4 | +81/-32 | 7 |
| Jorropo | 2 | +79/-19 | 6 |
| Mohsin Zaidi | 1 | +89/-1 | 20 |
| Andey Robins | 1 | +70/-3 | 3 |
| gammazero | 3 | +40/-25 | 4 |
| Steve Loeppky | 2 | +26/-27 | 3 |
| Dimitris Apostolou | 1 | +25/-25 | 15 |
| Sudarshan Reddy | 1 | +9/-40 | 1 |
| Richard Littauer | 2 | +42/-1 | 3 |
| pymq | 1 | +32/-8 | 2 |
| Dirk McCormick | 2 | +23/-1 | 2 |
| Nicholas Bollweg | 1 | +21/-0 | 1 |
| anorth | 1 | +14/-6 | 2 |
| Jack Loughran | 1 | +16/-0 | 2 |
| whyrusleeping | 2 | +11/-2 | 2 |
| bt90 | 1 | +13/-0 | 1 |
| Yi Cao | 1 | +10/-0 | 1 |
| Max | 1 | +7/-3 | 1 |
| Juan Batiz-Benet | 2 | +8/-2 | 2 |
| Keenan Nemetz | 1 | +8/-0 | 1 |
| muXxer | 1 | +3/-3 | 1 |
| galargh | 2 | +3/-3 | 3 |
| Didrik Nordström | 1 | +2/-4 | 1 |
| Ben Lubar | 1 | +3/-3 | 1 |
| arjunraghurama | 1 | +5/-0 | 1 |
| Whyrusleeping | 1 | +3/-2 | 1 |
| TUSF | 1 | +3/-2 | 3 |
| mathew-cf | 1 | +3/-1 | 2 |
| Stephen Whitmore | 1 | +2/-2 | 1 |
| Song Zhu | 1 | +2/-2 | 1 |
| Michael Muré | 1 | +4/-0 | 1 |
| Alex Good | 1 | +4/-0 | 2 |
| aarshkshah1992 | 1 | +2/-1 | 1 |
| susarlanikhilesh | 1 | +1/-1 | 1 |
| falstack | 1 | +1/-1 | 1 |
| Michael Vorburger ⛑️ | 1 | +1/-1 | 1 |
| Ismail Khoffi | 1 | +1/-1 | 1 |
| George Xie | 1 | +1/-1 | 1 |
| Bryan Stenson | 1 | +1/-1 | 1 |
| Lars Gierth | 1 | +1/-0 | 1 |

## v0.10.0 2021-09-30

We're happy to announce go-ipfs 0.10.0. This release brings some big changes to the IPLD internals of go-ipfs that make working with non-UnixFS DAGs easier than ever. There are also a variety of new commands and configuration options available.

As usual, this release includes important fixes, some of which may be critical for security. Unless the fix addresses a bug being exploited in the wild, the fix will _not_ be called out in the release notes. Please make sure to update ASAP. See our [release process](https://github.com/ipfs/go-ipfs/tree/master/docs/releases.md#security-fix-policy) for details.

### 🛠 TLDR: BREAKING CHANGES

- `ipfs dag get`
  - default output changed to [`dag-json`](https://ipld.io/specs/codecs/dag-json/spec/)
  - dag-pb (e.g. unixfs) field names changed - impacts userland code that works with `dag-pb` objects returned by `dag get`
  - no longer emits an additional new-line character at the end of the data output
- `ipfs dag put`
  - defaults changed to reduce ambiguity and surprises: input is now assumed to be [`dag-json`](https://ipld.io/specs/codecs/dag-json/spec/), and data is serialized to [`dag-cbor`](https://ipld.io/specs/codecs/dag-cbor/spec/) at rest.
  - `--format` and `--input-enc` were removed and replaced with `--store-codec` and `--input-codec`
  - codec names now match the ones defined in the [multicodec table](https://github.com/multiformats/multicodec/blob/master/table.csv) 
  - dag-pb (e.g. unixfs) field names changed - impacts userland code that works with `dag-pb` objects stored via `dag put`

Keep reading to learn more details.

### 🔦 Highlights

#### 🌲 IPLD Levels Up

The handling of data serialization as well as many aspects of DAG traversal and pathing have been migrated from older libraries, including [go-merkledag](https://github.com/ipfs/go-merkledag) and [go-ipld-format](https://github.com/ipfs/go-ipld-format) to the new **[go-ipld-prime](https://github.com/ipld/go-ipld-prime)** library and its components. This allows us to use many of the newer tools afforded by go-ipld-prime, stricter and more uniform codec implementations, support for additional (pluggable) codecs, and some minor performance improvements.

This is significant refactor of a core component that touches many parts of IPFS, and does come with some **breaking changes**:

* **IPLD plugins**:
  * The `PluginIPLD` interface has been changed to utilize go-ipld-prime. There is a demonstration of the change in the [bundled git plugin](./plugin/plugins/git/).
* **The semantics of `dag put` and `dag get` change**:
  * `dag get` now takes the `output-codec` option which accepts a [multicodec](https://docs.ipfs.io/concepts/glossary/#multicodec) name used to encode the output. By default this is `dag-json`, which is  a strict and deterministic subset of JSON created by the IPLD team. Users may notice differences from the previously plain Go JSON output, particularly where bytes are concerned which are now encoded using a form similar to CIDs: `{"/":{"bytes":"unpadded-base64-bytes"}}` rather than the previously Go-specific plain padded base64 string. See the [dag-json specification](https://ipld.io/specs/codecs/dag-json/spec/) for an explanation of these forms.
  * `dag get` no longer prints an additional new-line character at the end of the encoded block output. This means that the output as presented by `dag get` are the exact bytes of the requested node. A round-trip of such bytes back in through `dag put` using the same codec should result in the same CID.
  * `dag put` uses the `input-codec` option to specify the multicodec name of the format data is being provided in, and the `store-codec` option to specify the multicodec name of the format the data should be stored in at rest. These formerly defaulted to `json` and `cbor` respectively. They now default to `dag-json` and `dag-cbor` respectively but may be changed to any supported codec (bundled or loaded via plugin) by its [multicodec name](https://github.com/multiformats/multicodec/blob/master/table.csv).
  * The `json` and `cbor` multicodec names (as used by `input-enc` and `format` options) are now no longer aliases for `dag-json` and `dag-cbor` respectively. Instead, they now refer to their proper [multicodec](https://github.com/multiformats/multicodec/blob/master/table.csv) types. `cbor` refers to a plain CBOR format, which will not encode CIDs and does not have strict deterministic encoding rules. `json` is a plain JSON format, which also won't encode CIDs and will encode bytes in the Go-specific padded base64 string format rather than the dag-json method of byte encoding. See https://ipld.io/specs/codecs/ for more information on IPLD codecs.
  * `protobuf` is no longer used as the codec name for `dag-pb`
  * The codec name `raw` is used to mean Bytes in the [IPLD Data Model](https://github.com/ipld/specs/blob/master/data-model-layer/data-model.md#bytes-kind)
* **UnixFS refactor**. The **dag-pb codec**, which is used to encode UnixFS data for IPFS, is now represented through the `dag` API in a form that mirrors the protobuf schema used to define the binary format. This unifies the implementations and specification of dag-pb across the IPLD and IPFS stacks. Previously, additional layers of code for file and directory handling within IPFS between protobuf serialization and UnixFS obscured the protobuf representation. Much of this code has now been replaced and there are fewer layers of transformation. This means that interacting with dag-pb data via the `dag` API will use different forms:
  * Previously, using `dag get` on a dag-pb block would present the block serialized as JSON as `{"data":"padded-base64-bytes","links":[{"Name":"foo","Size":100,"Cid":{"/":"Qm..."}},...]}`.
  * Now,  the dag-pb data with dag-json codec for output will be serialized using the data model from the [dag-pb specification](https://ipld.io/specs/codecs/dag-pb/spec/): `{"Data":{"/":{"bytes":"unpadded-base64-bytes"}},"Links":[{"Name":"foo","Tsize":100,"Hash":{"/":"Qm..."}},...]}`. Aside from the change in byte formatting, most field names have changed: `data` → `Data`, `links` → `Links`, `Size` → `Tsize`, `Cid` → `Hash`. Note that this output can be changed now using the `output-codec` option to specify an alternative codec.
  * Similarly, using `dag put` and a `store-codec` option of `dag-pb` now requires that the input conform to this dag-pb specified form. Previously, input using `{"data":"...","links":[...]}` was accepted, now it must be `{"Data":"...","Links":[...]}`.
  * Previously it was not possible to use paths to navigate to any of these properties of a dag-pb node, the only possible paths were named links, e.g. `dag get QmFoo/NamedLink` where `NamedLink` was one of the links whose name was `NamedLink`. This functionality remains the same, but by prefixing the path with `/ipld/` we enter data model pathing semantics and can `dag get /ipld/QmFoo/Links/0/Hash` to navigate to links or `/ipld/QmFoo/Data` to simply retrieve the data section of the node, for example.
  * ℹ See the [dag-pb specification](https://ipld.io/specs/codecs/dag-pb/) for details on the codec and its data model representation.
  * ℹ See this [detailed write-up](https://github.com/ipld/ipld/blob/master/design/tricky-choices/dag-pb-forms-impl-and-use.md) for further background on these changes.

#### Ⓜ Multibase Command

go-ipfs now provides utility commands for working with [multibase](https://docs.ipfs.io/concepts/glossary/#multibase):

```console
$ echo -n hello | ipfs multibase encode -b base16 > file-mbase16
$ cat file-mbase16
f68656c6c6f

$ ipfs multibase decode file-mbase16
hello

$ cat file-mbase16 | ipfs multibase decode
hello

$ ipfs multibase transcode -b base2 file-mbase16
00110100001100101011011000110110001101111
```

See `ipfs multibase --help` for more examples.

#### 🔨 Bitswap now supports greater configurability

This release adds an [`Internal` section](https://github.com/ipfs/go-ipfs/blob/master/docs/config.md#internal) to the configuration file that is designed to help advanced users optimize their setups without needing a custom binary. The `Internal` section is not guaranteed to be the same from release to release and may not be covered by migrations. If you use the `Internal` section you should be making sure to check the config documentation between releases for any changes.

#### 🐚 Programmatic shell completions command

`ipfs commands completion bash` will generate a bash completion script for go-ipfs commands

#### 📜 Profile collection command

Performance profiles can now be collected using `ipfs diag profile`. If you need to do some debugging or have an issue to submit the collected profiles are very useful to have around.

#### 🍎 Mac OS notarized binaries

The go-ipfs and related migration binaries (for both Intel and Apple Sillicon) are now signed and notarized to make Mac OS installation easier.

#### 👨‍👩‍👦 Improved MDNS

There is a completed implementation of the revised libp2p MDNS spec. This should result in better MDNS discovery and better local/offline operation as a result.

#### 🚗 CAR import statistics

`dag import` command now  supports `--stats` option which will include the number of imported blocks and their total size in the output.

#### 🕸 Peering command

This release adds `swarm peering`  command for easy management of  the peering subsystem. Peer in the peering subsystem is maintained to be connected at all times, and gets reconnected on disconnect with a back-off.

See `ipfs swarm peering --help` for more details.

### Changelog

- github.com/ipfs/go-ipfs:
  - fuse: load unixfs adls as their dagpb substrates
  - enable the legacy mDNS implementation
  - test: add dag get --ouput-codec test
  - change ipfs dag get flag name from format to output-codec
  - test: check behavior of loading UnixFS sharded directories with missing shards
  - remove dag put option shortcuts
  - change names of ipfs dag put flags to make changes clearer
  - feat: dag import --stats (#8237) ([ipfs/go-ipfs#8237](https://github.com/ipfs/go-ipfs/pull/8237))
  - feat: ipfs-webui v2.13.0 (#8430) ([ipfs/go-ipfs#8430](https://github.com/ipfs/go-ipfs/pull/8430))
  - feat(cli): add daemon option --agent-version-suffix (#8419) ([ipfs/go-ipfs#8419](https://github.com/ipfs/go-ipfs/pull/8419))
  - feat: multibase transcode command (#8403) ([ipfs/go-ipfs#8403](https://github.com/ipfs/go-ipfs/pull/8403))
  - fix: take the lock while listing peers
  - feature: 'ipfs swarm peering' command (#8147) ([ipfs/go-ipfs#8147](https://github.com/ipfs/go-ipfs/pull/8147))
  - fix(sharness): add extra check in flush=false in files write
  - chore: update IPFS Desktop testing steps (#8393) ([ipfs/go-ipfs#8393](https://github.com/ipfs/go-ipfs/pull/8393))
  - add more buttons; remove some sections covered in the docs; general cleanup
  - Cosmetic fixups in examples (#8325) ([ipfs/go-ipfs#8325](https://github.com/ipfs/go-ipfs/pull/8325))
  - perf: use performance-enhancing FUSE mount options
  - ci: publish Docker images for bifrost-* branches
  - chore: add comments to peerlog plugin about being unsupported
  - test: add unit tests for peerlog config parsing
  - ci: preload peerlog plugin, disable by default
  - fix(mkreleaselog): specify the parent commit when diffing
  - update go-libp2p to v0.15.0-rc.1 ([ipfs/go-ipfs#8354](https://github.com/ipfs/go-ipfs/pull/8354))
  - feat: add 'ipfs multibase' commands (#8180) ([ipfs/go-ipfs#8180](https://github.com/ipfs/go-ipfs/pull/8180))
  - support bitswap configurability (#8268) ([ipfs/go-ipfs#8268](https://github.com/ipfs/go-ipfs/pull/8268))
  - IPLD Prime In IPFS: Target Merge Branch (#7976) ([ipfs/go-ipfs#7976](https://github.com/ipfs/go-ipfs/pull/7976))
  - ci: upgrade to Go 1.16.7 on CI ([ipfs/go-ipfs#8324](https://github.com/ipfs/go-ipfs/pull/8324))
  - Add flag to create parent directories in files cp command ([ipfs/go-ipfs#8340](https://github.com/ipfs/go-ipfs/pull/8340))
  - fix: avoid out of bounds error when rendering short hashes ([ipfs/go-ipfs#8318](https://github.com/ipfs/go-ipfs/pull/8318))
  - fix: remove some deprecated calls ([ipfs/go-ipfs#8296](https://github.com/ipfs/go-ipfs/pull/8296))
  - perf: set an appropriate capacity ([ipfs/go-ipfs#8244](https://github.com/ipfs/go-ipfs/pull/8244))
  - Fix: Use a pointer type on IpfsNode.Peering ([ipfs/go-ipfs#8331](https://github.com/ipfs/go-ipfs/pull/8331))
  - fix: macos notarized fs-repo-migrations (#8333) ([ipfs/go-ipfs#8333](https://github.com/ipfs/go-ipfs/pull/8333))
  - README.md: Add MacPorts to install section ([ipfs/go-ipfs#8220](https://github.com/ipfs/go-ipfs/pull/8220))
  - feat: register first block metric by default ([ipfs/go-ipfs#8332](https://github.com/ipfs/go-ipfs/pull/8332))
  - Build a go-ipfs:extras docker image ([ipfs/go-ipfs#8142](https://github.com/ipfs/go-ipfs/pull/8142))
  - fix/go-ipfs-as-a-library ([ipfs/go-ipfs#8266](https://github.com/ipfs/go-ipfs/pull/8266))
  - Expose additional migration APIs (#8153) ([ipfs/go-ipfs#8153](https://github.com/ipfs/go-ipfs/pull/8153))
  - point ipfs to pinner that syncs on every pin (#8231) ([ipfs/go-ipfs#8231](https://github.com/ipfs/go-ipfs/pull/8231))
  - docs: chocolatey package name
  - Disambiguate online/offline naming in sharness tests ([ipfs/go-ipfs#8254](https://github.com/ipfs/go-ipfs/pull/8254))
  - Rename DOCKER_HOST to TEST_DOCKER_HOST to avoid conflicts ([ipfs/go-ipfs#8283](https://github.com/ipfs/go-ipfs/pull/8283))
  - feat: add an "ipfs diag profile" command ([ipfs/go-ipfs#8291](https://github.com/ipfs/go-ipfs/pull/8291))
  - Merge branch 'release'
  - feat: improve mkreleaslog ([ipfs/go-ipfs#8290](https://github.com/ipfs/go-ipfs/pull/8290))
  - Add test with expected failure for #3503 ([ipfs/go-ipfs#8280](https://github.com/ipfs/go-ipfs/pull/8280))
  - Create PATCH_RELEASE_TEMPLATE.md
  - fix document error ([ipfs/go-ipfs#8271](https://github.com/ipfs/go-ipfs/pull/8271))
  - feat: webui v2.12.4
  - programmatic shell completions ([ipfs/go-ipfs#8043](https://github.com/ipfs/go-ipfs/pull/8043))
  - test: gateway response for bafkqaaa
  - doc(README): update chat links (and misc fixes) ([ipfs/go-ipfs#8222](https://github.com/ipfs/go-ipfs/pull/8222))
  - link to the actual doc (#8126) ([ipfs/go-ipfs#8126](https://github.com/ipfs/go-ipfs/pull/8126))
  - Improve peer hints for pin remote add (#8143) ([ipfs/go-ipfs#8143](https://github.com/ipfs/go-ipfs/pull/8143))
  - fix(mkreleaselog): support multiple commit authors ([ipfs/go-ipfs#8214](https://github.com/ipfs/go-ipfs/pull/8214))
  - fix(mkreleaselog): handle commit 0 ([ipfs/go-ipfs#8121](https://github.com/ipfs/go-ipfs/pull/8121))
  - bump snap to build with Go 1.16
  - chore: update CHANGELOG
  - chore: switch tar-utils dep to ipfs org
  - feat: print error on bootstrap failure ([ipfs/go-ipfs#8166](https://github.com/ipfs/go-ipfs/pull/8166))
  - fix: typo in migration error
  - refactor: improved humanNumber and humanSI
  - feat: humanized durations in stat provide
  - feat: humanized numbers in stat provide
  - feat: add a text output encoding for the stats provide command
  - fix: webui-2.12.3
  - refactor(pinmfs): log error if pre-existing pin failed (#8056) ([ipfs/go-ipfs#8056](https://github.com/ipfs/go-ipfs/pull/8056))
  - config.md: fix typos/improve wording ([ipfs/go-ipfs#8031](https://github.com/ipfs/go-ipfs/pull/8031))
  - fix(peering_test) : Fix the peering_test to check the connection explicitly added ([ipfs/go-ipfs#8140](https://github.com/ipfs/go-ipfs/pull/8140))
  - build: ignore generated files in changelog ([ipfs/go-ipfs#7712](https://github.com/ipfs/go-ipfs/pull/7712))
  - update version to 0.10.0-dev ([ipfs/go-ipfs#8136](https://github.com/ipfs/go-ipfs/pull/8136))
- github.com/ipfs/go-bitswap (v0.3.4 -> v0.4.0):
  - More stats, knobs and tunings (#514) ([ipfs/go-bitswap#514](https://github.com/ipfs/go-bitswap/pull/514))
  - fix: fix a map access race condition in the want index ([ipfs/go-bitswap#523](https://github.com/ipfs/go-bitswap/pull/523))
  - fix: make blockstore cancel test less timing dependent ([ipfs/go-bitswap#507](https://github.com/ipfs/go-bitswap/pull/507))
  - fix(decision): fix a datarace on disconnect ([ipfs/go-bitswap#508](https://github.com/ipfs/go-bitswap/pull/508))
  - optimize the lookup which peers are waiting for a given block ([ipfs/go-bitswap#486](https://github.com/ipfs/go-bitswap/pull/486))
  - fix: hold the task worker lock when starting task workers ([ipfs/go-bitswap#504](https://github.com/ipfs/go-bitswap/pull/504))
  - fix: Nil dereference while using SetSendDontHaves ([ipfs/go-bitswap#488](https://github.com/ipfs/go-bitswap/pull/488))
  - Fix flaky tests in message queue ([ipfs/go-bitswap#497](https://github.com/ipfs/go-bitswap/pull/497))
  - Fix flaky DontHaveTimeoutManger tests ([ipfs/go-bitswap#495](https://github.com/ipfs/go-bitswap/pull/495))
  - sync: update CI config files ([ipfs/go-bitswap#485](https://github.com/ipfs/go-bitswap/pull/485))
- github.com/ipfs/go-blockservice (v0.1.4 -> v0.1.7):
  - update go-bitswap to v0.3.4 ([ipfs/go-blockservice#78](https://github.com/ipfs/go-blockservice/pull/78))
  - fix staticcheck ([ipfs/go-blockservice#75](https://github.com/ipfs/go-blockservice/pull/75))
  - fix: handle missing session exchange in Session ([ipfs/go-blockservice#73](https://github.com/ipfs/go-blockservice/pull/73))
- github.com/ipfs/go-datastore (v0.4.5 -> v0.4.6):
  - sync: update CI config files ([ipfs/go-datastore#175](https://github.com/ipfs/go-datastore/pull/175))
  - speedup tests ([ipfs/go-datastore#177](https://github.com/ipfs/go-datastore/pull/177))
  - test: reduce element count when the race detector is enabled ([ipfs/go-datastore#176](https://github.com/ipfs/go-datastore/pull/176))
  - fix staticcheck ([ipfs/go-datastore#173](https://github.com/ipfs/go-datastore/pull/173))
  - remove Makefile ([ipfs/go-datastore#172](https://github.com/ipfs/go-datastore/pull/172))
- github.com/ipfs/go-ds-badger (v0.2.6 -> v0.2.7):
  - Log start and end of GC rounds ([ipfs/go-ds-badger#115](https://github.com/ipfs/go-ds-badger/pull/115))
- github.com/ipfs/go-fs-lock (v0.0.6 -> v0.0.7):
  - chore: update log ([ipfs/go-fs-lock#24](https://github.com/ipfs/go-fs-lock/pull/24))
  - sync: update CI config files ([ipfs/go-fs-lock#21](https://github.com/ipfs/go-fs-lock/pull/21))
  - fix TestLockedByOthers on Windows ([ipfs/go-fs-lock#19](https://github.com/ipfs/go-fs-lock/pull/19))
- github.com/ipfs/go-ipfs-config (v0.14.0 -> v0.16.0):
  - feat: add Internal and Internal.Bitswap config options
  - feat: add an OptionalInteger type
  - fix: make sure the Priority type properly implements the JSON marshal/unmarshal interfaces
  - fix: remove deprecated calls ([ipfs/go-ipfs-config#138](https://github.com/ipfs/go-ipfs-config/pull/138))
  - sync: update CI config files ([ipfs/go-ipfs-config#132](https://github.com/ipfs/go-ipfs-config/pull/132))
  - remove period, fix staticcheck ([ipfs/go-ipfs-config#131](https://github.com/ipfs/go-ipfs-config/pull/131))
- github.com/ipfs/go-ipfs-pinner (v0.1.1 -> v0.1.2):
  - Fix/minimize rebuild (#15) ([ipfs/go-ipfs-pinner#15](https://github.com/ipfs/go-ipfs-pinner/pull/15))
  - Define ErrNotPinned alongside the Pinner interface
  - fix staticcheck ([ipfs/go-ipfs-pinner#11](https://github.com/ipfs/go-ipfs-pinner/pull/11))
  - fix: remove the rest of the pb backed pinner ([ipfs/go-ipfs-pinner#9](https://github.com/ipfs/go-ipfs-pinner/pull/9))
  - Remove old ipldpinner that has been replaced by dspinner ([ipfs/go-ipfs-pinner#7](https://github.com/ipfs/go-ipfs-pinner/pull/7))
  - optimize CheckIfPinned ([ipfs/go-ipfs-pinner#6](https://github.com/ipfs/go-ipfs-pinner/pull/6))
- github.com/ipfs/go-ipfs-provider (v0.5.1 -> v0.6.1):
  - Update to IPLD Prime (#32) ([ipfs/go-ipfs-provider#32](https://github.com/ipfs/go-ipfs-provider/pull/32))
- github.com/ipfs/go-ipld-git (v0.0.4 -> v0.1.1):
  - return ErrUnexpectedEOF when Decode input is too short
  - Update go-ipld-git to a go-ipld-prime codec (#46) ([ipfs/go-ipld-git#46](https://github.com/ipfs/go-ipld-git/pull/46))
  - fix staticcheck ([ipfs/go-ipld-git#49](https://github.com/ipfs/go-ipld-git/pull/49))
  - change WriteTo to the standard signature ([ipfs/go-ipld-git#47](https://github.com/ipfs/go-ipld-git/pull/47))
  - don't copy mutexes ([ipfs/go-ipld-git#48](https://github.com/ipfs/go-ipld-git/pull/48))
- github.com/ipfs/go-ipns (v0.1.0 -> v0.1.2):
  - fix: remove deprecated calls ([ipfs/go-ipns#30](https://github.com/ipfs/go-ipns/pull/30))
  - remove Makefile ([ipfs/go-ipns#27](https://github.com/ipfs/go-ipns/pull/27))
- github.com/ipfs/go-log/v2 (v2.1.3 -> v2.3.0):
  - Stop defaulting to color output on non-TTY ([ipfs/go-log#116](https://github.com/ipfs/go-log/pull/116))
  - feat: add ability to use custom zap core ([ipfs/go-log#114](https://github.com/ipfs/go-log/pull/114))
  - fix staticcheck ([ipfs/go-log#112](https://github.com/ipfs/go-log/pull/112))
  - test: fix flaky label test ([ipfs/go-log#111](https://github.com/ipfs/go-log/pull/111))
  - per-subsystem log-levels ([ipfs/go-log#109](https://github.com/ipfs/go-log/pull/109))
  - fix: don't panic on invalid log labels ([ipfs/go-log#110](https://github.com/ipfs/go-log/pull/110))
- github.com/ipfs/go-merkledag (v0.3.2 -> v0.4.0):
  - Use IPLD-prime: target merge branch ([ipfs/go-merkledag#67](https://github.com/ipfs/go-merkledag/pull/67))
  - sync: update CI config files ([ipfs/go-merkledag#70](https://github.com/ipfs/go-merkledag/pull/70))
  - staticcheck ([ipfs/go-merkledag#69](https://github.com/ipfs/go-merkledag/pull/69))
  - Fix bug in dagutils MergeDiffs. (#59) ([ipfs/go-merkledag#59](https://github.com/ipfs/go-merkledag/pull/59))
  - chore: add tests to verify allowable data layouts ([ipfs/go-merkledag#58](https://github.com/ipfs/go-merkledag/pull/58))
- github.com/ipfs/go-namesys (v0.3.0 -> v0.3.1):
  - fix: remove deprecated call to pk.Bytes ([ipfs/go-namesys#19](https://github.com/ipfs/go-namesys/pull/19))
- github.com/ipfs/go-path (v0.0.9 -> v0.1.2):
  - fix: give one minute timeouts to function calls instead of block retrievals ([ipfs/go-path#44](https://github.com/ipfs/go-path/pull/44))
  - IPLD Prime In IPFS: Target Merge Branch (#36) ([ipfs/go-path#36](https://github.com/ipfs/go-path/pull/36))
  - remove Makefile ([ipfs/go-path#40](https://github.com/ipfs/go-path/pull/40))
  - sync: update CI config files ([ipfs/go-path#39](https://github.com/ipfs/go-path/pull/39))
- github.com/ipfs/go-peertaskqueue (v0.2.0 -> v0.4.0):
  - add stats
  - Have a configurable maximum active work per peer ([ipfs/go-peertaskqueue#10](https://github.com/ipfs/go-peertaskqueue/pull/10))
  - sync: update CI config files ([ipfs/go-peertaskqueue#13](https://github.com/ipfs/go-peertaskqueue/pull/13))
  - fix staticcheck ([ipfs/go-peertaskqueue#12](https://github.com/ipfs/go-peertaskqueue/pull/12))
  - fix go vet ([ipfs/go-peertaskqueue#11](https://github.com/ipfs/go-peertaskqueue/pull/11))
- github.com/ipfs/go-unixfsnode (null -> v1.1.3):
  - make UnixFSHAMTShard implement the ADL interface (#11) ([ipfs/go-unixfsnode#11](https://github.com/ipfs/go-unixfsnode/pull/11))
- github.com/ipfs/interface-go-ipfs-core (v0.4.0 -> v0.5.1):
  - IPLD In IPFS: Target Merge Branch (#67) ([ipfs/interface-go-ipfs-core#67](https://github.com/ipfs/interface-go-ipfs-core/pull/67))
  - fix staticcheck ([ipfs/interface-go-ipfs-core#72](https://github.com/ipfs/interface-go-ipfs-core/pull/72))
  - remove Makefile ([ipfs/interface-go-ipfs-core#70](https://github.com/ipfs/interface-go-ipfs-core/pull/70))
- github.com/ipld/go-codec-dagpb (v1.2.0 -> v1.3.0):
  - fix staticcheck warnings ([ipld/go-codec-dagpb#29](https://github.com/ipld/go-codec-dagpb/pull/29))
  - update go-ipld-prime, use go:generate
  - allow decoding PBNode fields in any order
  - expose APIs without Reader/Writer overhead
  - preallocate 1KiB on the stack for marshals
  - encode directly with a []byte
  - decode directly with a []byte
  - remove unnecessary xerrors dep
- github.com/ipld/go-ipld-prime (v0.9.1-0.20210324083106-dc342a9917db -> v0.12.2):
  - Printer feature ([ipld/go-ipld-prime#238](https://github.com/ipld/go-ipld-prime/pull/238))
  - schema: keep TypeSystem names ordered
  - schema/dmt: redesign with bindnode and add Compile
  - codec: make cbor and json codecs use ErrUnexpectedEOF
  - bindnode: fix for stringjoin struct emission when first field is the empty string ([ipld/go-ipld-prime#239](https://github.com/ipld/go-ipld-prime/pull/239))
  - schema: typekind names are not capitalized.
  - Bindnode fixes continued ([ipld/go-ipld-prime#233](https://github.com/ipld/go-ipld-prime/pull/233))
  - helper methods for encoding and decoding ([ipld/go-ipld-prime#232](https://github.com/ipld/go-ipld-prime/pull/232))
  - mark v0.12.0
  - Major refactor: extract datamodel package.
    ([ipld/go-ipld-prime#228](https://github.com/ipld/go-ipld-prime/pull/228))
  - Fix ExploreRecursive stopAt condition, add tests, add error return to Explore (#229) ([ipld/go-ipld-prime#229](https://github.com/ipld/go-ipld-prime/pull/229))
  - selector: add tests which are driven by language-agnostic spec fixtures. ([ipld/go-ipld-prime#231](https://github.com/ipld/go-ipld-prime/pull/231))
  - selector: Improve docs for implementors. (#227) ([ipld/go-ipld-prime#227](https://github.com/ipld/go-ipld-prime/pull/227))
  - Bindnode fixes of opportunity ([ipld/go-ipld-prime#226](https://github.com/ipld/go-ipld-prime/pull/226))
  - node/bindnode: redesign the shape of unions in Go ([ipld/go-ipld-prime#223](https://github.com/ipld/go-ipld-prime/pull/223))
  - summary of the v0.11.0 changelog should holler even more about how cool bindnode is.
  - mark v0.11.0
  - node/bindnode: mark as experimental in its godoc.
  - codecs: more docs, a terminology guide, consistency in options. ([ipld/go-ipld-prime#221](https://github.com/ipld/go-ipld-prime/pull/221))
  - Changelog backfill.
  - selectors: docs enhancements, new construction helpers. ([ipld/go-ipld-prime#199](https://github.com/ipld/go-ipld-prime/pull/199))
  - Changelog backfill.
  - Allow parsing of single Null tokens from refmt
  - Add link conditions for 'stop-at' expression in ExploreRecursive selector ([ipld/go-ipld-prime#214](https://github.com/ipld/go-ipld-prime/pull/214))
  - Remove base64 padding for dag-json bytes as per spec
  - node/bindnode: temporarily skip Links schema test
  - test: add test for traversal of typed node links
  - fix: typed links LinkTargetNodePrototype should return ReferencedType
  - Make `go vet` happy
  - Add MapSortMode to MarshalOptions
  - Add {Unm,M}arshalOptions for explicit mode switching for cbor vs dagcbor
  - Sort map entries marshalling dag-cbor
  - node/bindnode: first pass at inferring IPLD schemas
  - Add {Unm,M}arshalOptions for explicit mode switching for json vs dagjson
  - Make tests pass with sorted dag-json output
  - Sort map entries marshalling dag-json
  - Simplify refmt usage
  - Fix failing test using dagjson encoding
  - Fix some failing tests using dagjson
  - Remove pretty-printing
  - Update readme linking to specs and meta repo.
  - Fix example names so they render on go.pkg.dev.
  - fluent/quip: remove in favor of qp
  - node/basic: add Chooser
  - schema: add TypedPrototype
  - node/bindnode: rethink and better document APIs
  - node/tests: cover yet more interface methods
  - node/tests: cover more error cases for scalar kinds
  - node/tests: add more extensive scalar kind tests
  - node/bindnode: start running all schema tests
  - mark v0.10.0
  - More changelog grooming.
  - Changelog grooming.
  - node/tests: put most of the schema test cases here
  - Add more explicit discussion of indicies to ListIterator.
  - node/bindnode: start of a reflect-based Node implementation
  - add DeepEqual and start using it in tests
  - Add enumerate methods to the multicodec registries. ([ipld/go-ipld-prime#176](https://github.com/ipld/go-ipld-prime/pull/176))
  - Make a multicodec.Registry type available. ([ipld/go-ipld-prime#172](https://github.com/ipld/go-ipld-prime/pull/172))
  - fluent/qp: don't panic on string panics
  - Allow emitting & parsing of bytes per dagjson codec spec ([ipld/go-ipld-prime#166](https://github.com/ipld/go-ipld-prime/pull/166))
  - Package docs for dag-cbor.
  - Update package docs.
  - schema/gen/go: apply gofmt automatically ([ipld/go-ipld-prime#163](https://github.com/ipld/go-ipld-prime/pull/163))
  - schema/gen/go: fix remaining vet warnings on generated code
  - schema/gen/go: batch file writes via a bytes.Buffer ([ipld/go-ipld-prime#161](https://github.com/ipld/go-ipld-prime/pull/161))
  - schema/gen/go: avoid Maybe pointers for small types
  - fix readme formatting typo
  - feat(linksystem): add reification to LinkSystem ([ipld/go-ipld-prime#158](https://github.com/ipld/go-ipld-prime/pull/158))
- github.com/libp2p/go-addr-util (v0.0.2 -> v0.1.0):
  - stop using the deprecated go-multiaddr-net package ([libp2p/go-addr-util#34](https://github.com/libp2p/go-addr-util/pull/34))
  - Remove `IsFDCostlyTransport` ([libp2p/go-addr-util#31](https://github.com/libp2p/go-addr-util/pull/31))
- github.com/libp2p/go-libp2p (v0.14.3 -> v0.15.0):
  - chore: update go-tcp-transport to v0.2.8
  - implement the new mDNS spec, move the old mDNS implementation (#1161) ([libp2p/go-libp2p#1161](https://github.com/libp2p/go-libp2p/pull/1161))
  - remove deprecated basichost.New constructor ([libp2p/go-libp2p#1156](https://github.com/libp2p/go-libp2p/pull/1156))
  - Make BasicHost.evtLocalAddrsUpdated event emitter stateful. ([libp2p/go-libp2p#1147](https://github.com/libp2p/go-libp2p/pull/1147))
  - fix: deflake multipro echo test ([libp2p/go-libp2p#1149](https://github.com/libp2p/go-libp2p/pull/1149))
  - fix(basic_host): stream not closed when context done ([libp2p/go-libp2p#1148](https://github.com/libp2p/go-libp2p/pull/1148))
  - chore: update deps ([libp2p/go-libp2p#1141](https://github.com/libp2p/go-libp2p/pull/1141))
  - remove secio from examples ([libp2p/go-libp2p#1143](https://github.com/libp2p/go-libp2p/pull/1143))
  - remove deprecated Filter option ([libp2p/go-libp2p#1132](https://github.com/libp2p/go-libp2p/pull/1132))
  - fix: remove deprecated call ([libp2p/go-libp2p#1136](https://github.com/libp2p/go-libp2p/pull/1136))
  - test: fix flaky example test ([libp2p/go-libp2p#1135](https://github.com/libp2p/go-libp2p/pull/1135))
  - remove deprecated identify.ClientVersion ([libp2p/go-libp2p#1133](https://github.com/libp2p/go-libp2p/pull/1133))
  - remove Go version requirement and note about Go modules from README ([libp2p/go-libp2p#1126](https://github.com/libp2p/go-libp2p/pull/1126))
  - Error assignment fix ([libp2p/go-libp2p#1124](https://github.com/libp2p/go-libp2p/pull/1124))
  - perf/basic_host: Don't handle address change if we hasn't anyone ([libp2p/go-libp2p#1115](https://github.com/libp2p/go-libp2p/pull/1115))
- github.com/libp2p/go-libp2p-core (v0.8.5 -> v0.9.0):
  - feat: remove unused metrics (#208) ([libp2p/go-libp2p-core#208](https://github.com/libp2p/go-libp2p-core/pull/208))
  - feat: keep addresses for longer (#207) ([libp2p/go-libp2p-core#207](https://github.com/libp2p/go-libp2p-core/pull/207))
  - remove deprecated key stretching struct / function (#203) ([libp2p/go-libp2p-core#203](https://github.com/libp2p/go-libp2p-core/pull/203))
  - remove deprecated Bytes method from the Key interface (#204) ([libp2p/go-libp2p-core#204](https://github.com/libp2p/go-libp2p-core/pull/204))
  - remove deprecated functions in the peer package (#205) ([libp2p/go-libp2p-core#205](https://github.com/libp2p/go-libp2p-core/pull/205))
  - remove deprecated constructor for the insecure transport (#206) ([libp2p/go-libp2p-core#206](https://github.com/libp2p/go-libp2p-core/pull/206))
  - feat: add helper functions for working with addr infos (#202) ([libp2p/go-libp2p-core#202](https://github.com/libp2p/go-libp2p-core/pull/202))
  - fix: make timestamps strictly increasing (#201) ([libp2p/go-libp2p-core#201](https://github.com/libp2p/go-libp2p-core/pull/201))
  - ci: use github-actions for compatibility testing (#200) ([libp2p/go-libp2p-core#200](https://github.com/libp2p/go-libp2p-core/pull/200))
  - sync: update CI config files (#189) ([libp2p/go-libp2p-core#189](https://github.com/libp2p/go-libp2p-core/pull/189))
  - remove minimum Go version from README (#199) ([libp2p/go-libp2p-core#199](https://github.com/libp2p/go-libp2p-core/pull/199))
  - remove flaky tests (#194) ([libp2p/go-libp2p-core#194](https://github.com/libp2p/go-libp2p-core/pull/194))
  - reduce default timeouts to 15s (#192) ([libp2p/go-libp2p-core#192](https://github.com/libp2p/go-libp2p-core/pull/192))
  - fix benchmark of key verifications (#190) ([libp2p/go-libp2p-core#190](https://github.com/libp2p/go-libp2p-core/pull/190))
  - fix staticcheck errors (#191) ([libp2p/go-libp2p-core#191](https://github.com/libp2p/go-libp2p-core/pull/191))
  - doc: document Close on Transport (#188) ([libp2p/go-libp2p-core#188](https://github.com/libp2p/go-libp2p-core/pull/188))
  - add a helper function to go directly from a string to an AddrInfo (#184) ([libp2p/go-libp2p-core#184](https://github.com/libp2p/go-libp2p-core/pull/184))
- github.com/libp2p/go-libp2p-http (v0.2.0 -> v0.2.1):
  - remove Makefile ([libp2p/go-libp2p-http#70](https://github.com/libp2p/go-libp2p-http/pull/70))
  - fix staticcheck ([libp2p/go-libp2p-http#67](https://github.com/libp2p/go-libp2p-http/pull/67))
  - Revert "increase buffer size"
  - Increase read buffer size to reduce poll system calls ([libp2p/go-libp2p-http#66](https://github.com/libp2p/go-libp2p-http/pull/66))
- github.com/libp2p/go-libp2p-kad-dht (v0.12.2 -> v0.13.1):
  - Extract validation from ProtocolMessenger ([libp2p/go-libp2p-kad-dht#741](https://github.com/libp2p/go-libp2p-kad-dht/pull/741))
  - remove codecov.yml ([libp2p/go-libp2p-kad-dht#742](https://github.com/libp2p/go-libp2p-kad-dht/pull/742))
  - integrate some basic opentelemetry tracing ([libp2p/go-libp2p-kad-dht#734](https://github.com/libp2p/go-libp2p-kad-dht/pull/734))
  - feat: delete GetValues ([libp2p/go-libp2p-kad-dht#728](https://github.com/libp2p/go-libp2p-kad-dht/pull/728))
  - chore: skip flaky test when race detector is enabled ([libp2p/go-libp2p-kad-dht#731](https://github.com/libp2p/go-libp2p-kad-dht/pull/731))
  - Dont count connection times in usefulness ([libp2p/go-libp2p-kad-dht#660](https://github.com/libp2p/go-libp2p-kad-dht/pull/660))
  - Routing table refresh should NOT block ([libp2p/go-libp2p-kad-dht#705](https://github.com/libp2p/go-libp2p-kad-dht/pull/705))
  - update bootstrapPeers to be func() []peer.AddrInfo (#716) ([libp2p/go-libp2p-kad-dht#716](https://github.com/libp2p/go-libp2p-kad-dht/pull/716))
- github.com/libp2p/go-libp2p-noise (v0.2.0 -> v0.2.2):
  - remove note about go modules in README ([libp2p/go-libp2p-noise#100](https://github.com/libp2p/go-libp2p-noise/pull/100))
  - fix: remove deprecated call to pk.Bytes ([libp2p/go-libp2p-noise#99](https://github.com/libp2p/go-libp2p-noise/pull/99))
- github.com/libp2p/go-libp2p-peerstore (v0.2.7 -> v0.2.8):
  - Fix perfomance issue in updating addr book ([libp2p/go-libp2p-peerstore#141](https://github.com/libp2p/go-libp2p-peerstore/pull/141))
  - Fix test flakes ([libp2p/go-libp2p-peerstore#164](https://github.com/libp2p/go-libp2p-peerstore/pull/164))
  - Only remove records during GC ([libp2p/go-libp2p-peerstore#135](https://github.com/libp2p/go-libp2p-peerstore/pull/135))
  - sync: update CI config files ([libp2p/go-libp2p-peerstore#160](https://github.com/libp2p/go-libp2p-peerstore/pull/160))
  - fix: fix some race conditions in the ds address book ([libp2p/go-libp2p-peerstore#161](https://github.com/libp2p/go-libp2p-peerstore/pull/161))
  - address lints and test failures ([libp2p/go-libp2p-peerstore#159](https://github.com/libp2p/go-libp2p-peerstore/pull/159))
  - stop using the deprecated go-multiaddr-net package ([libp2p/go-libp2p-peerstore#158](https://github.com/libp2p/go-libp2p-peerstore/pull/158))
- github.com/libp2p/go-libp2p-pubsub (v0.4.2 -> v0.5.4):
  - make slowness a warning, with a user configurable threshold
  - reduce log spam from empty heartbeat messages
  - fix: code review
  - add support for custom protocol matching function
  - fix: remove deprecated Bytes call (#436) ([libp2p/go-libp2p-pubsub#436](https://github.com/libp2p/go-libp2p-pubsub/pull/436))
  - cleanup: fix vet and staticcheck failures (#435) ([libp2p/go-libp2p-pubsub#435](https://github.com/libp2p/go-libp2p-pubsub/pull/435))
  - Revert noisy newline changes
  - fix: avoid panic when peer is blacklisted after connection
  - release priority locks early when handling batches
  - don't respawn writer if we fail to open a stream; declare it a peer error
  - batch process dead peer notifications
  - use a priority lock instead of a semaphore
  - do the notification in a goroutine
  - emit new peer notification without holding the semaphore
  - use a semaphore for new peer notifications so that we don't block the event loop
  - don't accumulate pending goroutines from new connections
  - rename RawTracer's DroppedInSubscribe into UndeliverableMessage
  - add a new RawTracer event to track messages dropped in Subscribe
  - add an option to configure the Subscription output queue length
  - fix some comments
  - expose more events for RawTracer
  - Make close concurrent safe
  - Fix close of closed channel
  - Update README to point to correct example directory (#424) ([libp2p/go-libp2p-pubsub#424](https://github.com/libp2p/go-libp2p-pubsub/pull/424))
  - fix: remove deprecated and never used topic descriptors (#423) ([libp2p/go-libp2p-pubsub#423](https://github.com/libp2p/go-libp2p-pubsub/pull/423))
  - Refactor Gossipsub Parameters To Make Them More Configurable (#421) ([libp2p/go-libp2p-pubsub#421](https://github.com/libp2p/go-libp2p-pubsub/pull/421))
  - add tests for gs features and custom protocols
  - add support for custom gossipsub protocols and feature tests
  - RIP travis, Long Live CircleCI (#414) ([libp2p/go-libp2p-pubsub#414](https://github.com/libp2p/go-libp2p-pubsub/pull/414))
  - Ignore transient connections (#412) ([libp2p/go-libp2p-pubsub#412](https://github.com/libp2p/go-libp2p-pubsub/pull/412))
  - demote log spam to debug
  - fix bug
  - add last amount of validation
  - add threshold validation
  - strengthen validation
  - rename checkSignature to checkSigningPolicy
  - rename validation.Publish to PushLocal
  - fix TestValidate, add TestValidate2
  - skip flaky test until we can fix it
  - implement synchronous validation for locally published messages
  - expose internalTracer as RawTracer
  - export rejection named string constants
  - more intelligent handling of ip whitelist check
  - remove obsolete explicit IP whitelisting in favor of subnets
  - add subnet whitelisting for IPColocation
- github.com/libp2p/go-libp2p-quic-transport (v0.11.2 -> v0.12.0):
  - sync: update CI config files (#228) ([libp2p/go-libp2p-quic-transport#228](https://github.com/libp2p/go-libp2p-quic-transport/pull/228))
  - fix closing of streams in example ([libp2p/go-libp2p-quic-transport#221](https://github.com/libp2p/go-libp2p-quic-transport/pull/221))
  - close all UDP connections when the reuse is closed ([libp2p/go-libp2p-quic-transport#216](https://github.com/libp2p/go-libp2p-quic-transport/pull/216))
  - fix staticcheck ([libp2p/go-libp2p-quic-transport#217](https://github.com/libp2p/go-libp2p-quic-transport/pull/217))
  - sync: update CI config files (#214) ([libp2p/go-libp2p-quic-transport#214](https://github.com/libp2p/go-libp2p-quic-transport/pull/214))
  - implement a Transport.Close that waits for the reuse's GC to finish ([libp2p/go-libp2p-quic-transport#211](https://github.com/libp2p/go-libp2p-quic-transport/pull/211))
  - don't compare peer IDs when hole punching ([libp2p/go-libp2p-quic-transport#210](https://github.com/libp2p/go-libp2p-quic-transport/pull/210))
  - add hole punching support (#194) ([libp2p/go-libp2p-quic-transport#194](https://github.com/libp2p/go-libp2p-quic-transport/pull/194))
- github.com/libp2p/go-libp2p-swarm (v0.5.0 -> v0.5.3):
  - sync: update CI config files ([libp2p/go-libp2p-swarm#263](https://github.com/libp2p/go-libp2p-swarm/pull/263))
  - remove incorrect call to InterceptAddrDial ([libp2p/go-libp2p-swarm#260](https://github.com/libp2p/go-libp2p-swarm/pull/260))
  - speed up the TestFDLimitUnderflow test ([libp2p/go-libp2p-swarm#262](https://github.com/libp2p/go-libp2p-swarm/pull/262))
  - sync: update CI config files (#248) ([libp2p/go-libp2p-swarm#248](https://github.com/libp2p/go-libp2p-swarm/pull/248))
- github.com/libp2p/go-libp2p-testing (v0.4.0 -> v0.4.2):
  - fix deadlock in the transport's serve function ([libp2p/go-libp2p-testing#35](https://github.com/libp2p/go-libp2p-testing/pull/35))
  - fix: cleanup transport suite ([libp2p/go-libp2p-testing#34](https://github.com/libp2p/go-libp2p-testing/pull/34))
  - Address `go vet` and `saticcheck` issues ([libp2p/go-libp2p-testing#33](https://github.com/libp2p/go-libp2p-testing/pull/33))
  - Defer closing stream for reading ([libp2p/go-libp2p-testing#32](https://github.com/libp2p/go-libp2p-testing/pull/32))
- github.com/libp2p/go-libp2p-tls (v0.1.3 -> v0.2.0):
  - fix: don't fail the handshake when the libp2p extension is critical ([libp2p/go-libp2p-tls#88](https://github.com/libp2p/go-libp2p-tls/pull/88))
  - fix deprecated call to key.Bytes ([libp2p/go-libp2p-tls#86](https://github.com/libp2p/go-libp2p-tls/pull/86))
  - fix usage of deprecated peer.IDB58Decode ([libp2p/go-libp2p-tls#77](https://github.com/libp2p/go-libp2p-tls/pull/77))
  - remove setting of the TLS 1.3 GODEBUG flag ([libp2p/go-libp2p-tls#68](https://github.com/libp2p/go-libp2p-tls/pull/68))
  - improve the error message returned when peer verification fails ([libp2p/go-libp2p-tls#57](https://github.com/libp2p/go-libp2p-tls/pull/57))
  - update to Go 1.14 ([libp2p/go-libp2p-tls#54](https://github.com/libp2p/go-libp2p-tls/pull/54))
  - Update deps and fix tests ([libp2p/go-libp2p-tls#43](https://github.com/libp2p/go-libp2p-tls/pull/43))
- github.com/libp2p/go-libp2p-transport-upgrader (v0.4.2 -> v0.4.6):
  - chore: update deps ([libp2p/go-libp2p-transport-upgrader#78](https://github.com/libp2p/go-libp2p-transport-upgrader/pull/78))
  - fix typo in error message ([libp2p/go-libp2p-transport-upgrader#77](https://github.com/libp2p/go-libp2p-transport-upgrader/pull/77))
  - fix staticcheck ([libp2p/go-libp2p-transport-upgrader#74](https://github.com/libp2p/go-libp2p-transport-upgrader/pull/74))
  - don't listen on all interfaces in tests ([libp2p/go-libp2p-transport-upgrader#73](https://github.com/libp2p/go-libp2p-transport-upgrader/pull/73))
  - stop using the deprecated go-multiaddr-net ([libp2p/go-libp2p-transport-upgrader#72](https://github.com/libp2p/go-libp2p-transport-upgrader/pull/72))
- github.com/libp2p/go-libp2p-xor (v0.0.0-20200501025846-71e284145d58 -> v0.0.0-20210714161855-5c005aca55db):
  - Add immutable remove operation ([libp2p/go-libp2p-xor#14](https://github.com/libp2p/go-libp2p-xor/pull/14))
  - fix go vet and staticcheck ([libp2p/go-libp2p-xor#11](https://github.com/libp2p/go-libp2p-xor/pull/11))
- github.com/libp2p/go-reuseport-transport (v0.0.4 -> v0.0.5):
  - remove note about Go modules in README ([libp2p/go-reuseport-transport#32](https://github.com/libp2p/go-reuseport-transport/pull/32))
  - stop using the deprecated go-multiaddr-net package ([libp2p/go-reuseport-transport#30](https://github.com/libp2p/go-reuseport-transport/pull/30))
- github.com/libp2p/go-socket-activation (v0.0.2 -> v0.1.0):
  - chore: stop using the deprecated go-multiaddr-net package ([libp2p/go-socket-activation#16](https://github.com/libp2p/go-socket-activation/pull/16))
  - fix staticcheck ([libp2p/go-socket-activation#13](https://github.com/libp2p/go-socket-activation/pull/13))
- github.com/libp2p/go-tcp-transport (v0.2.4 -> v0.2.8):
  - disable metrics collection on Windows ([libp2p/go-tcp-transport#93](https://github.com/libp2p/go-tcp-transport/pull/93))
  - sync: update CI config files (#90) ([libp2p/go-tcp-transport#90](https://github.com/libp2p/go-tcp-transport/pull/90))
  - chore: update go-libp2p-transport-upgrader and go-reuseport-transport ([libp2p/go-tcp-transport#84](https://github.com/libp2p/go-tcp-transport/pull/84))
- github.com/libp2p/go-ws-transport (v0.4.0 -> v0.5.0):
  - chore: update go-libp2p-transport-upgrader and go-libp2p-core ([libp2p/go-ws-transport#103](https://github.com/libp2p/go-ws-transport/pull/103))
  - remove deprecated type ([libp2p/go-ws-transport#102](https://github.com/libp2p/go-ws-transport/pull/102))
  - sync: update CI config files ([libp2p/go-ws-transport#101](https://github.com/libp2p/go-ws-transport/pull/101))
  - chore: various cleanups required to get vet/staticcheck/test to pass ([libp2p/go-ws-transport#100](https://github.com/libp2p/go-ws-transport/pull/100))
- github.com/lucas-clemente/quic-go (v0.21.2 -> v0.23.0):
  - update to Go 1.17.x ([lucas-clemente/quic-go#3258](https://github.com/lucas-clemente/quic-go/pull/3258))
  - quicvarint: export Min and Max (#3253) ([lucas-clemente/quic-go#3253](https://github.com/lucas-clemente/quic-go/pull/3253))
  - drop support for Go 1.15 ([lucas-clemente/quic-go#3247](https://github.com/lucas-clemente/quic-go/pull/3247))
  - quicvarint: add Reader and Writer interfaces (#3233) ([lucas-clemente/quic-go#3233](https://github.com/lucas-clemente/quic-go/pull/3233))
  - fix race when stream.Read and CancelRead are called concurrently ([lucas-clemente/quic-go#3241](https://github.com/lucas-clemente/quic-go/pull/3241))
  - also count coalesced 0-RTT packets in the integration tests ([lucas-clemente/quic-go#3251](https://github.com/lucas-clemente/quic-go/pull/3251))
  - remove draft versions 32 and 34 from README (#3244) ([lucas-clemente/quic-go#3244](https://github.com/lucas-clemente/quic-go/pull/3244))
  - update Changelog ([lucas-clemente/quic-go#3245](https://github.com/lucas-clemente/quic-go/pull/3245))
  - optimize hasOutstandingCryptoPackets in sentPacketHandler ([lucas-clemente/quic-go#3230](https://github.com/lucas-clemente/quic-go/pull/3230))
  - permit underlying conn to implement batch interface directly ([lucas-clemente/quic-go#3237](https://github.com/lucas-clemente/quic-go/pull/3237))
  - cancel the PTO timer when all Handshake packets are acknowledged ([lucas-clemente/quic-go#3231](https://github.com/lucas-clemente/quic-go/pull/3231))
  - fix flaky INVALID_TOKEN server test ([lucas-clemente/quic-go#3223](https://github.com/lucas-clemente/quic-go/pull/3223))
  - drop support for QUIC draft version 32 and 34 ([lucas-clemente/quic-go#3217](https://github.com/lucas-clemente/quic-go/pull/3217))
  - fix flaky 0-RTT integration test ([lucas-clemente/quic-go#3224](https://github.com/lucas-clemente/quic-go/pull/3224))
  - use batched reads ([lucas-clemente/quic-go#3142](https://github.com/lucas-clemente/quic-go/pull/3142))
  - add a config option to disable sending of Version Negotiation packets ([lucas-clemente/quic-go#3216](https://github.com/lucas-clemente/quic-go/pull/3216))
  - remove the RetireBugBackwardsCompatibilityMode ([lucas-clemente/quic-go#3213](https://github.com/lucas-clemente/quic-go/pull/3213))
  - remove outdated ackhandler test case ([lucas-clemente/quic-go#3212](https://github.com/lucas-clemente/quic-go/pull/3212))
  - remove unused StripGreasedVersions function ([lucas-clemente/quic-go#3214](https://github.com/lucas-clemente/quic-go/pull/3214))
  - fix incorrect usage of errors.Is ([lucas-clemente/quic-go#3215](https://github.com/lucas-clemente/quic-go/pull/3215))
  - return error on SendMessage when session is closed ([lucas-clemente/quic-go#3218](https://github.com/lucas-clemente/quic-go/pull/3218))
  - remove a redundant error check ([lucas-clemente/quic-go#3210](https://github.com/lucas-clemente/quic-go/pull/3210))
  - update golangci-lint to v1.41.1 ([lucas-clemente/quic-go#3205](https://github.com/lucas-clemente/quic-go/pull/3205))
  - Update doc for dialer in http3.RoundTripper ([lucas-clemente/quic-go#3208](https://github.com/lucas-clemente/quic-go/pull/3208))
- github.com/multiformats/go-multiaddr (v0.3.3 -> v0.4.0):
  - remove forced dependency on deprecated go-maddr-filter ([multiformats/go-multiaddr#162](https://github.com/multiformats/go-multiaddr/pull/162))
  - remove deprecated SwapToP2pMultiaddrs ([multiformats/go-multiaddr#161](https://github.com/multiformats/go-multiaddr/pull/161))
  - remove Makefile ([multiformats/go-multiaddr#163](https://github.com/multiformats/go-multiaddr/pull/163))
  - remove deprecated filter functions ([multiformats/go-multiaddr#157](https://github.com/multiformats/go-multiaddr/pull/157))
  - remove deprecated NetCodec ([multiformats/go-multiaddr#159](https://github.com/multiformats/go-multiaddr/pull/159))
  - add Noise ([multiformats/go-multiaddr#156](https://github.com/multiformats/go-multiaddr/pull/156))
  - Add TLS protocol ([multiformats/go-multiaddr#153](https://github.com/multiformats/go-multiaddr/pull/153))
- github.com/multiformats/go-multicodec (v0.2.0 -> v0.3.0):
  - Export reserved range constants (#53) ([multiformats/go-multicodec#53](https://github.com/multiformats/go-multicodec/pull/53))
  - make Code.Set accept valid code numbers
  - replace Of with Code.Set, implementing flag.Value
  - add multiformats/multicodec as a git submodule
  - update the generator with the "status" CSV column
  - Run `go generate` to generate the latest codecs
  - Add lookup for multicodec code by string name ([multiformats/go-multicodec#40](https://github.com/multiformats/go-multicodec/pull/40))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Daniel Martí | 42 | +8549/-6587 | 170 |
| Eric Myhre | 55 | +5883/-6715 | 395 |
| Marten Seemann | 100 | +1814/-2028 | 275 |
| Steven Allen | 80 | +1573/-1998 | 127 |
| hannahhoward | 18 | +1721/-671 | 53 |
| Will | 2 | +1114/-1217 | 18 |
| Andrew Gillis | 2 | +1220/-720 | 14 |
| gammazero | 3 | +43/-1856 | 10 |
| Masih H. Derkani | 3 | +960/-896 | 8 |
| Adin Schmahmann | 25 | +1458/-313 | 44 |
| vyzo | 27 | +986/-353 | 60 |
| Will Scott | 6 | +852/-424 | 16 |
| Rod Vagg | 19 | +983/-255 | 66 |
| Petar Maymounkov | 6 | +463/-179 | 22 |
| web3-bot | 10 | +211/-195 | 24 |
| adlrocha | 1 | +330/-75 | 15 |
| RubenKelevra | 2 | +128/-210 | 2 |
| Ian Davis | 3 | +200/-109 | 17 |
| Cory Schwartz | 3 | +231/-33 | 7 |
| Keenan Nemetz | 1 | +184/-71 | 2 |
| Randy Reddig | 2 | +187/-53 | 8 |
| Takashi Matsuda | 3 | +201/-2 | 7 |
| guseggert | 4 | +161/-20 | 9 |
| Lucas Molas | 5 | +114/-47 | 27 |
| nisdas | 4 | +115/-45 | 7 |
| Michael Muré | 6 | +107/-33 | 24 |
| Richard Ramos | 2 | +113/-9 | 3 |
| Marcin Rataj | 12 | +88/-24 | 13 |
| Ondrej Prazak | 2 | +104/-6 | 4 |
| Michal Dobaczewski | 2 | +77/-28 | 3 |
| Jorropo | 3 | +9/-75 | 4 |
| Andey Robins | 1 | +70/-3 | 3 |
| Gus Eggert | 10 | +34/-31 | 12 |
| noot | 1 | +54/-9 | 5 |
| Maxim Merzhanov | 1 | +29/-24 | 1 |
| Adrian Lanzafame | 1 | +30/-13 | 2 |
| Bogdan Stirbat | 1 | +22/-16 | 2 |
| Shad Sterling | 1 | +28/-3 | 1 |
| Jesse Bouwman | 5 | +30/-0 | 5 |
| Pavel Karpy | 1 | +19/-7 | 2 |
| lasiar | 5 | +14/-10 | 5 |
| Dennis Trautwein | 1 | +20/-4 | 2 |
| Louis Thibault | 1 | +22/-1 | 2 |
| whyrusleeping | 2 | +21/-1 | 2 |
| aarshkshah1992 | 3 | +12/-8 | 3 |
| Peter Rabbitson | 2 | +20/-0 | 2 |
| bt90 | 2 | +17/-2 | 2 |
| Dominic Della Valle | 1 | +13/-1 | 2 |
| Audrius Butkevicius | 1 | +12/-1 | 1 |
| Brian Strauch | 1 | +9/-3 | 1 |
| Aarsh Shah | 2 | +1/-11 | 2 |
| Whyrusleeping | 1 | +11/-0 | 1 |
| Max | 1 | +7/-3 | 1 |
| vallder | 1 | +3/-5 | 1 |
| Michael Burns | 3 | +2/-6 | 3 |
| Lasse Johnsen | 1 | +4/-4 | 2 |
| snyh | 1 | +5/-2 | 1 |
| Hector Sanjuan | 2 | +3/-2 | 2 |
| 市川恭佑 (ebi) | 1 | +1/-3 | 1 |
| godcong | 2 | +2/-1 | 2 |
| Mathis Engelbart | 1 | +1/-2 | 1 |
| folbrich | 1 | +1/-1 | 1 |
| Med Mouine | 1 | +1/-1 | 1 |


## v0.9.1 2021-07-20

This is a small bug fix release resolving the following issues:
1. A regression where the empty CID bafkqaaa could not resolve on gateways [#8230](https://github.com/ipfs/go-ipfs/issues/8230)
2. A panic on OpenBSD [#8211](https://github.com/ipfs/go-ipfs/issues/8211)
3. High CPU usage with QUIC [#8256](https://github.com/ipfs/go-ipfs/issues/8256)
4. High memory usage with TCP [#8219](https://github.com/ipfs/go-ipfs/issues/8219)
5. Some pubsub issues ([libp2p/go-libp2p-pubsub#427](https://github.com/libp2p/go-libp2p-pubsub/pull/427), [libp2p/go-libp2p-pubsub#430](https://github.com/libp2p/go-libp2p-pubsub/pull/430))
6. Updated WebUI to [v2.12.4](https://github.com/ipfs/ipfs-webui/releases/tag/v2.12.4)
7. Fixed the snap deployment [#8212](https://github.com/ipfs/go-ipfs/pull/8212)

### Changelog

- github.com/ipfs/go-ipfs:
  - chore: update deps
  - feat: webui v2.12.4
  - test: gateway response for bafkqaaa
  - fix: downgrade mimetype dependency
  - update go-libp2p to v0.14.3
  - bump snap to build with Go 1.16
- github.com/libp2p/go-libp2p (v0.14.2 -> v0.14.3):
  - update go-tcp-transport to v0.2.3 and go-multiaddr to v0.3.3 ([libp2p/go-libp2p#1121](https://github.com/libp2p/go-libp2p/pull/1121))
- github.com/libp2p/go-libp2p-pubsub (v0.4.1 -> v0.4.2):
  - release priority locks early when handling batches
  - don't respawn writer if we fail to open a stream; declare it a peer error
  - batch process dead peer notifications
  - use a priority lock instead of a semaphore
  - do the notification in a goroutine
  - emit new peer notification without holding the semaphore
  - use a semaphore for new peer notifications so that we don't block the event loop
  - don't accumulate pending goroutines from new connections
  - Make close concurrent safe
  - Fix close of closed channel
- github.com/libp2p/go-libp2p-quic-transport (v0.11.1 -> v0.11.2):
  - update quic-go to v0.21.2
- github.com/libp2p/go-tcp-transport (v0.2.2 -> v0.2.4):
  - collect metrics in a separate go routine ([libp2p/go-tcp-transport#82](https://github.com/libp2p/go-tcp-transport/pull/82))
  - fix: avoid logging "invalid argument" errors when setting keepalive ([libp2p/go-tcp-transport#83](https://github.com/libp2p/go-tcp-transport/pull/83))
  - Skip SetKeepAlivePeriod call on OpenBSD ([libp2p/go-tcp-transport#80](https://github.com/libp2p/go-tcp-transport/pull/80))
  - sync: update CI config files (#79) ([libp2p/go-tcp-transport#79](https://github.com/libp2p/go-tcp-transport/pull/79))
- github.com/lucas-clemente/quic-go (v0.21.1 -> v0.21.2):
  - update qtls to include the crypto/tls fix of Go 1.16.6 / 1.15.14
  - cancel the PTO timer when all Handshake packets are acknowledged
  - update to Go 1.17rc1
  - update Ginkgo to v1.16.4 and Gomega to v1.13.0 ([lucas-clemente/quic-go#3139](https://github.com/lucas-clemente/quic-go/pull/3139))
- github.com/multiformats/go-multiaddr (v0.3.2 -> v0.3.3):
  - guard against nil {Local,Remote}Addr() return values ([multiformats/go-multiaddr#155](https://github.com/multiformats/go-multiaddr/pull/155))
  - sync: update CI config files (#154) ([multiformats/go-multiaddr#154](https://github.com/multiformats/go-multiaddr/pull/154))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| vyzo | 8 | +205/-141 | 12 |
| Marten Seemann | 7 | +127/-74 | 11 |
| gammazero | 2 | +43/-5 | 3 |
| Steven Allen | 1 | +13/-2 | 1 |
| Adin Schmahmann | 3 | +13/-2 | 3 |
| Marcin Rataj | 2 | +9/-1 | 2 |
| Aaron Bieber | 1 | +6/-2 | 1 |

## v0.9.0 2021-06-22

We're happy to announce go-ipfs 0.9.0. This release makes go-ipfs even more configurable with some fun experiments to boot. We're also deprecating or removing some uncommonly used features to make it easier for users to discover the easy ways to use go-ipfs safely and efficiently.

As usual, this release includes important fixes, some of which may be critical for security. Unless the fix addresses a bug being exploited in the wild, the fix will _not_ be called out in the release notes. Please make sure to update ASAP. See our [release process](https://github.com/ipfs/go-ipfs/tree/master/docs/releases.md#security-fix-policy) for details.

### 🔦 Highlights

#### 📦 Exporting of DAGs via Gateways

Gateways now support downloading arbitrary IPLD graphs via the `/api/v0/dag/export` endpoint. This endpoint works in the same way as the `ipfs dag export` command.

One major thing this enables is ability to verify data downloaded from public gateways. If you go to `https://somegateway.example.net/ipfs/bafyexample` you are using the old school HTTP transport, and trusting that the gateway is being well behaved. However, if you download the graph as a [DAG archive](https://github.com/ipld/specs/blob/master/block-layer/content-addressable-archives.md) then it is possible to verify that the data you downloaded does in fact match `bafyexample`.

Additionally, it was previously quite painful to download things other than UnixFS (files + directories) using gateways. It is now possible to download arbitrary IPLD graphs from gateways, making them useful as a general-purpose alternative to p2p transports.

This opens exciting opportunities in areas like thin clients, mobile browsers and IoT devices, which now can delegate IPFS resolution to any public gateway, and have ability to verify that the data received matches the requested hash.

#### ☁ Custom DNS Resolvers

Resolution of DNS records for DNSLink and DNSAddrs means that names  are sent in cleartext between the operating system and the DNS server provided by an ISP. In the past, the only way to customize DNS resolution in IPFS stack was to set up own DNS proxy server.

There is now the ability to [customize DNS resolution](https://github.com/ipfs/go-ipfs/blob/master/docs/config.md#dns) and override the default resolver from the OS with [DNS over HTTPS](https://en.wikipedia.org/wiki/DNS_over_HTTPS) (DoH) one. We made it really flexible: override can be applied globally, or per specific [TLD](https://en.wikipedia.org/wiki/Top-level_domain)/[FQDN](https://en.wikipedia.org/wiki/Fully_qualified_domain_name). Examples can be found in the [documentation](https://github.com/ipfs/go-ipfs/blob/master/docs/config.md#dns).

#### 👪 Support for non-ICANN DNSLink names

Building off of the support for custom DNS resolvers it is now possible to create DNSLink names not handled by ICANN and choose how that domain name will be resolved. An example of this is how ENS is supported, despite `.eth` not being an ICANN TLD you can point `.eth` to any ENS resolver you want (including a local one).

While go-ipfs may have some DoH defaults for a few popular non-ICANN DNSLink names (e.g. ENS), you are free to use any protocol for a naming system and as long as it exposes a DNSLink record via a DNS endpoint you can make it work.

#### 🖥️ Updated to the latest WebUI

Our web interface now includes experimental support for pinning services, and various updates to _Files_ and _Peers_ screens.

Remote pinning services added via the `ipfs pin remote service add` command are already detected, one can also add one from _Settings_ screen, and it will appear in _Set pinning_ interface on the _Files_ screen.

Data presented on the _Peers_ screen can now be copied by simply clicking on a specific cell, and a list of open streams gives better insight into how a local node interacts with a specific peer.

See release notes for [ipfs-webui v2.12](https://github.com/ipfs/ipfs-webui/releases/tag/v2.12.0) for screenshots and more details.

#### 🔑 IPNS keys can now be exported via the CLI without stopping the daemon

`ipfs key export` no longer requires interrupting `ipfs daemon` ✨

#### 🕸 Experimental DHT Client and Provider System

An area of go-ipfs that has been historically tricky is how go-ipfs finds who has the data they are looking for. While the IPFS Public DHT is only one of the ways go-ipfs can find data it tends to be an important one. While since go-ipfs v0.5.0 the time to find content in the network has dropped significantly the time to put/get IPNS records or for a node to advertise the content it has still has much room for improvement.

We have been doing some experimenting and have an alternative DHT client that essentially trades off some resources and in return is much more performant. We have also included with the experimental DHT client a bulk provider system that takes advantage of the new client to more efficiently do many advertisements at a time

This work is quite new and still under development, however, the results so far have been promising especially for users with lots of data who have otherwise been having difficulty advertising their data into the IPFS Public DHT

As described in the experimental features [documentation](https://github.com/ipfs/go-ipfs/blob/master/docs/experimental-features.md#accelerated-dht-client) the experimental client can be enabled using the command below (or modifying the config file).

```
ipfs config --json Experimental.AcceleratedDHTClient true
```

A few things to take note of when `AcceleratedDHTClient` is enabled:
- go-ipfs will likely use more resources then previously
- DHT queries will not be usable (i.e. finding which peers have some data, finding where a particular peer is, etc.) for the first 5-10 minutes of operation depending on your network conditions
- There is an `ipfs stats provide` command that will help you track your provide/reprovide usage, if you are providing lots of data you may want to consider how to reduce the amount you are providing (e.g. [Reprovider Strategies](https://github.com/ipfs/go-ipfs/blob/master/docs/config.md#reproviderstrategy) and/or [Strategic Providing](https://github.com/ipfs/go-ipfs/blob/master/docs/experimental-features.md#strategic-providing))

See the [documentation](https://github.com/ipfs/go-ipfs/blob/master/docs/experimental-features.md#accelerated-dht-client) for more details.

#### 🚶‍♀️ Migrations

##### Migrations are now individually packaged

While previously the go-ipfs [repo migration](https://github.com/ipfs/fs-repo-migrations) binary was monolithic and contained all migrations from previous go-ipfs versions the binaries are now packaged individually. However, the fs-repo-migrations binary is still there to help those who manually upgrade their repos to download all the individual migrations.

This means faster download times for upgrades, a much easier time building migrations for those who make use of custom plugins, and an easier time developing new migrations going forward.

##### Configurable migration downloads enable downloading over IPFS

Previously the migration downloader built into go-ipfs downloaded the migrations from [dist.ipfs.io](https://dist.ipfs.io). While users could use tools like [ipfs-update](https://github.com/ipfs/ipfs-update) to download the migrations over IPFS or manually download the migrations (over IPFS or otherwise) themselves, this is now automated and configurable. Users can choose to download the migrations over IPFS or from any specified IPFS Gateway.

The configurable migration options are described in the config file [documentation](https://github.com/ipfs/go-ipfs/blob/master/docs/config.md#migration), although most users should not need to change the default settings.

The main benefit here is that users behind restrictive firewalls, or in offline/private deployments, won't have to run migrations manually, which is especially important for desktop use cases where go-ipfs is running inside of [IPFS Desktop](https://github.com/ipfs-shipyard/ipfs-desktop#readme) and [Brave](https://brave.com/ipfs-support/).

#### 🍎 Published builds for Apple M1 hardware

Go now supports building for Darwin ARM64, and we are now publishing those builds

#### 👋 Deprecations and Feature Removals

##### The `ipfs object` commands are now deprecated

In the last couple years most of the Object API's commands have become fulfillable using alternative APIs.

The utility of Object API's is limited to data in UnixFS-v1  (`dag-pb`) format. If you are still using it, it is highly recommended that you switch to the DAG `ipfs dag` (supports modern data types like `dag-cbor`) or Files `ipfs files` (more intuitive for working with `dag-pb`) APIs.

While the Object API and commands are still usable they are now marked as deprecated and hidden from users on the command line to discourage further use. We also updated their `--help` text to point at the modern replacements.


##### `X-Ipfs-Gateway-Prefix` is now deprecated

IPFS community moved towards dedicated Origins (DNSLink and [subdomain gateways](https://docs.ipfs.io/how-to/address-ipfs-on-web/#subdomain-gateway)) which are much easier to isolate and reason about.

Setting up `Gateway.PathPrefixes` and `X-Ipfs-Gateway-Prefix` is no longer necessary and support [will be removed in near future](https://github.com/ipfs/go-ipfs/issues/7702).

##### Proquints support removed

A little known feature that was not well used or documented and was more well known for the error message `Error: not a valid proquint string` users received when trying to download invalid IPNS or DNSLink names (e.g. `https://dweb.link/ipns/badname`). We have removed support for proquints as they were out of place and largely unused, however proquints are [valid multibases](https://github.com/multiformats/multibase/pull/78) so if there is renewed interest in them there is a way forward.

##### SECIO support removed

SECIO was deprecated and turned off by default given the prevalence of TLS and Noise support, SECIO support is now removed entirely.

### Changelog

- github.com/ipfs/go-ipfs:
  - chore: switch tar-utils dep to ipfs org
  - chore: update CHANGELOG
  - refactor: warning when bootstrap disabled by user
  - feat: print error on bootstrap failure
  - fix: typo in migration error
  - Release v0.9.0-rc2
  - refactor: improved humanNumber and humanSI
  - feat: humanized durations in stat provide
  - feat: humanized numbers in stat provide
  - feat: add a text output encoding for the stats provide command
  - fix: webui-2.12.3
  - refactor(pinmfs): log error if pre-existing pin failed (#8056) ([ipfs/go-ipfs#8056](https://github.com/ipfs/go-ipfs/pull/8056))
  - Release v0.9.0-rc1
  - Added support for an experimental DHT client and provider system via the Experiments.AcceleratedDHTClient config option ([ipfs/go-ipfs#8061](https://github.com/ipfs/go-ipfs/pull/8061))
  - feat: support DNSLink on non-ICANN DNS names ([ipfs/go-ipfs#8071](https://github.com/ipfs/go-ipfs/pull/8071))
  - update go-tcp-transport to v0.2.2 ([ipfs/go-ipfs#8129](https://github.com/ipfs/go-ipfs/pull/8129))
  - update quic-go to v0.21.0-rc.1 ([ipfs/go-ipfs#8125](https://github.com/ipfs/go-ipfs/pull/8125))
  - chore: bump minimum go version to 1.15 ([ipfs/go-ipfs#7944](https://github.com/ipfs/go-ipfs/pull/7944))
  - chore: update deps ([ipfs/go-ipfs#8128](https://github.com/ipfs/go-ipfs/pull/8128))
  - feat: allow key export in online mode ([ipfs/go-ipfs#8113](https://github.com/ipfs/go-ipfs/pull/8113))
  - Feat/migration ipfs download (#8064) ([ipfs/go-ipfs#8064](https://github.com/ipfs/go-ipfs/pull/8064))
  - feat: support custom DoH resolvers ([ipfs/go-ipfs#8068](https://github.com/ipfs/go-ipfs/pull/8068))
  - update go-libp2p to v0.14.0 ([ipfs/go-ipfs#8122](https://github.com/ipfs/go-ipfs/pull/8122))
  - feat(gw): expose /api/v0/dag/export on gateway port ([ipfs/go-ipfs#8111](https://github.com/ipfs/go-ipfs/pull/8111))
  - chore: update webui to 2.12.2 ([ipfs/go-ipfs#8097](https://github.com/ipfs/go-ipfs/pull/8097))
  - docs: deprecate object commands ([ipfs/go-ipfs#8098](https://github.com/ipfs/go-ipfs/pull/8098))
  - fix: omit empty pins slice when reporting pin progress ([ipfs/go-ipfs#8023](https://github.com/ipfs/go-ipfs/pull/8023))
  - Fix typo in comment ([ipfs/go-ipfs#8087](https://github.com/ipfs/go-ipfs/pull/8087))
  - fix: set systemd startup timeout to infinity ([ipfs/go-ipfs#8040](https://github.com/ipfs/go-ipfs/pull/8040))
  - fix(mkreleaselog): partially handle v2 modules ([ipfs/go-ipfs#8073](https://github.com/ipfs/go-ipfs/pull/8073))
  - Update migration sharness tests for new migrations (#8053) ([ipfs/go-ipfs#8053](https://github.com/ipfs/go-ipfs/pull/8053))
  - fix: make migrations log output to stdout ([ipfs/go-ipfs#8054](https://github.com/ipfs/go-ipfs/pull/8054))
  - fix(gw): remove hardcoded hostnames ([ipfs/go-ipfs#8069](https://github.com/ipfs/go-ipfs/pull/8069))
  - Fix transposed words in docs/config.md ([ipfs/go-ipfs#8051](https://github.com/ipfs/go-ipfs/pull/8051))
  - fix: update root help ([ipfs/go-ipfs#8052](https://github.com/ipfs/go-ipfs/pull/8052))
  - chore: dont docker tag rc as latest ([ipfs/go-ipfs#8055](https://github.com/ipfs/go-ipfs/pull/8055))
  - Add info to "pin rm" help about how to tell if pin is indirect ([ipfs/go-ipfs#8044](https://github.com/ipfs/go-ipfs/pull/8044))
  - build(deps): bump contrib.go.opencensus.io/exporter/prometheus from 0.2.0 to 0.3.0 ([ipfs/go-ipfs#8020](https://github.com/ipfs/go-ipfs/pull/8020))
  - fix(gw): remove use of Clear-Site-Data in subdomain router ([ipfs/go-ipfs#7890](https://github.com/ipfs/go-ipfs/pull/7890))
  -  ([ipfs/go-ipfs#7857](https://github.com/ipfs/go-ipfs/pull/7857))
  - docs: clarification of the Strategic Providing functionality ([ipfs/go-ipfs#8035](https://github.com/ipfs/go-ipfs/pull/8035))
  - docs: cosmetic fixes of help text ([ipfs/go-ipfs#8010](https://github.com/ipfs/go-ipfs/pull/8010))
  - chore: deprecate Gateway.PathPrefixes ([ipfs/go-ipfs#7994](https://github.com/ipfs/go-ipfs/pull/7994))
  - Fix text contrast for dark mode ([ipfs/go-ipfs#8027](https://github.com/ipfs/go-ipfs/pull/8027))
  - Do not fetch recursive pins from pinner unnecessarily ([ipfs/go-ipfs#7883](https://github.com/ipfs/go-ipfs/pull/7883))
  - test(sharness): verify the list of exported metrics ([ipfs/go-ipfs#7987](https://github.com/ipfs/go-ipfs/pull/7987))
  - fix: return an error if repo verify is canceled ([ipfs/go-ipfs#7973](https://github.com/ipfs/go-ipfs/pull/7973))
  - doc: document security fix policy ([ipfs/go-ipfs#7991](https://github.com/ipfs/go-ipfs/pull/7991))
  - feat(gw): /ipfs/ipfs/{cid} → /ipfs/{cid} ([ipfs/go-ipfs#7930](https://github.com/ipfs/go-ipfs/pull/7930))
  - Fix: innacuracies in MFS command documentation. ([ipfs/go-ipfs#8001](https://github.com/ipfs/go-ipfs/pull/8001))
  - Feat: Re-import InitializeKeyspace code from go-namesys ([ipfs/go-ipfs#7984](https://github.com/ipfs/go-ipfs/pull/7984))
  - revert registration of metrics against unexposed prom registry ([ipfs/go-ipfs#7986](https://github.com/ipfs/go-ipfs/pull/7986))
  - Extract the namesys and the keystore submodules ([ipfs/go-ipfs#7925](https://github.com/ipfs/go-ipfs/pull/7925))
  - split core/commands/dag into individual files for different subcommands ([ipfs/go-ipfs#7970](https://github.com/ipfs/go-ipfs/pull/7970))
  - test(sharness): pass correct timeout format to go-timeout ([ipfs/go-ipfs#7971](https://github.com/ipfs/go-ipfs/pull/7971))
  - fix race condition when logging requests ([ipfs/go-ipfs#7953](https://github.com/ipfs/go-ipfs/pull/7953))
  - fix some sharness-in-CI issues ([ipfs/go-ipfs#7946](https://github.com/ipfs/go-ipfs/pull/7946))
  - chore: update deps ([ipfs/go-ipfs#7941](https://github.com/ipfs/go-ipfs/pull/7941))
  - fix: correctly return pin ls errors ([ipfs/go-ipfs#7942](https://github.com/ipfs/go-ipfs/pull/7942))
  - feat: remove secio support ([ipfs/go-ipfs#7943](https://github.com/ipfs/go-ipfs/pull/7943))
  - Set supported platforms by go-version ([ipfs/go-ipfs#7927](https://github.com/ipfs/go-ipfs/pull/7927))
  - docs: tips on debugging Policies.MFS (#7929) ([ipfs/go-ipfs#7929](https://github.com/ipfs/go-ipfs/pull/7929))
  - docs: fix DNSLink gw recipe ([ipfs/go-ipfs#7932](https://github.com/ipfs/go-ipfs/pull/7932))
  - Merge branch 'release'
  - docs: RepinInterval
  - style: docs/config.md
  - style: improved MFS PinName example
  - docs: Pinning.RemoteServices.Policies
  - peering: add logs before many-second waits ([ipfs/go-ipfs#7904](https://github.com/ipfs/go-ipfs/pull/7904))
  - all: gofmt -s ([ipfs/go-ipfs#7900](https://github.com/ipfs/go-ipfs/pull/7900))
- github.com/ipfs/go-bitswap (v0.3.3 -> v0.3.4):
  - remove Makefile ([ipfs/go-bitswap#483](https://github.com/ipfs/go-bitswap/pull/483))
  - test: deflake engine test ([ipfs/go-bitswap#480](https://github.com/ipfs/go-bitswap/pull/480))
  - test: deflake large-message test ([ipfs/go-bitswap#479](https://github.com/ipfs/go-bitswap/pull/479))
  - fix: fix alignment of stats struct in virtual network ([ipfs/go-bitswap#478](https://github.com/ipfs/go-bitswap/pull/478))
  - fix(network): impl: add timeout in newStreamToPeer call ([ipfs/go-bitswap#477](https://github.com/ipfs/go-bitswap/pull/477))
  - fix staticcheck ([ipfs/go-bitswap#474](https://github.com/ipfs/go-bitswap/pull/474))
  - ignore transient connections ([ipfs/go-bitswap#470](https://github.com/ipfs/go-bitswap/pull/470))
  - fix a startup race by creating the blockstoremanager process on init ([ipfs/go-bitswap#465](https://github.com/ipfs/go-bitswap/pull/465))
- github.com/ipfs/go-block-format (v0.0.2 -> v0.0.3):
  - doc: add a lead maintainer ([ipfs/go-block-format#16](https://github.com/ipfs/go-block-format/pull/16))
- github.com/ipfs/go-graphsync (v0.6.0 -> v0.8.0):
  - docs(CHANGELOG): update for v0.8.0
  - Update for LinkSystem (#161) ([ipfs/go-graphsync#161](https://github.com/ipfs/go-graphsync/pull/161))
  - Round out diagnostic parameters (#157) ([ipfs/go-graphsync#157](https://github.com/ipfs/go-graphsync/pull/157))
  - map response codes to names (#148) ([ipfs/go-graphsync#148](https://github.com/ipfs/go-graphsync/pull/148))
  - Discard http output (#156) ([ipfs/go-graphsync#156](https://github.com/ipfs/go-graphsync/pull/156))
  - Add debug logging (#121) ([ipfs/go-graphsync#121](https://github.com/ipfs/go-graphsync/pull/121))
  - Add optional HTTP comparison (#153) ([ipfs/go-graphsync#153](https://github.com/ipfs/go-graphsync/pull/153))
  - docs(architecture): update architecture docs (#154) ([ipfs/go-graphsync#154](https://github.com/ipfs/go-graphsync/pull/154))
  - release v0.7.0 ([ipfs/go-graphsync#152](https://github.com/ipfs/go-graphsync/pull/152))
  - chore: update deps (#151) ([ipfs/go-graphsync#151](https://github.com/ipfs/go-graphsync/pull/151))
  - Automatically record heap profiles in testplans (#147) ([ipfs/go-graphsync#147](https://github.com/ipfs/go-graphsync/pull/147))
  - feat(deps): update go-ipld-prime v0.7.0 (#145) ([ipfs/go-graphsync#145](https://github.com/ipfs/go-graphsync/pull/145))
  - Release/v0.6.0 ([ipfs/go-graphsync#144](https://github.com/ipfs/go-graphsync/pull/144))
- github.com/ipfs/go-ipfs-blockstore (v0.1.4 -> v0.1.6):
  - use bloom filter in GetSize
- github.com/ipfs/go-ipfs-config (v0.12.0 -> v0.14.0):
  - Added Experiments.AcceleratedDHTClient option ([ipfs/go-ipfs-config#125](https://github.com/ipfs/go-ipfs-config/pull/125))
  - Add config for downloading repo migrations ([ipfs/go-ipfs-config#128](https://github.com/ipfs/go-ipfs-config/pull/128))
  - remove duplicate entries in defaultServerFilters ([ipfs/go-ipfs-config#121](https://github.com/ipfs/go-ipfs-config/pull/121))
  - add custom DNS Resolver configuration ([ipfs/go-ipfs-config#126](https://github.com/ipfs/go-ipfs-config/pull/126))
- github.com/ipfs/go-ipfs-provider (v0.4.3 -> v0.5.1):
  - Fix batched providing of empty keys ([ipfs/go-ipfs-provider#37](https://github.com/ipfs/go-ipfs-provider/pull/37))
  - Bulk Provide/Reproviding System (#34) ([ipfs/go-ipfs-provider#34](https://github.com/ipfs/go-ipfs-provider/pull/34))
  - chore: update the Usage part of readme ([ipfs/go-ipfs-provider#33](https://github.com/ipfs/go-ipfs-provider/pull/33))
  - Retract and revert 1.0.0 ([ipfs/go-ipfs-provider#31](https://github.com/ipfs/go-ipfs-provider/pull/31))
  - replace go-merkledag with go-fetcher ([ipfs/go-ipfs-provider#30](https://github.com/ipfs/go-ipfs-provider/pull/30))
- github.com/ipfs/go-ipld-git (v0.0.3 -> v0.0.4):
  - add license file so it can be found by go-licenses ([ipfs/go-ipld-git#42](https://github.com/ipfs/go-ipld-git/pull/42))
- github.com/ipfs/go-ipns (v0.0.2 -> v0.1.0):
  - Add support for extensible records (and v2 signature)
- github.com/ipfs/go-log (v1.0.4 -> v1.0.5):
  - chore: update v1 deps ([ipfs/go-log#108](https://github.com/ipfs/go-log/pull/108))
- github.com/ipfs/go-log/v2 (v2.1.1 -> v2.1.3):
  - doc(README): use circle-ci badge ([ipfs/go-log#106](https://github.com/ipfs/go-log/pull/106))
  - feat: add ability to specify labels for all loggers ([ipfs/go-log#105](https://github.com/ipfs/go-log/pull/105))
  - Add an option to pass URL to zap ([ipfs/go-log#101](https://github.com/ipfs/go-log/pull/101))
  - enable configuring several log outputs ([ipfs/go-log#98](https://github.com/ipfs/go-log/pull/98))
  - Fix caller not being added ([ipfs/go-log#96](https://github.com/ipfs/go-log/pull/96))
- github.com/ipfs/go-unixfs (v0.2.4 -> v0.2.5):
  - correct file size for raw node ([ipfs/go-unixfs#88](https://github.com/ipfs/go-unixfs/pull/88))
- github.com/ipld/go-car (v0.1.1-0.20201015032735-ff6ccdc46acc -> v0.3.1):
  - chore: make sure we get an error where we expect one
  - chore: refactor header tests to iterate over a struct
  - chore: add header error tests
  - fix: lint errors
  - fix: go mod tidy
  - chore: update go.mod to 1.15
  - fix: ReadHeader return value mismatch
  - Updates for ipld linksystem branch ([ipld/go-car#56](https://github.com/ipld/go-car/pull/56))
  - replace go-ipld-prime-proto with go-codec-dagpb
  - fix staticcheck errors ([ipld/go-car#67](https://github.com/ipld/go-car/pull/67))
  - chore: switch to a single license file ([ipld/go-car#59](https://github.com/ipld/go-car/pull/59))
  - chore: remove LICENSE ([ipld/go-car#58](https://github.com/ipld/go-car/pull/58))
  - chore: relicense ([ipld/go-car#57](https://github.com/ipld/go-car/pull/57))
  - ci: remove travis support ([ipld/go-car#55](https://github.com/ipld/go-car/pull/55))
  - run gofmt -s
  - Allow user defined block hooks when using two step write for selective cars ([ipld/go-car#37](https://github.com/ipld/go-car/pull/37))
  - feat: handle mid-varint EOF case as UnexpectedEOF
  - fix: main NewReader call
- github.com/ipld/go-ipld-prime (v0.5.1-0.20201021195245-109253e8a018 -> v0.9.1-0.20210324083106-dc342a9917db):
  - Add option to tell link system storage is trusted and we can skip hash on read ([ipld/go-ipld-prime#149](https://github.com/ipld/go-ipld-prime/pull/149))
  - implement non-dag cbor codec ([ipld/go-ipld-prime#153](https://github.com/ipld/go-ipld-prime/pull/153))
  - add non-dag json codec ([ipld/go-ipld-prime#152](https://github.com/ipld/go-ipld-prime/pull/152))
  - typo fixes
  - mark v0.9.0
  - Changelog: more backfill :)
  - hackme: about merge strategies.
  - Dropping .gopath and other unmaintained scripts.
  - introduce LinkSystem ([ipld/go-ipld-prime#143](https://github.com/ipld/go-ipld-prime/pull/143))
  - Readme updates.
  - codec/raw: implement the raw codec
  - add an ADL interface type
  - schema/gen/go: cache genned code in os.TempDir
  - fluent/qp: finish writing all data model helpers
  - fluent: add qp, a different spin on quip
  - schema/gen/go: prevent some unkeyed literal vet errors
  - schema/gen/go: remove two common subtest levels
  - use %q in error strings
  - schema/gen/go: please vet a bit more
  - Introduce 'quip' data building helpers. ([ipld/go-ipld-prime#134](https://github.com/ipld/go-ipld-prime/pull/134))
  - gengo: support for unions with stringprefix representation. ([ipld/go-ipld-prime#133](https://github.com/ipld/go-ipld-prime/pull/133))
  - target of opporunity DRY improvement: use more shared templates for structs with stringjoin representations.
  - fix small consistency typo in gen function names.
  - drop old generation mechanisms that were already deprecated.
  - error type cleanup, and helpers.
  - v0.7.0 and changelog update
  - Revert "rename AssignNode to ConvertFrom"
  - Implement traversal.FocusedTransform. ([ipld/go-ipld-prime#130](https://github.com/ipld/go-ipld-prime/pull/130))
  - Update a few more lingering ReprKind references.
  - all: rename schema.Kind to TypeKind, ipld.ReprKind to Kind ([ipld/go-ipld-prime#127](https://github.com/ipld/go-ipld-prime/pull/127))
  - all: rename AssignNode to ConvertFrom
  - all: rewrite interfaces and APIs to support int64
  - mark v0.6.0
  - clean up node/gendemo regeneration ([ipld/go-ipld-prime#123](https://github.com/ipld/go-ipld-prime/pull/123))
  - cleanup: drop orphaned gitignore file.
  - Schema types rebased to use codegen types for the data ([ipld/go-ipld-prime#107](https://github.com/ipld/go-ipld-prime/pull/107))
  - codegen: assembler for struct with map representation validates all non-optional fields are present ([ipld/go-ipld-prime#121](https://github.com/ipld/go-ipld-prime/pull/121))
  - changelog: backfill.
  - fluent: finish out matrix of helper methods, and fix error handling of the non-Must methods.
  - all: fix a lot of "unkeyed literal" vet warnings
  - node/mixins: use simpler filenames
  - node/gendemo: use the new code generator
  - Merge pull request #96 , originally known as ipld/cidlink-only-usable-as-ptr
  - Codec revamp ([ipld/go-ipld-prime#112](https://github.com/ipld/go-ipld-prime/pull/112))
  - Allow overriden types (#116) ([ipld/go-ipld-prime#116](https://github.com/ipld/go-ipld-prime/pull/116))
  - add import to ipld in ipldsch_types.go ([ipld/go-ipld-prime#115](https://github.com/ipld/go-ipld-prime/pull/115))
  - Codegen output rearrange ([ipld/go-ipld-prime#105](https://github.com/ipld/go-ipld-prime/pull/105))
  - Validate struct builder sufficiency ([ipld/go-ipld-prime#111](https://github.com/ipld/go-ipld-prime/pull/111))
  - Fresh take on codec APIs, and some tokenization utilities. ([ipld/go-ipld-prime#101](https://github.com/ipld/go-ipld-prime/pull/101))
  - Add a demo ADL (rot13adl) ([ipld/go-ipld-prime#98](https://github.com/ipld/go-ipld-prime/pull/98))
  - Introduce traversal function that selects links out of a tree. ([ipld/go-ipld-prime#110](https://github.com/ipld/go-ipld-prime/pull/110))
  - Codegen various improvements ([ipld/go-ipld-prime#106](https://github.com/ipld/go-ipld-prime/pull/106))
- github.com/libp2p/go-conn-security-multistream (v0.2.0 -> v0.2.1):
  - Implement support for simultaneous open (#14) ([libp2p/go-conn-security-multistream#14](https://github.com/libp2p/go-conn-security-multistream/pull/14))
- github.com/libp2p/go-libp2p (v0.13.0 -> v0.14.2):
  - Fix race in adding connections to connsByPeer ([libp2p/go-libp2p#1116](https://github.com/libp2p/go-libp2p/pull/1116))
  - speed up the mock tests ([libp2p/go-libp2p#1103](https://github.com/libp2p/go-libp2p/pull/1103))
  - remove slow ObservedAddrManager test that doesn't test anything ([libp2p/go-libp2p#1104](https://github.com/libp2p/go-libp2p/pull/1104))
  - remove Codecov config ([libp2p/go-libp2p#1100](https://github.com/libp2p/go-libp2p/pull/1100))
  - doc: document standard connection manager ([libp2p/go-libp2p#1099](https://github.com/libp2p/go-libp2p/pull/1099))
  - run go mod tidy in the examples ([libp2p/go-libp2p#1098](https://github.com/libp2p/go-libp2p/pull/1098))
  - Cleanup some remaining examples nits ([libp2p/go-libp2p#1097](https://github.com/libp2p/go-libp2p/pull/1097))
  - chore: bring examples back into repository and add tests ([libp2p/go-libp2p#1092](https://github.com/libp2p/go-libp2p/pull/1092))
  - fix(mkreleasenotes): handle first commit ([libp2p/go-libp2p#1095](https://github.com/libp2p/go-libp2p/pull/1095))
  - doc: add a basic release process ([libp2p/go-libp2p#1080](https://github.com/libp2p/go-libp2p/pull/1080))
  - chore: update yamux ([libp2p/go-libp2p#1089](https://github.com/libp2p/go-libp2p/pull/1089))
  - fix: re-expose AutoNAT service on BasicHost ([libp2p/go-libp2p#1088](https://github.com/libp2p/go-libp2p/pull/1088))
  - remove NEWS.md ([libp2p/go-libp2p#1086](https://github.com/libp2p/go-libp2p/pull/1086))
  - test: deflake TestProtoDowngrade ([libp2p/go-libp2p#1084](https://github.com/libp2p/go-libp2p/pull/1084))
  - sync: update CI config files (and fix tests) ([libp2p/go-libp2p#1083](https://github.com/libp2p/go-libp2p/pull/1083))
  - static check fixes ([libp2p/go-libp2p#1076](https://github.com/libp2p/go-libp2p/pull/1076))
  - fix go vet ([libp2p/go-libp2p#1075](https://github.com/libp2p/go-libp2p/pull/1075))
  - option for custom dns resolver ([libp2p/go-libp2p#1073](https://github.com/libp2p/go-libp2p/pull/1073))
  - chore: update deps ([libp2p/go-libp2p#1066](https://github.com/libp2p/go-libp2p/pull/1066))
  - fix autonat race ([libp2p/go-libp2p#1062](https://github.com/libp2p/go-libp2p/pull/1062))
  - use transient connections in identify streams ([libp2p/go-libp2p#1061](https://github.com/libp2p/go-libp2p/pull/1061))
  - Emit event for User's NAT Type i.e. Hard NAT or Easy NAT (#1042) ([libp2p/go-libp2p#1042](https://github.com/libp2p/go-libp2p/pull/1042))
  - Finish and Test the simultaneous connect problem in libp2p peers (#1041) ([libp2p/go-libp2p#1041](https://github.com/libp2p/go-libp2p/pull/1041))
  - Close peerstore and document Host Close (#1037) ([libp2p/go-libp2p#1037](https://github.com/libp2p/go-libp2p/pull/1037))
  - Timeout all Identify stream reads (#1032) ([libp2p/go-libp2p#1032](https://github.com/libp2p/go-libp2p/pull/1032))
- github.com/libp2p/go-libp2p-autonat (v0.4.0 -> v0.4.2):
  - Fix: Stream read timeout ([libp2p/go-libp2p-autonat#99](https://github.com/libp2p/go-libp2p-autonat/pull/99))
  - fix: simplify address replacement ([libp2p/go-libp2p-autonat#102](https://github.com/libp2p/go-libp2p-autonat/pull/102))
  - replace the port number for double NAT mapping ([libp2p/go-libp2p-autonat#101](https://github.com/libp2p/go-libp2p-autonat/pull/101))
- github.com/libp2p/go-libp2p-core (v0.8.0 -> v0.8.5):
  - mind the dot.
  - context option for simultaneous connect
  - Event for user's NAT Device Type: Tell user if the node is behind an Easy or Hard NAT (#173) ([libp2p/go-libp2p-core#173](https://github.com/libp2p/go-libp2p-core/pull/173))
  - address aarshian nitpicks
  - make UseTransient context option take a reason argument, for consistency with other options
  - abstract Conn Stat interface for threading
  - Update network/context.go
  - add ErrTransientConn error
  - add support for transient connections
  - more docs for stream fncs (#183) ([libp2p/go-libp2p-core#183](https://github.com/libp2p/go-libp2p-core/pull/183))
  - refactor: use a helper type to decode AddrInfo from JSON (#178) ([libp2p/go-libp2p-core#178](https://github.com/libp2p/go-libp2p-core/pull/178))
  - fix stream docs (#182) ([libp2p/go-libp2p-core#182](https://github.com/libp2p/go-libp2p-core/pull/182))
  - context to force direct dial (#181) ([libp2p/go-libp2p-core#181](https://github.com/libp2p/go-libp2p-core/pull/181))
  - Secure Muxer Interface (#180) ([libp2p/go-libp2p-core#180](https://github.com/libp2p/go-libp2p-core/pull/180))
- github.com/libp2p/go-libp2p-discovery (v0.5.0 -> v0.5.1):
  - Fix hang in BackoffDiscovery.FindPeers when requesting limit lower than number of peers available ([libp2p/go-libp2p-discovery#69](https://github.com/libp2p/go-libp2p-discovery/pull/69))
  - fix staticcheck ([libp2p/go-libp2p-discovery#70](https://github.com/libp2p/go-libp2p-discovery/pull/70))
- github.com/libp2p/go-libp2p-kad-dht (v0.11.1 -> v0.12.2):
  - fullrt rework batching (#720) ([libp2p/go-libp2p-kad-dht#720](https://github.com/libp2p/go-libp2p-kad-dht/pull/720))
  - sync: update CI config files ([libp2p/go-libp2p-kad-dht#712](https://github.com/libp2p/go-libp2p-kad-dht/pull/712))
  - fix staticcheck ([libp2p/go-libp2p-kad-dht#721](https://github.com/libp2p/go-libp2p-kad-dht/pull/721))
  - fix: fullrt dht bug fixes ([libp2p/go-libp2p-kad-dht#719](https://github.com/libp2p/go-libp2p-kad-dht/pull/719))
  - Crawler based DHT client (#709) ([libp2p/go-libp2p-kad-dht#709](https://github.com/libp2p/go-libp2p-kad-dht/pull/709))
  - test: fix unique addr check ([libp2p/go-libp2p-kad-dht#714](https://github.com/libp2p/go-libp2p-kad-dht/pull/714))
  - chore: update deps ([libp2p/go-libp2p-kad-dht#713](https://github.com/libp2p/go-libp2p-kad-dht/pull/713))
  - Add basic crawler (#663) ([libp2p/go-libp2p-kad-dht#663](https://github.com/libp2p/go-libp2p-kad-dht/pull/663))
  - various staticcheck fixes ([libp2p/go-libp2p-kad-dht#710](https://github.com/libp2p/go-libp2p-kad-dht/pull/710))
  - findpeer should work even on peers that are not part of DHT queries ([libp2p/go-libp2p-kad-dht#711](https://github.com/libp2p/go-libp2p-kad-dht/pull/711))
  - Extract DHT message sender from the DHT ([libp2p/go-libp2p-kad-dht#659](https://github.com/libp2p/go-libp2p-kad-dht/pull/659))
- github.com/libp2p/go-libp2p-noise (v0.1.2 -> v0.2.0):
  - Update github.com/flynn/noise to address nonce handling security issues ([libp2p/go-libp2p-noise#95](https://github.com/libp2p/go-libp2p-noise/pull/95))
  - fix staticcheck ([libp2p/go-libp2p-noise#96](https://github.com/libp2p/go-libp2p-noise/pull/96))
  - chore: update deps ([libp2p/go-libp2p-noise#94](https://github.com/libp2p/go-libp2p-noise/pull/94))
  - chore: relicense MIT/Apache-2.0 ([libp2p/go-libp2p-noise#93](https://github.com/libp2p/go-libp2p-noise/pull/93))
- github.com/libp2p/go-libp2p-peerstore (v0.2.6 -> v0.2.7):
  - fix: delete addrs when "updating" them to zero ([libp2p/go-libp2p-peerstore#157](https://github.com/libp2p/go-libp2p-peerstore/pull/157))
- github.com/libp2p/go-libp2p-quic-transport (v0.10.0 -> v0.11.1):
  - update quic-go, enable QUIC v1 (RFC 9000) ([libp2p/go-libp2p-quic-transport#207](https://github.com/libp2p/go-libp2p-quic-transport/pull/207))
  - update quic-go to v0.21.0-rc2 ([libp2p/go-libp2p-quic-transport#206](https://github.com/libp2p/go-libp2p-quic-transport/pull/206))
  - increase test timeout to reduce flakiness of test on Windows ([libp2p/go-libp2p-quic-transport#204](https://github.com/libp2p/go-libp2p-quic-transport/pull/204))
  - correctly export version negotiation failures to Prometheus ([libp2p/go-libp2p-quic-transport#205](https://github.com/libp2p/go-libp2p-quic-transport/pull/205))
  - update quic-go to v0.20.1 ([libp2p/go-libp2p-quic-transport#201](https://github.com/libp2p/go-libp2p-quic-transport/pull/201))
  - expose some Prometheus metrics ([libp2p/go-libp2p-quic-transport#200](https://github.com/libp2p/go-libp2p-quic-transport/pull/200))
  - update quic-go to v0.20.0 ([libp2p/go-libp2p-quic-transport#198](https://github.com/libp2p/go-libp2p-quic-transport/pull/198))
  - reduce the zstd window size from 8 MB to 32 KB ([libp2p/go-libp2p-quic-transport#195](https://github.com/libp2p/go-libp2p-quic-transport/pull/195))
  - compress qlogs when the QUIC connection is closed ([libp2p/go-libp2p-quic-transport#193](https://github.com/libp2p/go-libp2p-quic-transport/pull/193))
  - switch from gzip to zstd for qlog compression ([libp2p/go-libp2p-quic-transport#190](https://github.com/libp2p/go-libp2p-quic-transport/pull/190))
- github.com/libp2p/go-libp2p-swarm (v0.4.0 -> v0.5.0):
  - run connection gating tests on both TCP and QUIC ([libp2p/go-libp2p-swarm#258](https://github.com/libp2p/go-libp2p-swarm/pull/258))
  - fix: avoid returning typed nils ([libp2p/go-libp2p-swarm#257](https://github.com/libp2p/go-libp2p-swarm/pull/257))
  - fix staticcheck ([libp2p/go-libp2p-swarm#255](https://github.com/libp2p/go-libp2p-swarm/pull/255))
  - fix go vet ([libp2p/go-libp2p-swarm#253](https://github.com/libp2p/go-libp2p-swarm/pull/253))
  - New Dialer ([libp2p/go-libp2p-swarm#243](https://github.com/libp2p/go-libp2p-swarm/pull/243))
  - fix: use 64bit stream/conn IDs ([libp2p/go-libp2p-swarm#247](https://github.com/libp2p/go-libp2p-swarm/pull/247))
  - feat: close transports that implement io.Closer ([libp2p/go-libp2p-swarm#227](https://github.com/libp2p/go-libp2p-swarm/pull/227))
  - fix swarm transient conn (#241) ([libp2p/go-libp2p-swarm#241](https://github.com/libp2p/go-libp2p-swarm/pull/241))
  - Support for Hole punching (#233) ([libp2p/go-libp2p-swarm#233](https://github.com/libp2p/go-libp2p-swarm/pull/233))
  - Treat transient connections as opt-in when opening new streams ([libp2p/go-libp2p-swarm#236](https://github.com/libp2p/go-libp2p-swarm/pull/236))
  - avoid assigning a function to a variable ([libp2p/go-libp2p-swarm#239](https://github.com/libp2p/go-libp2p-swarm/pull/239))
  - only listen on localhost in tests ([libp2p/go-libp2p-swarm#238](https://github.com/libp2p/go-libp2p-swarm/pull/238))
  - prevent dialing addresses that we're listening on ([libp2p/go-libp2p-swarm#237](https://github.com/libp2p/go-libp2p-swarm/pull/237))
  - Enable QUIC in Test Swarm (#235) ([libp2p/go-libp2p-swarm#235](https://github.com/libp2p/go-libp2p-swarm/pull/235))
- github.com/libp2p/go-libp2p-transport-upgrader (v0.4.0 -> v0.4.2):
  - Expose underlying transport connection stat where available ([libp2p/go-libp2p-transport-upgrader#71](https://github.com/libp2p/go-libp2p-transport-upgrader/pull/71))
  - Implement support for simultaneous open (#25) ([libp2p/go-libp2p-transport-upgrader#25](https://github.com/libp2p/go-libp2p-transport-upgrader/pull/25))
- github.com/libp2p/go-libp2p-yamux (v0.5.1 -> v0.5.4):
  - remove Makefile ([libp2p/go-libp2p-yamux#35](https://github.com/libp2p/go-libp2p-yamux/pull/35))
- github.com/libp2p/go-netroute (v0.1.3 -> v0.1.6):
  - add js stub impl
- github.com/libp2p/go-sockaddr (v0.0.2 -> v0.1.1):
  - fix: allocate "any" socket type then cast ([libp2p/go-sockaddr#20](https://github.com/libp2p/go-sockaddr/pull/20))
  - fix: remove CGO functions ([libp2p/go-sockaddr#18](https://github.com/libp2p/go-sockaddr/pull/18))
- github.com/libp2p/go-tcp-transport (v0.2.1 -> v0.2.2):
  - use log.Warn instead of log.Warning ([libp2p/go-tcp-transport#77](https://github.com/libp2p/go-tcp-transport/pull/77))
  - add bandwidth-related metrics (for Linux and OSX) ([libp2p/go-tcp-transport#76](https://github.com/libp2p/go-tcp-transport/pull/76))
  - expose some Prometheus metrics ([libp2p/go-tcp-transport#75](https://github.com/libp2p/go-tcp-transport/pull/75))
  - enable TCP keepalives ([libp2p/go-tcp-transport#73](https://github.com/libp2p/go-tcp-transport/pull/73))
  - stop using the deprecated go-multiaddr-net package ([libp2p/go-tcp-transport#72](https://github.com/libp2p/go-tcp-transport/pull/72))
- github.com/libp2p/go-yamux/v2 (v2.0.0 -> v2.2.0):
  - make the initial stream receive window configurable ([libp2p/go-yamux#59](https://github.com/libp2p/go-yamux/pull/59))
  - set initial window size to spec value (256 kB), remove config option ([libp2p/go-yamux#57](https://github.com/libp2p/go-yamux/pull/57))
  - fix: don't change the receive window if we're forcing an update ([libp2p/go-yamux#56](https://github.com/libp2p/go-yamux/pull/56))
  - sync: update CI config files ([libp2p/go-yamux#55](https://github.com/libp2p/go-yamux/pull/55))
  - increase the receive window size if we're sending updates to frequently ([libp2p/go-yamux#54](https://github.com/libp2p/go-yamux/pull/54))
  - remove unused Stream.Shrink() method ([libp2p/go-yamux#52](https://github.com/libp2p/go-yamux/pull/52))
  - remove misleading comment about the MaxMessageSize ([libp2p/go-yamux#50](https://github.com/libp2p/go-yamux/pull/50))
  - clean up the receive window check ([libp2p/go-yamux#49](https://github.com/libp2p/go-yamux/pull/49))
  - don't reslice byte slices taking from the buffer ([libp2p/go-yamux#48](https://github.com/libp2p/go-yamux/pull/48))
  - don't reimplement io.ReadFull ([libp2p/go-yamux#38](https://github.com/libp2p/go-yamux/pull/38))
  - remove the recvLock in the stream ([libp2p/go-yamux#42](https://github.com/libp2p/go-yamux/pull/42))
  - remove the sendLock in the stream ([libp2p/go-yamux#41](https://github.com/libp2p/go-yamux/pull/41))
  - remove misleading statement about NAT traversal ([libp2p/go-yamux#45](https://github.com/libp2p/go-yamux/pull/45))
  - remove .gx directory, add last gx version to README ([libp2p/go-yamux#43](https://github.com/libp2p/go-yamux/pull/43))
  - reduce usage of goto ([libp2p/go-yamux#40](https://github.com/libp2p/go-yamux/pull/40))
  - remove unused error return value in Stream.processFlags ([libp2p/go-yamux#39](https://github.com/libp2p/go-yamux/pull/39))
- github.com/lucas-clemente/quic-go (v0.19.3 -> v0.21.1):
  - add support for Go 1.17 Beta 1 ([lucas-clemente/quic-go#3203](https://github.com/lucas-clemente/quic-go/pull/3203))
  - add a CI test that go mod vendor works ([lucas-clemente/quic-go#3202](https://github.com/lucas-clemente/quic-go/pull/3202))
  - prevent go mod vendor from stumbling over the Go 1.18 file ([lucas-clemente/quic-go#3195](https://github.com/lucas-clemente/quic-go/pull/3195))
  - remove CipherSuiteName and HkdfExtract for Go 1.17 ([lucas-clemente/quic-go#3192](https://github.com/lucas-clemente/quic-go/pull/3192))
  - fix relocation target for cipherSuiteTLS13ByID in Go 1.17
  - use HkdfExtract from x/crypto ([lucas-clemente/quic-go#3173](https://github.com/lucas-clemente/quic-go/pull/3173))
  - add support for QUIC v1, RFC 9000 ([lucas-clemente/quic-go#3190](https://github.com/lucas-clemente/quic-go/pull/3190))
  - use tls.CipherSuiteName, instead of wrapping it in the qtls package ([lucas-clemente/quic-go#3174](https://github.com/lucas-clemente/quic-go/pull/3174))
  - use a pre-generated test vectors to test hkdfExpandLabel ([lucas-clemente/quic-go#3175](https://github.com/lucas-clemente/quic-go/pull/3175))
  - reduce flakiness of packet number generation test ([lucas-clemente/quic-go#3181](https://github.com/lucas-clemente/quic-go/pull/3181))
  - simplify the qtls tests ([lucas-clemente/quic-go#3185](https://github.com/lucas-clemente/quic-go/pull/3185))
  - add support for Go 1.17 (tip) ([lucas-clemente/quic-go#3182](https://github.com/lucas-clemente/quic-go/pull/3182))
  - prevent quic-go from building on Go 1.17 ([lucas-clemente/quic-go#3180](https://github.com/lucas-clemente/quic-go/pull/3180))
  - fix DONT_FRAGMENT error when using a IPv6 connection on Windows ([lucas-clemente/quic-go#3178](https://github.com/lucas-clemente/quic-go/pull/3178))
  - use net.ErrClosed (for Go 1.16) ([lucas-clemente/quic-go#3163](https://github.com/lucas-clemente/quic-go/pull/3163))
  - use the new error types to log the reason why a connection is closed ([lucas-clemente/quic-go#3166](https://github.com/lucas-clemente/quic-go/pull/3166))
  - fix race condition in deadline integration test ([lucas-clemente/quic-go#3165](https://github.com/lucas-clemente/quic-go/pull/3165))
  - add support for QUIC v1 ([lucas-clemente/quic-go#3160](https://github.com/lucas-clemente/quic-go/pull/3160))
  - rework error return values ([lucas-clemente/quic-go#3159](https://github.com/lucas-clemente/quic-go/pull/3159))
  - declare Path MTU probe packets lost with the early retransmit timer ([lucas-clemente/quic-go#3152](https://github.com/lucas-clemente/quic-go/pull/3152))
  - declare the handshake confirmed when receiving an ACK for a 1-RTT packet ([lucas-clemente/quic-go#3148](https://github.com/lucas-clemente/quic-go/pull/3148))
  - trace and qlog version selection / negotiation ([lucas-clemente/quic-go#3153](https://github.com/lucas-clemente/quic-go/pull/3153))
  - set the don't fragment (DF) bit on Windows (#3155) ([lucas-clemente/quic-go#3155](https://github.com/lucas-clemente/quic-go/pull/3155))
  - fix doc comment for Tracer.TracerForConnection ([lucas-clemente/quic-go#3154](https://github.com/lucas-clemente/quic-go/pull/3154))
  - make it possible to associate a ConnectionTracer with a Session ([lucas-clemente/quic-go#3146](https://github.com/lucas-clemente/quic-go/pull/3146))
  - remove the .editorconfig ([lucas-clemente/quic-go#3147](https://github.com/lucas-clemente/quic-go/pull/3147))
  - don't use a lower RTT than 5ms after receiving a Retry packet ([lucas-clemente/quic-go#3129](https://github.com/lucas-clemente/quic-go/pull/3129))
  - don't pass the QUIC version to the StartedConnection event ([lucas-clemente/quic-go#3109](https://github.com/lucas-clemente/quic-go/pull/3109))
  - update the packet numbers in decoding test to the ones from the draft ([lucas-clemente/quic-go#3137](https://github.com/lucas-clemente/quic-go/pull/3137))
  - various amplification limit fixes ([lucas-clemente/quic-go#3132](https://github.com/lucas-clemente/quic-go/pull/3132))
  - fix calculation of the handshake idle timeout ([lucas-clemente/quic-go#3120](https://github.com/lucas-clemente/quic-go/pull/3120))
  - only start PMTUD after handshake confirmation ([lucas-clemente/quic-go#3138](https://github.com/lucas-clemente/quic-go/pull/3138))
  - don't regard PMTU probe packets as outstanding ([lucas-clemente/quic-go#3126](https://github.com/lucas-clemente/quic-go/pull/3126))
  - expose the draft-34 version ([lucas-clemente/quic-go#3100](https://github.com/lucas-clemente/quic-go/pull/3100))
  - clean up the testutils ([lucas-clemente/quic-go#3104](https://github.com/lucas-clemente/quic-go/pull/3104))
  - initialize the congestion controller with the actual max datagram size ([lucas-clemente/quic-go#3107](https://github.com/lucas-clemente/quic-go/pull/3107))
  - make it possible to trace acknowledged packets ([lucas-clemente/quic-go#3134](https://github.com/lucas-clemente/quic-go/pull/3134))
  - avoid type confusion between protocol.PacketType and logging.PacketType ([lucas-clemente/quic-go#3108](https://github.com/lucas-clemente/quic-go/pull/3108))
  - fix duplicate logging of errors when the first error was a timeout error ([lucas-clemente/quic-go#3112](https://github.com/lucas-clemente/quic-go/pull/3112))
  - use a tracer to make the packetization test more useful ([lucas-clemente/quic-go#3136](https://github.com/lucas-clemente/quic-go/pull/3136))
  - improve string representation of timeout errors ([lucas-clemente/quic-go#3118](https://github.com/lucas-clemente/quic-go/pull/3118))
  - fix flaky timeout test ([lucas-clemente/quic-go#3105](https://github.com/lucas-clemente/quic-go/pull/3105))
  - fix calculation of the time for the next keep alive
  - add a 0-RTT test with different connecton ID lengths ([lucas-clemente/quic-go#3098](https://github.com/lucas-clemente/quic-go/pull/3098))
  - only run Ginkgo focus detection in staged files in pre-commit hook ([lucas-clemente/quic-go#3099](https://github.com/lucas-clemente/quic-go/pull/3099))
  - allow 0-RTT when flow control windows are increased ([lucas-clemente/quic-go#3096](https://github.com/lucas-clemente/quic-go/pull/3096))
  - improve the 0-RTT rejection integration test ([lucas-clemente/quic-go#3097](https://github.com/lucas-clemente/quic-go/pull/3097))
  - rename config values for flow control limits ([lucas-clemente/quic-go#3089](https://github.com/lucas-clemente/quic-go/pull/3089))
  - allow 0-RTT resumption if the server's stream limit was increased ([lucas-clemente/quic-go#3086](https://github.com/lucas-clemente/quic-go/pull/3086))
  - cache the serialized OOB in the conn, not in the packet info  ([lucas-clemente/quic-go#3093](https://github.com/lucas-clemente/quic-go/pull/3093))
  - use code points from x/sys/unix for PKTINFO syscalls ([lucas-clemente/quic-go#3094](https://github.com/lucas-clemente/quic-go/pull/3094))
  - make it possible to detect version negotiation failures in logging, fix qlogging of those ([lucas-clemente/quic-go#3092](https://github.com/lucas-clemente/quic-go/pull/3092))
  - make the initial stream / connection flow control windows configurable ([lucas-clemente/quic-go#3083](https://github.com/lucas-clemente/quic-go/pull/3083))
  - only apply server's transport parameters after handshake completion ([lucas-clemente/quic-go#3085](https://github.com/lucas-clemente/quic-go/pull/3085))
  - fix documentation for baseFlowController.UpdateSendWindow ([lucas-clemente/quic-go#3087](https://github.com/lucas-clemente/quic-go/pull/3087))
  - set the Content-Length for HTTP/3 responses ([lucas-clemente/quic-go#3091](https://github.com/lucas-clemente/quic-go/pull/3091))
  - update the flow control windows of streams opened in 0-RTT ([lucas-clemente/quic-go#3088](https://github.com/lucas-clemente/quic-go/pull/3088))
  - Use the correct source IP when binding multiple IPs ([lucas-clemente/quic-go#3067](https://github.com/lucas-clemente/quic-go/pull/3067))
  - fix race condition when receiving 0-RTT packets ([lucas-clemente/quic-go#3074](https://github.com/lucas-clemente/quic-go/pull/3074))
  - require the application to handle 0-RTT rejection ([lucas-clemente/quic-go#3066](https://github.com/lucas-clemente/quic-go/pull/3066))
  - add an internal queue to signal that a datagram frame has been dequeued ([lucas-clemente/quic-go#3081](https://github.com/lucas-clemente/quic-go/pull/3081))
  - increase the maximum size of DATAGRAM frames ([lucas-clemente/quic-go#2966](https://github.com/lucas-clemente/quic-go/pull/2966))
  - remove non-functioning 0-RTT test with different conn ID lengths ([lucas-clemente/quic-go#3079](https://github.com/lucas-clemente/quic-go/pull/3079))
  - remove stray struct equality check ([lucas-clemente/quic-go#3078](https://github.com/lucas-clemente/quic-go/pull/3078))
  - fix issuing of connection IDs when dialing a 0-RTT connections ([lucas-clemente/quic-go#3058](https://github.com/lucas-clemente/quic-go/pull/3058))
  - only accept 0-RTT it the active_connection_id_limit didn't change ([lucas-clemente/quic-go#3060](https://github.com/lucas-clemente/quic-go/pull/3060))
  - remove unused error return value from HandleMaxStreamsFrame ([lucas-clemente/quic-go#3072](https://github.com/lucas-clemente/quic-go/pull/3072))
  - fix flaky accept queue integration test ([lucas-clemente/quic-go#3068](https://github.com/lucas-clemente/quic-go/pull/3068))
  - don't reset the QPACK encoder / decoder streams ([lucas-clemente/quic-go#3063](https://github.com/lucas-clemente/quic-go/pull/3063))
  - remove incorrect logging for client side retry packet ([lucas-clemente/quic-go#3071](https://github.com/lucas-clemente/quic-go/pull/3071))
  - allow sending 1xx responses (#3047) ([lucas-clemente/quic-go#3047](https://github.com/lucas-clemente/quic-go/pull/3047))
  - fix retry key and nonce for draft-34 ([lucas-clemente/quic-go#3062](https://github.com/lucas-clemente/quic-go/pull/3062))
  - implement DPLPMTUD ([lucas-clemente/quic-go#3028](https://github.com/lucas-clemente/quic-go/pull/3028))
  - only read multiple packets at a time after handshake completion ([lucas-clemente/quic-go#3041](https://github.com/lucas-clemente/quic-go/pull/3041))
  - make the certificate verificiation integration tests more explicit ([lucas-clemente/quic-go#3040](https://github.com/lucas-clemente/quic-go/pull/3040))
  - update gomock to v1.5.0, use mockgen source mode ([lucas-clemente/quic-go#3049](https://github.com/lucas-clemente/quic-go/pull/3049))
  - trace dropping of 0-RTT keys ([lucas-clemente/quic-go#3054](https://github.com/lucas-clemente/quic-go/pull/3054))
  - improve timeout measurement in the timeout test ([lucas-clemente/quic-go#3042](https://github.com/lucas-clemente/quic-go/pull/3042))
  - add a randomized test for the received_packet_history ([lucas-clemente/quic-go#3052](https://github.com/lucas-clemente/quic-go/pull/3052))
  - fix documentation of default values for MaxReceive{Stream, Connection}FlowControlWindow ([lucas-clemente/quic-go#3055](https://github.com/lucas-clemente/quic-go/pull/3055))
  - refactor merge packet number ranges ([lucas-clemente/quic-go#3051](https://github.com/lucas-clemente/quic-go/pull/3051))
  - add draft-34 to support versions in README
  - update README to reflect dropped Go 1.14 support
  - remove redundant nil-check in the packet packer  ([lucas-clemente/quic-go#3048](https://github.com/lucas-clemente/quic-go/pull/3048))
  - avoid using rand.Source ([lucas-clemente/quic-go#3046](https://github.com/lucas-clemente/quic-go/pull/3046))
  - update Go to 1.16, drop support for 1.14 ([lucas-clemente/quic-go#3045](https://github.com/lucas-clemente/quic-go/pull/3045))
  - fix error message when the UDP receive buffer size can't be increased ([lucas-clemente/quic-go#3039](https://github.com/lucas-clemente/quic-go/pull/3039))
  - add the time_format field to qlog common_fields ([lucas-clemente/quic-go#3038](https://github.com/lucas-clemente/quic-go/pull/3038))
  - log connection IDs without the 0x prefix ([lucas-clemente/quic-go#3036](https://github.com/lucas-clemente/quic-go/pull/3036))
  - add support for QUIC draft-34 ([lucas-clemente/quic-go#3031](https://github.com/lucas-clemente/quic-go/pull/3031))
  - fix qtls imports in mockgen generated mocks ([lucas-clemente/quic-go#3037](https://github.com/lucas-clemente/quic-go/pull/3037))
  - improve error message when the read buffer size can't be set ([lucas-clemente/quic-go#3030](https://github.com/lucas-clemente/quic-go/pull/3030))
  - qlog the quic-go version ([lucas-clemente/quic-go#3033](https://github.com/lucas-clemente/quic-go/pull/3033))
  - remove the metrics package ([lucas-clemente/quic-go#3032](https://github.com/lucas-clemente/quic-go/pull/3032))
  - expose the constructor for the qlog connection tracer ([lucas-clemente/quic-go#3034](https://github.com/lucas-clemente/quic-go/pull/3034))
  - expose the constructor for the multipexed connection tracer ([lucas-clemente/quic-go#3035](https://github.com/lucas-clemente/quic-go/pull/3035))
  - make sure the server is stopped before closing all server sessions ([lucas-clemente/quic-go#3020](https://github.com/lucas-clemente/quic-go/pull/3020))
  - increase the size of the send queue ([lucas-clemente/quic-go#3016](https://github.com/lucas-clemente/quic-go/pull/3016))
  - prioritize receiving packets over sending out more packets ([lucas-clemente/quic-go#3015](https://github.com/lucas-clemente/quic-go/pull/3015))
  - reenable key updates for HTTP/3 ([lucas-clemente/quic-go#3017](https://github.com/lucas-clemente/quic-go/pull/3017))
  - check for errors after handling each previously undecryptable packet ([lucas-clemente/quic-go#3011](https://github.com/lucas-clemente/quic-go/pull/3011))
  - fix flaky streams map test on Windows ([lucas-clemente/quic-go#3013](https://github.com/lucas-clemente/quic-go/pull/3013))
  - fix flaky stream cancelation integration test ([lucas-clemente/quic-go#3014](https://github.com/lucas-clemente/quic-go/pull/3014))
  - preallocate a slice of one frame when packing a packet ([lucas-clemente/quic-go#3018](https://github.com/lucas-clemente/quic-go/pull/3018))
  - allow sending of ACKs when pacing limited ([lucas-clemente/quic-go#3010](https://github.com/lucas-clemente/quic-go/pull/3010))
  - fix qlogging of the packet payload length ([lucas-clemente/quic-go#3004](https://github.com/lucas-clemente/quic-go/pull/3004))
  - corrupt more ACKs in the MITM test ([lucas-clemente/quic-go#3007](https://github.com/lucas-clemente/quic-go/pull/3007))
  - fix flaky key update integration test ([lucas-clemente/quic-go#3005](https://github.com/lucas-clemente/quic-go/pull/3005))
  - immediately complete streams that were canceled, drop retransmissions ([lucas-clemente/quic-go#3003](https://github.com/lucas-clemente/quic-go/pull/3003))
  - stop generating new packets when the send queue is full ([lucas-clemente/quic-go#2971](https://github.com/lucas-clemente/quic-go/pull/2971))
  - allow access to the underlying quic.Stream from a http.ResponseWriter ([lucas-clemente/quic-go#2993](https://github.com/lucas-clemente/quic-go/pull/2993))
  - remove stay print statement from session test
  - allow receiving of multiple packets before sending a packet ([lucas-clemente/quic-go#2984](https://github.com/lucas-clemente/quic-go/pull/2984))
  - use cryptographic random for determining skipped packet numbers ([lucas-clemente/quic-go#2940](https://github.com/lucas-clemente/quic-go/pull/2940))
  - fix interpretation of time.Time{} as a pacing deadline ([lucas-clemente/quic-go#2980](https://github.com/lucas-clemente/quic-go/pull/2980))
  - qlog restored transport parameters ([lucas-clemente/quic-go#2991](https://github.com/lucas-clemente/quic-go/pull/2991))
  - use a pkg.go.dev instead of a GoDoc badge ([lucas-clemente/quic-go#2982](https://github.com/lucas-clemente/quic-go/pull/2982))
  - introduce a separate queue for undecryptable packets ([lucas-clemente/quic-go#2988](https://github.com/lucas-clemente/quic-go/pull/2988))
  - improve 0-RTT queue ([lucas-clemente/quic-go#2990](https://github.com/lucas-clemente/quic-go/pull/2990))
  - simplify switch statement in the transport parameter parser ([lucas-clemente/quic-go#2995](https://github.com/lucas-clemente/quic-go/pull/2995))
  - remove unneeded overflow check when parsing the max_ack_delay ([lucas-clemente/quic-go#2996](https://github.com/lucas-clemente/quic-go/pull/2996))
  - remove unneeded check in receivedPacketHandler.IsPotentiallyDuplicate ([lucas-clemente/quic-go#2998](https://github.com/lucas-clemente/quic-go/pull/2998))
  - qlog the max_datagram_frame_size transport parameter ([lucas-clemente/quic-go#2997](https://github.com/lucas-clemente/quic-go/pull/2997))
  - qlog draft-02 fixes ([lucas-clemente/quic-go#2987](https://github.com/lucas-clemente/quic-go/pull/2987))
  - fix flaky qlog test ([lucas-clemente/quic-go#2981](https://github.com/lucas-clemente/quic-go/pull/2981))
  - only run gofumpt on .go files in pre-commit hook ([lucas-clemente/quic-go#2983](https://github.com/lucas-clemente/quic-go/pull/2983))
  - fix outdated comment for the http3.Server
  - make the OpenStreamSync cancelation test less flaky ([lucas-clemente/quic-go#2978](https://github.com/lucas-clemente/quic-go/pull/2978))
  - add some useful pre-commit hooks ([lucas-clemente/quic-go#2979](https://github.com/lucas-clemente/quic-go/pull/2979))
  - publicize QUIC varint reading and writing ([lucas-clemente/quic-go#2973](https://github.com/lucas-clemente/quic-go/pull/2973))
  - add a http3.RoundTripOpt to skip the request scheme check ([lucas-clemente/quic-go#2962](https://github.com/lucas-clemente/quic-go/pull/2962))
  - use the standard quic.Config in the deadline tests ([lucas-clemente/quic-go#2970](https://github.com/lucas-clemente/quic-go/pull/2970))
  - update golangci-lint to v1.34.1 ([lucas-clemente/quic-go#2964](https://github.com/lucas-clemente/quic-go/pull/2964))
  - update text about QUIC versions in the README ([lucas-clemente/quic-go#2975](https://github.com/lucas-clemente/quic-go/pull/2975))
  - remove stray TODO in the http3.Server
  - add support for Go 1.16 ([lucas-clemente/quic-go#2953](https://github.com/lucas-clemente/quic-go/pull/2953))
  - cancel reading on unidirectional streams when the stream type is unknown ([lucas-clemente/quic-go#2952](https://github.com/lucas-clemente/quic-go/pull/2952))
  - remove duplicate check of the URL scheme in the HTTP/3 client ([lucas-clemente/quic-go#2956](https://github.com/lucas-clemente/quic-go/pull/2956))
  - increase queueing duration in 0-RTT queue test to reduce flakiness ([lucas-clemente/quic-go#2954](https://github.com/lucas-clemente/quic-go/pull/2954))
  - implement the HTTP/3 Datagram negotiation ([lucas-clemente/quic-go#2951](https://github.com/lucas-clemente/quic-go/pull/2951))
  - implement HTTP/3 control stream handling ([lucas-clemente/quic-go#2949](https://github.com/lucas-clemente/quic-go/pull/2949))
  - fix flaky sentPacketHandler test ([lucas-clemente/quic-go#2950](https://github.com/lucas-clemente/quic-go/pull/2950))
  - don't retransmit PING frames added to ACK-only packets ([lucas-clemente/quic-go#2942](https://github.com/lucas-clemente/quic-go/pull/2942))
  - move the transport parameter stream limit check to the parser  ([lucas-clemente/quic-go#2944](https://github.com/lucas-clemente/quic-go/pull/2944))
  - remove unused initialVersion variable in session ([lucas-clemente/quic-go#2946](https://github.com/lucas-clemente/quic-go/pull/2946))
  - remove unneeded check for the peer's transport parameters ([lucas-clemente/quic-go#2945](https://github.com/lucas-clemente/quic-go/pull/2945))
  - add the H3_MESSAGE_ERROR ([lucas-clemente/quic-go#2947](https://github.com/lucas-clemente/quic-go/pull/2947))
  - simplify Read and Write mock calls in http3 tests ([lucas-clemente/quic-go#2948](https://github.com/lucas-clemente/quic-go/pull/2948))
  - implement the datagram draft ([lucas-clemente/quic-go#2162](https://github.com/lucas-clemente/quic-go/pull/2162))
  - fix logging of bytes_in_flight when receiving an ACK ([lucas-clemente/quic-go#2937](https://github.com/lucas-clemente/quic-go/pull/2937))
  - trace when a packet is dropped because the receivedPackets chan is full ([lucas-clemente/quic-go#2939](https://github.com/lucas-clemente/quic-go/pull/2939))
  - various improvements to the packet number generator ([lucas-clemente/quic-go#2905](https://github.com/lucas-clemente/quic-go/pull/2905))
  - introduce a quic.Config.HandshakeIdleTimeout, remove HandshakeTimeout ([lucas-clemente/quic-go#2930](https://github.com/lucas-clemente/quic-go/pull/2930))
  - allow up to 20 byte for the initial connection IDs ([lucas-clemente/quic-go#2936](https://github.com/lucas-clemente/quic-go/pull/2936))
  - reduce memory footprint of undecryptable packet handling ([lucas-clemente/quic-go#2932](https://github.com/lucas-clemente/quic-go/pull/2932))
  - use a buffer from the pool for composing Retry packets ([lucas-clemente/quic-go#2934](https://github.com/lucas-clemente/quic-go/pull/2934))
  - release the packet buffer after sending a CONNECTION_CLOSE in the server ([lucas-clemente/quic-go#2935](https://github.com/lucas-clemente/quic-go/pull/2935))
  - move integration tests to GitHub Actions, disable Travis ([lucas-clemente/quic-go#2891](https://github.com/lucas-clemente/quic-go/pull/2891))
  - use golang.org/x/sys/unix instead of syscall ([lucas-clemente/quic-go#2927](https://github.com/lucas-clemente/quic-go/pull/2927))
  - add support for the connection_closed qlog event ([lucas-clemente/quic-go#2921](https://github.com/lucas-clemente/quic-go/pull/2921))
  - qlog tokens in NEW_TOKEN frames, Retry packets and Initial packets ([lucas-clemente/quic-go#2863](https://github.com/lucas-clemente/quic-go/pull/2863))
  - qlog the packet_type as part of the packet header, not the event itself ([lucas-clemente/quic-go#2758](https://github.com/lucas-clemente/quic-go/pull/2758))
  - use the new, streaming-friendly NDJSON-based qlog encoding ([lucas-clemente/quic-go#2736](https://github.com/lucas-clemente/quic-go/pull/2736))
  - add a generic Debug() function to the connection tracer ([lucas-clemente/quic-go#2909](https://github.com/lucas-clemente/quic-go/pull/2909))
  - remove unnecessary call to time.Now() when sending a packet ([lucas-clemente/quic-go#2911](https://github.com/lucas-clemente/quic-go/pull/2911))
  - remove support for quic-trace ([lucas-clemente/quic-go#2913](https://github.com/lucas-clemente/quic-go/pull/2913))
  - reduce the maximum number of ACK ranges ([lucas-clemente/quic-go#2887](https://github.com/lucas-clemente/quic-go/pull/2887))
  - don't allocate for acked packets ([lucas-clemente/quic-go#2899](https://github.com/lucas-clemente/quic-go/pull/2899))
  - avoid allocating when detecting lost packets ([lucas-clemente/quic-go#2898](https://github.com/lucas-clemente/quic-go/pull/2898))
  - use the string optimization for map keys in the packet handler map ([lucas-clemente/quic-go#2892](https://github.com/lucas-clemente/quic-go/pull/2892))
  - use a single map in the incoming streams map ([lucas-clemente/quic-go#2890](https://github.com/lucas-clemente/quic-go/pull/2890))
- github.com/marten-seemann/qtls-go1-15 (v0.1.1 -> v0.1.4):
  - use a prefix for client session cache keys
  - add callbacks to store and restore app data along a session state
  - don't use TLS 1.3 compatibility mode when using alternative record layer
  - delete the session ticket after attempting 0-RTT
  - reject 0-RTT when a different ALPN is chosen
  - encode the ALPN into the session ticket
  - add a field to the ConnectionState to tell if 0-RTT was used
  - add a callback to tell the client about rejection of 0-RTT
  - don't offer 0-RTT after a HelloRetryRequest
  - add Accept0RTT to Config callback to decide if 0-RTT should be accepted
- github.com/marten-seemann/qtls-go1-16 (null -> v0.1.3):
  - use a prefix for client session cache keys
  - add callbacks to store and restore app data along a session state
  - don't use TLS 1.3 compatibility mode when using alternative record layer
  - delete the session ticket after attempting 0-RTT
  - reject 0-RTT when a different ALPN is chosen
- github.com/multiformats/go-multiaddr (v0.3.1 -> v0.3.2):
  - fix(net): export new net.Addr conversion registration functions ([multiformats/go-multiaddr#152](https://github.com/multiformats/go-multiaddr/pull/152))
  - sync: run go mod tidy (and set Go 1.15) and gofmt -s in copy workflow (#146) ([multiformats/go-multiaddr#146](https://github.com/multiformats/go-multiaddr/pull/146))
  - more linter fixes ([multiformats/go-multiaddr#145](https://github.com/multiformats/go-multiaddr/pull/145))
  - fix go vet and staticcheck failures ([multiformats/go-multiaddr#143](https://github.com/multiformats/go-multiaddr/pull/143))
  - don't listen on all interfaces in tests, unless on CI ([multiformats/go-multiaddr#136](https://github.com/multiformats/go-multiaddr/pull/136))
  - Fix Local Address on TCP connections ([multiformats/go-multiaddr#135](https://github.com/multiformats/go-multiaddr/pull/135))
- github.com/multiformats/go-multiaddr-dns (v0.2.0 -> v0.3.1):
  - Normalize domains to fqdn for resolver selection ([multiformats/go-multiaddr-dns#27](https://github.com/multiformats/go-multiaddr-dns/pull/27))
  - refactor Resolver to support custom per-TLD resolvers ([multiformats/go-multiaddr-dns#26](https://github.com/multiformats/go-multiaddr-dns/pull/26))
  - feat: exposes backend ([multiformats/go-multiaddr-dns#25](https://github.com/multiformats/go-multiaddr-dns/pull/25))
- github.com/multiformats/go-multihash (v0.0.14 -> v0.0.15):
  - Refactor registry system: no direct dependencies; expose standard hash.Hash; be a data carrier. ([multiformats/go-multihash#136](https://github.com/multiformats/go-multihash/pull/136))
- github.com/multiformats/go-multistream (v0.2.0 -> v0.2.2):
  - change the simultaneous open protocol to /libp2p/simultaneous-connect ([multiformats/go-multistream#66](https://github.com/multiformats/go-multistream/pull/66))
  - fix the lazy stress read test on Windows ([multiformats/go-multistream#61](https://github.com/multiformats/go-multistream/pull/61))
  - fix go vet and staticcheck errors ([multiformats/go-multistream#60](https://github.com/multiformats/go-multistream/pull/60))
  - Implement simultaneous open extension ([multiformats/go-multistream#42](https://github.com/multiformats/go-multistream/pull/42))
  - reduce the number of streams in the stress tests, fix error handling ([multiformats/go-multistream#54](https://github.com/multiformats/go-multistream/pull/54))
- github.com/whyrusleeping/cbor-gen (v0.0.0-20200710004633-5379fc63235d -> v0.0.0-20210219115102-f37d292932f2):
  - feat: allow unmarshaling of struct with more fields than marshaled struct ([whyrusleeping/cbor-gen#50](https://github.com/whyrusleeping/cbor-gen/pull/50))
  - chore: add a license file ([whyrusleeping/cbor-gen#49](https://github.com/whyrusleeping/cbor-gen/pull/49))
  - fix: enforce maxlen in ReadByteArray() ([whyrusleeping/cbor-gen#43](https://github.com/whyrusleeping/cbor-gen/pull/43))
  - use unix nanoseconds for encoding Cbortime ([whyrusleeping/cbor-gen#41](https://github.com/whyrusleeping/cbor-gen/pull/41))
  - add json marshalers to CborTime
  - add a helper for roundtripping time.time objects ([whyrusleeping/cbor-gen#40](https://github.com/whyrusleeping/cbor-gen/pull/40))
  - Add a validate function. ([whyrusleeping/cbor-gen#39](https://github.com/whyrusleeping/cbor-gen/pull/39))
  - Fix import handling ([whyrusleeping/cbor-gen#38](https://github.com/whyrusleeping/cbor-gen/pull/38))
  - Optimize discarding in ScanForLinks ([whyrusleeping/cbor-gen#36](https://github.com/whyrusleeping/cbor-gen/pull/36))
  - Always allocate scratch space when marshalling into a map. ([whyrusleeping/cbor-gen#37](https://github.com/whyrusleeping/cbor-gen/pull/37))
  - optimize byte reading ([whyrusleeping/cbor-gen#35](https://github.com/whyrusleeping/cbor-gen/pull/35))
  - Optimize decoding ([whyrusleeping/cbor-gen#34](https://github.com/whyrusleeping/cbor-gen/pull/34))
  - Fix named string issue ([whyrusleeping/cbor-gen#30](https://github.com/whyrusleeping/cbor-gen/pull/30))
  - Fix encoding/decoding fixed byte arrays ([whyrusleeping/cbor-gen#29](https://github.com/whyrusleeping/cbor-gen/pull/29))
  - fix overread on scanforlinks ([whyrusleeping/cbor-gen#28](https://github.com/whyrusleeping/cbor-gen/pull/28))

### ❤️ Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Marten Seemann | 358 | +17444/-12000 | 1268 |
| Eric Myhre | 82 | +9672/-2459 | 328 |
| Ian Davis | 7 | +8421/-737 | 116 |
| Daniel Martí | 18 | +2733/-4377 | 313 |
| Adin Schmahmann | 46 | +5387/-1289 | 125 |
| Steven Allen | 95 | +3278/-1861 | 200 |
| hannahhoward | 14 | +1380/-3667 | 84 |
| gammazero | 29 | +2520/-1161 | 88 |
| Hector Sanjuan | 12 | +511/-3129 | 52 |
| vyzo | 77 | +2198/-940 | 117 |
| Will Scott | 12 | +912/-593 | 37 |
| Dirk McCormick | 3 | +1384/-63 | 14 |
| Andrew Gillis | 3 | +1231/-39 | 19 |
| Marcin Rataj | 37 | +549/-308 | 72 |
| Aarsh Shah | 13 | +668/-86 | 30 |
| Olivier Poitrey | 1 | +469/-182 | 15 |
| Rod Vagg | 9 | +364/-184 | 14 |
| whyrusleeping | 5 | +253/-32 | 11 |
| Cory Schwartz | 10 | +162/-115 | 37 |
| Adrian Lanzafame | 8 | +212/-60 | 11 |
| aarshkshah1992 | 7 | +102/-110 | 9 |
| Jakub Sztandera | 7 | +126/-75 | 16 |
| huoju | 4 | +127/-41 | 6 |
| acruikshank | 6 | +32/-24 | 7 |
| Toby | 1 | +41/-1 | 2 |
| Naveen | 1 | +40/-0 | 1 |
| Bogdan Stirbat | 1 | +22/-16 | 2 |
| Kévin Dunglas | 1 | +32/-2 | 2 |
| Nicholas Bollweg | 1 | +22/-0 | 1 |
| q191201771 | 2 | +4/-11 | 2 |
| Mathis Engelbart | 1 | +12/-2 | 1 |
| requilence | 1 | +13/-0 | 1 |
| divingpetrel | 1 | +7/-4 | 2 |
| Oli Evans | 2 | +9/-2 | 3 |
| Lucas Molas | 3 | +7/-3 | 3 |
| RubenKelevra | 3 | +2/-6 | 3 |
| Will | 1 | +1/-5 | 1 |
| Jorropo | 1 | +4/-2 | 1 |
| Ju Huo | 1 | +2/-2 | 1 |
| zhoujiajie | 1 | +1/-1 | 1 |
| Luflosi | 1 | +1/-1 | 1 |
| Jonathan Rudenberg | 1 | +1/-1 | 1 |
| David Pflug | 1 | +1/-1 | 1 |
| Ari Mattila | 1 | +1/-1 | 1 |
| Yingrong Zhao | 1 | +0/-1 | 1 |

## v0.8.0 2021-02-18

We're happy to announce go-ipfs 0.8.0! This is planned to be a fairly small release focused on integrating in the new pinning service/remote pinning [API](https://github.com/ipfs/pinning-services-api-spec) that makes the experience of managing pins across pinning services easier and more uniform.

### 🔦 Highlights

#### 🧷 Remote pinning services

There is now support for asking remote services to pin data for you. This means anyone can implement the [spec](https://ipfs.github.io/pinning-services-api-spec/) (developed in this [repo](https://github.com/ipfs/pinning-services-api-spec)) and allow for pin management.

All of the CLI (and corresponding HTTP API) commands are available under `ipfs pin remote`.

This remote pinning service comes with a redesign of how we're thinking about pinning and includes some commonly requested features such as:
- Pins can have names (and coming soon metadata)
- The same content can be pinned multiple times, but of course stored only once
  - This allows applications using the same pinning service to manage their own pins without worrying about removing content important to another application
- Data can be pinned in either the foreground or background


Examples include:
```
ipfs pin remote service add myservice https://myservice.tld:1234/api/path myaccess key

ipfs pin remote add /ipfs/bafymydata --service=myservice --name=myfile 
ipfs pin remote ls --service=myservice --name=myfile
ipfs pin remote ls --service=myservice --cid=bafymydata
ipfs pin remote rm --serivce=myservice --name=myfile
```
A few notes:

Remote pinning services work with recursive pins. This means commands like `ipfs pin remote ls` will not list indirectly pinned CIDs.

While pinning service data is stored in the configuration file it cannot be edited directly via the `ipfs config` commands due to the sensitive nature of pinning service API keys. The `ipfs pin remote service` commands can be used for interacting with remote service settings.

#### 📌 Faster local pinning and unpinning

The pinning subsystem has been redesigned to be much faster and more flexible in how it tracks pins. For users who are working with many pins this will lead to a big speed increase in listing and modifying the set of pinned items as well as decreased memory usage.

Part of the redesign was setup to account for being able to interact with local pins the same way we can now interact with remote pins (e.g. names, being allowed to pin the same CID multiple times, etc.). Keep posted for more improvements to pinning.

#### DNSLink names on https:// subdomains

Previously DNSLink names would have trouble loading over subdomain gateways with HTTPS support since there is no way to get multilevel wildcard certificates (e.g. `en.wikipedia-on-ipfs.org.ipns.dweb.link` cannot be covered by `*.ipns.dweb.link`). Therefore, when trying to load DNSLink names over https:// subdomains go-ipfs we now forward to an encoded DNS name. Since DNS names cannot contain `.` in them they are escaped using `-`.

`/ipns/en.wikipedia-on-ipfs.org` →
`ipns://en.wikipedia-on-ipfs.org`  →
`https://dweb.link/ipns/en.wikipedia-on-ipfs.org` 
`https://en-wikipedia--on--ipfs-org.ipns.dweb.link` :point_left: _a single DNS label, no TLS error_

#### QUIC update

QUIC support has received a number of upgrades, including the ability to take advantage of larger UDP receive buffers for increased performance.

Linux users may notice a logged error on daemon startup if your system needs extra configuration to allow IPFS increase the buffer size. A helpful link for resolving this is in the log message as well as [here](https://github.com/lucas-clemente/quic-go/wiki/UDP-Receive-Buffer-Size).

#### 👋 No more Darwin 386 builds

Go 1.15 (the latest version of Go) [no longer supports](https://github.com/golang/go/issues/34749) Darwin 386 and so we are dropping support as well.

### Changelog

- github.com/ipfs/go-ipfs:
  - Release v0.8.0
  - docs: RepinInterval
  - style: docs/config.md
  - style: improved MFS PinName example
  - docs: Pinning.RemoteServices.Policies
  - fix: decrease log level of opencensus initialization ([ipfs/go-ipfs#7815](https://github.com/ipfs/go-ipfs/pull/7815))
  - Register oc metrics ([ipfs/go-ipfs#7593](https://github.com/ipfs/go-ipfs/pull/7593))
  - add remote pinning to ipfs command (#7661) ([ipfs/go-ipfs#7661](https://github.com/ipfs/go-ipfs/pull/7661))
  - More p2p proxy checks ([ipfs/go-ipfs#7797](https://github.com/ipfs/go-ipfs/pull/7797))
  - Use datastore based pinning ([ipfs/go-ipfs#7750](https://github.com/ipfs/go-ipfs/pull/7750))
  - fix: return an error when an unknown object type is passed ([ipfs/go-ipfs#7795](https://github.com/ipfs/go-ipfs/pull/7795))
  - clarify why ipfs file ls is being deprecated ([ipfs/go-ipfs#7755](https://github.com/ipfs/go-ipfs/pull/7755))
  - fix: ipfs dag export uses the CoreAPI and respects the offline flag ([ipfs/go-ipfs#7753](https://github.com/ipfs/go-ipfs/pull/7753))
  - return an error when trying to download fs-repo-migrations for linux + musl ([ipfs/go-ipfs#7735](https://github.com/ipfs/go-ipfs/pull/7735))
  - fix: do not create a new (unused) peerID when initializing from config ([ipfs/go-ipfs#7730](https://github.com/ipfs/go-ipfs/pull/7730))
  - docs: Add a link in config.md ([ipfs/go-ipfs#7780](https://github.com/ipfs/go-ipfs/pull/7780))
  - update libp2p for stream closure refactor ([ipfs/go-ipfs#7747](https://github.com/ipfs/go-ipfs/pull/7747))
  - Fix typo in ipfs dag stat command ([ipfs/go-ipfs#7761](https://github.com/ipfs/go-ipfs/pull/7761))
  - docs(readme): key rotation in docker (#7721) ([ipfs/go-ipfs#7721](https://github.com/ipfs/go-ipfs/pull/7721))
  - fix(dnslink-gw): breadcrumbs and CID column when dir listing ([ipfs/go-ipfs#7699](https://github.com/ipfs/go-ipfs/pull/7699))
  - fix(gw): preserve query on website redirect ([ipfs/go-ipfs#7727](https://github.com/ipfs/go-ipfs/pull/7727))
  - feat: ipfs-webui v2.11.4 ([ipfs/go-ipfs#7716](https://github.com/ipfs/go-ipfs/pull/7716))
  - docs: how the ipfs snap is built and published ([ipfs/go-ipfs#7725](https://github.com/ipfs/go-ipfs/pull/7725))
  - fix: webui on ipv6 localhost ([ipfs/go-ipfs#7731](https://github.com/ipfs/go-ipfs/pull/7731))
  - Add missing plugin support on FreeBSD ([ipfs/go-ipfs#7722](https://github.com/ipfs/go-ipfs/pull/7722))
  - fix error when computing coverage ([ipfs/go-ipfs#7726](https://github.com/ipfs/go-ipfs/pull/7726))
  - docs(config): X-Forwarded-Host ([ipfs/go-ipfs#7651](https://github.com/ipfs/go-ipfs/pull/7651))
  - chore: webui v2.11.2 ([ipfs/go-ipfs#7703](https://github.com/ipfs/go-ipfs/pull/7703))
  - Add task for updating CLI docs right after updating the HTTP-api docs ([ipfs/go-ipfs#7711](https://github.com/ipfs/go-ipfs/pull/7711))
  - feat(gateway): Content-Disposition improvements ([ipfs/go-ipfs#7677](https://github.com/ipfs/go-ipfs/pull/7677))
  - fix build on Plan 9 ([ipfs/go-ipfs#7690](https://github.com/ipfs/go-ipfs/pull/7690))
  - docs: update changelog for v0.7.0
  - chore: bump webui version
  - fix: remove the (empty) alias for --peerid-base
  - fix: use override GOFLAGS changes from 480defab689610550ee3d346e31441a2bb881fcb but keep trimpath usage as is
  - Revert "fix: override GOFLAGS"
  - Fix --ipns-base alias ([ipfs/go-ipfs#7659](https://github.com/ipfs/go-ipfs/pull/7659))
  - docs: update config to indicate SECIO deprecation ([ipfs/go-ipfs#7630](https://github.com/ipfs/go-ipfs/pull/7630))
  - fix: ipfs dht put/get commands with peerIDs encoded as CIDs ([ipfs/go-ipfs#7633](https://github.com/ipfs/go-ipfs/pull/7633))
  - update version to 0.8.0-dev ([ipfs/go-ipfs#7629](https://github.com/ipfs/go-ipfs/pull/7629))
- github.com/ipfs/go-bitswap (v0.2.20 -> v0.3.3):
  - feat: configurable engine blockstore worker count (#449) ([ipfs/go-bitswap#449](https://github.com/ipfs/go-bitswap/pull/449))
  - fix: set the score ledger on start ([ipfs/go-bitswap#447](https://github.com/ipfs/go-bitswap/pull/447))
  - feat: update for go-libp2p-core 0.7.0 interface changes ([ipfs/go-bitswap#445](https://github.com/ipfs/go-bitswap/pull/445))
  - fix: guard access to the mock wiretap with a lock ([ipfs/go-bitswap#446](https://github.com/ipfs/go-bitswap/pull/446))
  - Add WireTap interface (#444) ([ipfs/go-bitswap#444](https://github.com/ipfs/go-bitswap/pull/444))
  - Fix: Increment stats.MessagesSent in msgToStream() function (#441) ([ipfs/go-bitswap#441](https://github.com/ipfs/go-bitswap/pull/441))
  - refactor: remove extraneous ledger field init (#437) ([ipfs/go-bitswap#437](https://github.com/ipfs/go-bitswap/pull/437))
  - Added `WithScoreLedger` Bitswap option  (#430) ([ipfs/go-bitswap#430](https://github.com/ipfs/go-bitswap/pull/430))
- github.com/ipfs/go-blockservice (v0.1.3 -> v0.1.4):
  - Avoid modifying passed in slice of cids ([ipfs/go-blockservice#65](https://github.com/ipfs/go-blockservice/pull/65))
- github.com/ipfs/go-ds-badger (v0.2.4 -> v0.2.6):
  - Log error if batch not committed or canceled ([ipfs/go-ds-badger#108](https://github.com/ipfs/go-ds-badger/pull/108))
  -  Add Cancel function; add finalizer to cleanup abandoned batch ([ipfs/go-ds-badger#105](https://github.com/ipfs/go-ds-badger/pull/105))
  - Do not implement batches using transactions ([ipfs/go-ds-badger#104](https://github.com/ipfs/go-ds-badger/pull/104))
  - readme: add information on Badger2 datastore ([ipfs/go-ds-badger#102](https://github.com/ipfs/go-ds-badger/pull/102))
  - update contributing link ([ipfs/go-ds-badger#91](https://github.com/ipfs/go-ds-badger/pull/91))
  - Use current go-log (#89) ([ipfs/go-ds-badger#89](https://github.com/ipfs/go-ds-badger/pull/89))
- github.com/ipfs/go-graphsync (v0.1.1 -> v0.6.0):
  - docs(CHANGELOG): revise for 0.6.0
  - Merge branch 'master' into release/v0.6.0
  - docs(CHANGELOG): update for 0.6.0 release
  - move block allocation into message queue (#140) ([ipfs/go-graphsync#140](https://github.com/ipfs/go-graphsync/pull/140))
  - Response Assembler Refactor (#138) ([ipfs/go-graphsync#138](https://github.com/ipfs/go-graphsync/pull/138))
  - Add error listener on receiver (#136) ([ipfs/go-graphsync#136](https://github.com/ipfs/go-graphsync/pull/136))
  - Run testplan on in CI (#137) ([ipfs/go-graphsync#137](https://github.com/ipfs/go-graphsync/pull/137))
  - fix(responsemanager): fix network error propogation (#133) ([ipfs/go-graphsync#133](https://github.com/ipfs/go-graphsync/pull/133))
  - testground test for graphsync (#132) ([ipfs/go-graphsync#132](https://github.com/ipfs/go-graphsync/pull/132))
  - docs(CHANGELOG): update for v0.5.2 ([ipfs/go-graphsync#130](https://github.com/ipfs/go-graphsync/pull/130))
  - RegisterNetworkErrorListener should fire when there's an error connecting to the peer (#127) ([ipfs/go-graphsync#127](https://github.com/ipfs/go-graphsync/pull/127))
  - Permit multiple data subscriptions per original topic (#128) ([ipfs/go-graphsync#128](https://github.com/ipfs/go-graphsync/pull/128))
  - release: v0.5.1 (#123) ([ipfs/go-graphsync#123](https://github.com/ipfs/go-graphsync/pull/123))
  - feat(responsemanager): allow configuration of max requests (#122) ([ipfs/go-graphsync#122](https://github.com/ipfs/go-graphsync/pull/122))
  - docs(CHANGELOG): update for 0.5.0 ([ipfs/go-graphsync#120](https://github.com/ipfs/go-graphsync/pull/120))
  - feat: use go-libp2p-core 0.7.0 stream interfaces (#116) ([ipfs/go-graphsync#116](https://github.com/ipfs/go-graphsync/pull/116))
  - Merge branch 'release/v0.4.3'
  - chore(benchmarks): remove extra files
  - fix(peerresponsemanager): avoid race condition that could result in NPE in link tracker (#118) ([ipfs/go-graphsync#118](https://github.com/ipfs/go-graphsync/pull/118))
  - docs(CHANGELOG): update for 0.4.2 ([ipfs/go-graphsync#117](https://github.com/ipfs/go-graphsync/pull/117))
  - feat(memory): improve memory usage (#110) ([ipfs/go-graphsync#110](https://github.com/ipfs/go-graphsync/pull/110))
  - fix(notifications): fix lock in close (#115) ([ipfs/go-graphsync#115](https://github.com/ipfs/go-graphsync/pull/115))
  - docs(CHANGELOG): update for v0.4.1 ([ipfs/go-graphsync#114](https://github.com/ipfs/go-graphsync/pull/114))
  - fix(allocator): remove peer from peer status list
  - docs(CHANGELOG): update for v0.4.0
  - docs(CHANGELOG): update for 0.3.1 ([ipfs/go-graphsync#112](https://github.com/ipfs/go-graphsync/pull/112))
  - Add allocator for memory backpressure (#108) ([ipfs/go-graphsync#108](https://github.com/ipfs/go-graphsync/pull/108))
  - Shutdown notifications go routines (#109) ([ipfs/go-graphsync#109](https://github.com/ipfs/go-graphsync/pull/109))
  - Switch to google protobuf generator (#105) ([ipfs/go-graphsync#105](https://github.com/ipfs/go-graphsync/pull/105))
  - feat(CHANGELOG): update for 0.3.0 ([ipfs/go-graphsync#104](https://github.com/ipfs/go-graphsync/pull/104))
  - docs(CHANGELOG): update for 0.2.1 ([ipfs/go-graphsync#103](https://github.com/ipfs/go-graphsync/pull/103))
  - Track actual network operations in a response (#102) ([ipfs/go-graphsync#102](https://github.com/ipfs/go-graphsync/pull/102))
  - feat(responsecache): prune blocks more intelligently (#101) ([ipfs/go-graphsync#101](https://github.com/ipfs/go-graphsync/pull/101))
  - Release/0.2.0 ([ipfs/go-graphsync#99](https://github.com/ipfs/go-graphsync/pull/99))
  - fix(metadata): fix cbor-gen (#98) ([ipfs/go-graphsync#98](https://github.com/ipfs/go-graphsync/pull/98))
  - fix(selectorvalidator): memory optimization (#97) ([ipfs/go-graphsync#97](https://github.com/ipfs/go-graphsync/pull/97))
  - Update go-ipld-prime@v0.5.0 (#92) ([ipfs/go-graphsync#92](https://github.com/ipfs/go-graphsync/pull/92))
  - refactor(metadata): use cbor-gen encoding (#96) ([ipfs/go-graphsync#96](https://github.com/ipfs/go-graphsync/pull/96))
  - Release/v0.1.2 ([ipfs/go-graphsync#95](https://github.com/ipfs/go-graphsync/pull/95))
  - Return Request context cancelled error (#93) ([ipfs/go-graphsync#93](https://github.com/ipfs/go-graphsync/pull/93))
  - feat(benchmarks): add p2p stress test (#91) ([ipfs/go-graphsync#91](https://github.com/ipfs/go-graphsync/pull/91))
  - Benchmark framework + First memory fixes (#89) ([ipfs/go-graphsync#89](https://github.com/ipfs/go-graphsync/pull/89))
  - docs(CHANGELOG): update for v0.1.1 ([ipfs/go-graphsync#85](https://github.com/ipfs/go-graphsync/pull/85))
- github.com/ipfs/go-ipfs-cmds (v0.4.0 -> v0.6.0):
  - Added DelimitedStringsOption for enabling delimited strings on the CLI ([ipfs/go-ipfs-cmds#204](https://github.com/ipfs/go-ipfs-cmds/pull/204))
  - feat: support strings option over HTTP API ([ipfs/go-ipfs-cmds#203](https://github.com/ipfs/go-ipfs-cmds/pull/203))
- github.com/ipfs/go-ipfs-config (v0.9.0 -> v0.12.0):
  - add support for pinning mfs (#116) ([ipfs/go-ipfs-config#116](https://github.com/ipfs/go-ipfs-config/pull/116))
  - add remote pinning services config ([ipfs/go-ipfs-config#113](https://github.com/ipfs/go-ipfs-config/pull/113))
  - Remove badger2 profile ([ipfs/go-ipfs-config#115](https://github.com/ipfs/go-ipfs-config/pull/115))
  - Add badger2 profile and config spec
- github.com/ipfs/go-ipfs-pinner (v0.0.4 -> v0.1.1):
  - Avoid loading all pins into memory during migration (#5) ([ipfs/go-ipfs-pinner#5](https://github.com/ipfs/go-ipfs-pinner/pull/5))
  - Datastore based pinner (#4) ([ipfs/go-ipfs-pinner#4](https://github.com/ipfs/go-ipfs-pinner/pull/4))
- github.com/ipfs/go-ipld-cbor (v0.0.4 -> v0.0.5):
  - add the ability to leverage zero-copy on blockstores. (#75) ([ipfs/go-ipld-cbor#75](https://github.com/ipfs/go-ipld-cbor/pull/75))
  - ipldstore: Also wrap Put serialization errors ([ipfs/go-ipld-cbor#74](https://github.com/ipfs/go-ipld-cbor/pull/74))
  - add helper constructor for inmem cbor store
  - docs: add comments describing methods & interfaces ([ipfs/go-ipld-cbor#71](https://github.com/ipfs/go-ipld-cbor/pull/71))
- github.com/ipfs/go-path (v0.0.8 -> v0.0.9):
  - fix: improved error message on broken CIDv0 ([ipfs/go-path#33](https://github.com/ipfs/go-path/pull/33))
- github.com/ipfs/go-pinning-service-http-client (null -> v0.1.0):
  - feat: LsBatchSync to fetch single batch of results ([ipfs/go-pinning-service-http-client#6](https://github.com/ipfs/go-pinning-service-http-client/pull/6))
  - Initial Implementation ([ipfs/go-pinning-service-http-client#1](https://github.com/ipfs/go-pinning-service-http-client/pull/1))
- github.com/ipld/go-car (v0.1.1-0.20200429200904-c222d793c339 -> v0.1.1-0.20201015032735-ff6ccdc46acc):
  - Update ipld libs ([ipld/go-car#35](https://github.com/ipld/go-car/pull/35))
- github.com/ipld/go-ipld-prime (v0.0.2-0.20200428162820-8b59dc292b8e -> v0.5.1-0.20201021195245-109253e8a018):
  - Merge branch 'codec-hardening'
  - Add fluent.MustReflect convenience method.
  - codegen: make error info available when tuples process data that is too long. ([ipld/go-ipld-prime#99](https://github.com/ipld/go-ipld-prime/pull/99))
  - Merge branch 'codegen-typofixes'
  - Implement resource budgets in dagcbor parsing. ([ipld/go-ipld-prime#85](https://github.com/ipld/go-ipld-prime/pull/85))
  - Codegen for links should emit the methods to conform to the schema.TypedLinkNode interface where applicable. ([ipld/go-ipld-prime#91](https://github.com/ipld/go-ipld-prime/pull/91))
  - Introduce fluent.Reflect convenience functions. ([ipld/go-ipld-prime#81](https://github.com/ipld/go-ipld-prime/pull/81))
  - schema/gen/go: make all top-level tests parallel
  - all: don't use buffers where readers suffice
  - fix typo in documentation
  - schema-schema codegen demo now includes unmarshal exercise ([ipld/go-ipld-prime#76](https://github.com/ipld/go-ipld-prime/pull/76))
  - Update tests for unions; several fixes ([ipld/go-ipld-prime#75](https://github.com/ipld/go-ipld-prime/pull/75))
  - New testcase system for exercising typed nodes; Revamp struct tests to use it. ([ipld/go-ipld-prime#66](https://github.com/ipld/go-ipld-prime/pull/66))
  - small docs fixes on an internal component.
  - Fix formatting in README.
  - fix(cidlink): check for byte buffer ([ipld/go-ipld-prime#70](https://github.com/ipld/go-ipld-prime/pull/70))
  - linking/cid: check a previously unused error ([ipld/go-ipld-prime#68](https://github.com/ipld/go-ipld-prime/pull/68))
  - all: make 'go test ./...' pass on Go 1.15 ([ipld/go-ipld-prime#67](https://github.com/ipld/go-ipld-prime/pull/67))
  - Merge branch 'kinded-union-gen'
  - Add traversal.Get function ([ipld/go-ipld-prime#65](https://github.com/ipld/go-ipld-prime/pull/65))
  - Kinded union gen ([ipld/go-ipld-prime#64](https://github.com/ipld/go-ipld-prime/pull/64))
  - Struct tuple representation codegen ([ipld/go-ipld-prime#63](https://github.com/ipld/go-ipld-prime/pull/63))
  - Merge branch 'moar-codegen'
  - Self-hosting gen of the schema-schema. ([ipld/go-ipld-prime#62](https://github.com/ipld/go-ipld-prime/pull/62))
  - Codegen: approaching self-host ([ipld/go-ipld-prime#61](https://github.com/ipld/go-ipld-prime/pull/61))
  - Codegen of unions, and their keyed representations ([ipld/go-ipld-prime#60](https://github.com/ipld/go-ipld-prime/pull/60))
  - mark v0.5
  - API updates for v0.5: the renamening ([ipld/go-ipld-prime#59](https://github.com/ipld/go-ipld-prime/pull/59))
  - mark v0.4
  - changelog: note the codegen work.
  - Codegen update -- Assemblers, and many new representations ([ipld/go-ipld-prime#52](https://github.com/ipld/go-ipld-prime/pull/52))
  - Merge branch 'json-tables-codec'
  - Merge branch 'docs-updates'
  - Introduce changelog!
  - Add examples of creating and loading links.
- github.com/ipld/go-ipld-prime-proto (v0.0.0-20200428191222-c1ffdadc01e1 -> v0.1.0):
  - Update go-ipld-prime ([ipld/go-ipld-prime-proto#6](https://github.com/ipld/go-ipld-prime-proto/pull/6))
  - feat(coding use -1 instead of 0):
  - Update ipld prime, use proper code-gen ([ipld/go-ipld-prime-proto#5](https://github.com/ipld/go-ipld-prime-proto/pull/5))
  - Updates to dependencies ([ipld/go-ipld-prime-proto#4](https://github.com/ipld/go-ipld-prime-proto/pull/4))
  - Check for byte buffer on decode ([ipld/go-ipld-prime-proto#3](https://github.com/ipld/go-ipld-prime-proto/pull/3))
- github.com/libp2p/go-libp2p (v0.11.0 -> v0.13.0):
  - use a context when opening streams ([libp2p/go-libp2p#1033](https://github.com/libp2p/go-libp2p/pull/1033))
  - fix: obey new stream timeout ([libp2p/go-libp2p#1029](https://github.com/libp2p/go-libp2p/pull/1029))
  - feat: update to go-libp2p-core 0.7.0 interface changes ([libp2p/go-libp2p#1001](https://github.com/libp2p/go-libp2p/pull/1001))
  - Basic Connection Gater Implementation ([libp2p/go-libp2p#1005](https://github.com/libp2p/go-libp2p/pull/1005))
  - Fixed bug for inbound connections gated by the deprecated filter option (#1004) ([libp2p/go-libp2p#1004](https://github.com/libp2p/go-libp2p/pull/1004))
- github.com/libp2p/go-libp2p-autonat (v0.3.2 -> v0.4.0):
  - feat: update to go-libp2p-core 0.7.0 ([libp2p/go-libp2p-autonat#97](https://github.com/libp2p/go-libp2p-autonat/pull/97))
- github.com/libp2p/go-libp2p-circuit (v0.3.1 -> v0.4.0):
  - feat: update to go-libp2p-core 0.7.0 ([libp2p/go-libp2p-circuit#123](https://github.com/libp2p/go-libp2p-circuit/pull/123))
- github.com/libp2p/go-libp2p-core (v0.6.1 -> v0.8.0):
  - add a context to OpenStream and NewStream (#172) ([libp2p/go-libp2p-core#172](https://github.com/libp2p/go-libp2p-core/pull/172))
  - sec/insecure/insecure.go: Fix typo (#167) ([libp2p/go-libp2p-core#167](https://github.com/libp2p/go-libp2p-core/pull/167))
  - add CloseRead/CloseWrite on streams (#166) ([libp2p/go-libp2p-core#166](https://github.com/libp2p/go-libp2p-core/pull/166))
  - Fix typo in docs (#163) ([libp2p/go-libp2p-core#163](https://github.com/libp2p/go-libp2p-core/pull/163))
- github.com/libp2p/go-libp2p-gostream (v0.2.1 -> v0.3.0):
  - feat: use go-libp2p-core 0.7.0 stream interfaces ([libp2p/go-libp2p-gostream#60](https://github.com/libp2p/go-libp2p-gostream/pull/60))
- github.com/libp2p/go-libp2p-http (v0.1.5 -> v0.2.0):
  - Fix var name in README ([libp2p/go-libp2p-http#63](https://github.com/libp2p/go-libp2p-http/pull/63))
  - Fix var name in doc ([libp2p/go-libp2p-http#62](https://github.com/libp2p/go-libp2p-http/pull/62))
- github.com/libp2p/go-libp2p-kad-dht (v0.9.0 -> v0.11.1):
  - Fix constructor ordering ([libp2p/go-libp2p-kad-dht#698](https://github.com/libp2p/go-libp2p-kad-dht/pull/698))
  - feat: update to go-libp2p-core 0.7.0 ([libp2p/go-libp2p-kad-dht#693](https://github.com/libp2p/go-libp2p-kad-dht/pull/693))
  - Run fixLowPeers on startup ([libp2p/go-libp2p-kad-dht#694](https://github.com/libp2p/go-libp2p-kad-dht/pull/694))
  - feat: add advanced V1ProtocolOverride option to be used by legacy networks
  - feat: remove dht v2 as it's not actually in use and could be confusing
- github.com/libp2p/go-libp2p-mplex (v0.2.4 -> v0.4.1):
  - update go-mplex, use the context passed to OpenStream ([libp2p/go-libp2p-mplex#23](https://github.com/libp2p/go-libp2p-mplex/pull/23))
  - change OpenStream to accept a context ([libp2p/go-libp2p-mplex#21](https://github.com/libp2p/go-libp2p-mplex/pull/21))
  - feat: update stream interfaces ([libp2p/go-libp2p-mplex#20](https://github.com/libp2p/go-libp2p-mplex/pull/20))
- github.com/libp2p/go-libp2p-noise (v0.1.1 -> v0.1.2):
  - optimize: reduce syscalls using a buffered reader.
- github.com/libp2p/go-libp2p-pubsub (v0.3.5 -> v0.4.1):
  - defer stream removal instead of doing it inline.
  - add test for inbound stream deduplication
  - deduplicate inbound streams
  - populate receivedFrom field in delivery trace
  - add receivedFrom field in delivery trace
  - fix: reduce log spam (#394) ([libp2p/go-libp2p-pubsub#394](https://github.com/libp2p/go-libp2p-pubsub/pull/394))
  - fix: treat peers already connected to the host before pubsub is initialized as valid potential pubsub peers
  - test: add test for if nodes are connected before pubsub is started
  - feat: update to go-libp2p-core 0.7.0
  - Add go-libp2p example in README.md (#392) ([libp2p/go-libp2p-pubsub#392](https://github.com/libp2p/go-libp2p-pubsub/pull/392))
  - subscription filters
  - remove multi-topic message support
  - satisfy race detector
  - clean up
  - copy string topic
  - add test for score adjustment from topis params reset
  - prettify things
  - add test for topic score parameter reset method
  - add test for topic score parameter reset
  - add api for dynamically setting and resetting topic score parameters
  - add support for priority topic delivery weights
  - tweak duplicate/reject weights
  - decay global counters after 2 min
  - decouple global coutner decay from source counter decay
  - add warning for failure to parse IP out of remote multiaddr
  - more docs
  - configure the peer gater using a parameter object, docs and stuff
  - disable codecov annotations, makes things unreadable
  - further tweak gate threshold weights
  - fix test races
  - use IPs for peer gater stat tracking
  - mix total accounting components with different weights
  - count all rejections by default
  - fix non-determinism in test
  - tweak probability threshold
  - also account for duplicates in gating decisions
  - test throttle code path in gossip tracer
  - add test for peer gater
  - more efficient promise processing on throttling
  - trace throttle peers to avoid breaking promises unfairly
  - better log messages around gating
  - implement peer gater
  - peer gater scaffolding
  - rich router acceptance semantics
  - reduce log verbosity; debug mostly
- github.com/libp2p/go-libp2p-pubsub-router (v0.3.2 -> v0.4.0):
  - feat: use new stream interfaces from go-libp2p-core 0.7.0 ([libp2p/go-libp2p-pubsub-router#81](https://github.com/libp2p/go-libp2p-pubsub-router/pull/81))
- github.com/libp2p/go-libp2p-quic-transport (v0.8.0 -> v0.10.0):
  - change OpenStream to accept a context ([libp2p/go-libp2p-quic-transport#189](https://github.com/libp2p/go-libp2p-quic-transport/pull/189))
  - update quic-go to v0.19.1 ([libp2p/go-libp2p-quic-transport#182](https://github.com/libp2p/go-libp2p-quic-transport/pull/182))
  - pass a conn that can be type asserted to a net.UDPConn to quic-go ([libp2p/go-libp2p-quic-transport#180](https://github.com/libp2p/go-libp2p-quic-transport/pull/180))
  - add more integration tests ([libp2p/go-libp2p-quic-transport#181](https://github.com/libp2p/go-libp2p-quic-transport/pull/181))
  - always close the connection in the cmd client ([libp2p/go-libp2p-quic-transport#175](https://github.com/libp2p/go-libp2p-quic-transport/pull/175))
  - use GitHub Actions to test interopability of releases ([libp2p/go-libp2p-quic-transport#173](https://github.com/libp2p/go-libp2p-quic-transport/pull/173))
  - Implement CloseRead/CloseWrite ([libp2p/go-libp2p-quic-transport#174](https://github.com/libp2p/go-libp2p-quic-transport/pull/174))
  - enable quic-go metrics collection ([libp2p/go-libp2p-quic-transport#172](https://github.com/libp2p/go-libp2p-quic-transport/pull/172))
- github.com/libp2p/go-libp2p-swarm (v0.2.8 -> v0.4.0):
  - use a context for OpenStream and NewStream ([libp2p/go-libp2p-swarm#232](https://github.com/libp2p/go-libp2p-swarm/pull/232))
  - fix: handle case where swarm closes before stream ([libp2p/go-libp2p-swarm#229](https://github.com/libp2p/go-libp2p-swarm/pull/229))
  - feat: update to latest go-libp2p-core interfaces ([libp2p/go-libp2p-swarm#228](https://github.com/libp2p/go-libp2p-swarm/pull/228))
- github.com/libp2p/go-libp2p-testing (v0.2.0 -> v0.4.0):
  - pass contexts to OpenStream in tests ([libp2p/go-libp2p-testing#31](https://github.com/libp2p/go-libp2p-testing/pull/31))
  - chore: Adding LICENSE. ([libp2p/go-libp2p-testing#30](https://github.com/libp2p/go-libp2p-testing/pull/30))
  - feat: update to go-libp2p-core 0.7.0 ([libp2p/go-libp2p-testing#29](https://github.com/libp2p/go-libp2p-testing/pull/29))
- github.com/libp2p/go-libp2p-transport-upgrader (v0.3.0 -> v0.4.0):
  - pass contexts to OpenStream in tests ([libp2p/go-libp2p-transport-upgrader#70](https://github.com/libp2p/go-libp2p-transport-upgrader/pull/70))
  - fix int to string conversion in tests, update Go version on CI ([libp2p/go-libp2p-transport-upgrader#69](https://github.com/libp2p/go-libp2p-transport-upgrader/pull/69))
- github.com/libp2p/go-libp2p-yamux (v0.2.8 -> v0.5.1):
  - update go-yamux to v2.0.0, use context passed to OpenStream ([libp2p/go-libp2p-yamux#31](https://github.com/libp2p/go-libp2p-yamux/pull/31))
  - change OpenStream to accept a context ([libp2p/go-libp2p-yamux#29](https://github.com/libp2p/go-libp2p-yamux/pull/29))
  - feat: update to new stream interfaces ([libp2p/go-libp2p-yamux#27](https://github.com/libp2p/go-libp2p-yamux/pull/27))
- github.com/libp2p/go-mplex (v0.1.2 -> v0.3.0):
  - add a context to NewStream, remove the NewStreamTimeout ([libp2p/go-mplex#82](https://github.com/libp2p/go-mplex/pull/82))
  - Implement new CloseWrite/CloseRead interface ([libp2p/go-mplex#81](https://github.com/libp2p/go-mplex/pull/81))
  - Bump lodash from 4.17.15 to 4.17.19 in /interop/js ([libp2p/go-mplex#79](https://github.com/libp2p/go-mplex/pull/79))
  - upgrade deps + interoperable varints. (#80) ([libp2p/go-mplex#80](https://github.com/libp2p/go-mplex/pull/80))
  - write benchmarks (#77) ([libp2p/go-mplex#77](https://github.com/libp2p/go-mplex/pull/77))
- github.com/libp2p/go-ws-transport (v0.3.1 -> v0.4.0):
  - pass a context to OpenStream in tests ([libp2p/go-ws-transport#98](https://github.com/libp2p/go-ws-transport/pull/98))
  - Dependency: Remove deprecated multiaddr-net ([libp2p/go-ws-transport#97](https://github.com/libp2p/go-ws-transport/pull/97))
  - Update for go 1.14 Wasm changes ([libp2p/go-ws-transport#96](https://github.com/libp2p/go-ws-transport/pull/96))
- github.com/libp2p/go-yamux (v1.3.7 -> v1.4.1):
  - feat: improve ping accuracy ([libp2p/go-yamux#35](https://github.com/libp2p/go-yamux/pull/35))
  - implement CloseRead/CloseWrite ([libp2p/go-yamux#5](https://github.com/libp2p/go-yamux/pull/5))
  - fix space accounting in the receive buffer ([libp2p/go-yamux#33](https://github.com/libp2p/go-yamux/pull/33))
  - Limit pings ([libp2p/go-yamux#32](https://github.com/libp2p/go-yamux/pull/32))
  - fix: simplify inflight fix ([libp2p/go-yamux#31](https://github.com/libp2p/go-yamux/pull/31))
  - Clearing inflight along with streams to avoid memory leak ([libp2p/go-yamux#30](https://github.com/libp2p/go-yamux/pull/30))
- github.com/lucas-clemente/quic-go (v0.18.0 -> v0.19.3):
  - create a v0.19.x release
  - improve the warning about the UDP receive buffer size ([lucas-clemente/quic-go#2923](https://github.com/lucas-clemente/quic-go/pull/2923))
  - immediately remove reset tokens when retiring a connection ID ([lucas-clemente/quic-go#2897](https://github.com/lucas-clemente/quic-go/pull/2897))
  - add common temporary file patterns to .gitignore ([lucas-clemente/quic-go#2917](https://github.com/lucas-clemente/quic-go/pull/2917))
  - disable key updates when using HTTP/3 to avoid breaking Chrome 87 ([lucas-clemente/quic-go#2906](https://github.com/lucas-clemente/quic-go/pull/2906))
  - fix decoding of packet numbers in different packet number spaces ([lucas-clemente/quic-go#2903](https://github.com/lucas-clemente/quic-go/pull/2903))
  - log sent packet before logging its congestion / loss recovery effects ([lucas-clemente/quic-go#2912](https://github.com/lucas-clemente/quic-go/pull/2912))
  - fix a crash in the http3.Server when GetConfigForClient returns nil ([lucas-clemente/quic-go#2925](https://github.com/lucas-clemente/quic-go/pull/2925))
  - set the UDP receive buffer size on Windows ([lucas-clemente/quic-go#2896](https://github.com/lucas-clemente/quic-go/pull/2896))
  - remove superfluous sleep in packet handler map test ([lucas-clemente/quic-go#2894](https://github.com/lucas-clemente/quic-go/pull/2894))
  - fix setting of http.Handler in the example server ([lucas-clemente/quic-go#2900](https://github.com/lucas-clemente/quic-go/pull/2900))
  - remove stray print statement
  - remove unnecessary mutex locking in the stream flow controller ([lucas-clemente/quic-go#2869](https://github.com/lucas-clemente/quic-go/pull/2869))
  - only use syscalls on platforms that we're actually testing ([lucas-clemente/quic-go#2886](https://github.com/lucas-clemente/quic-go/pull/2886))
  - only write headers with a length that fits into 2 bytes in fuzz test ([lucas-clemente/quic-go#2884](https://github.com/lucas-clemente/quic-go/pull/2884))
  - fix packing of 1-RTT probe packets ([lucas-clemente/quic-go#2882](https://github.com/lucas-clemente/quic-go/pull/2882))
  - use PADDING frames to pad packets ([lucas-clemente/quic-go#2876](https://github.com/lucas-clemente/quic-go/pull/2876))
  - fix race condition when accepting streams ([lucas-clemente/quic-go#2874](https://github.com/lucas-clemente/quic-go/pull/2874))
  - only trace dropped 0-RTT packets when a tracer is set ([lucas-clemente/quic-go#2871](https://github.com/lucas-clemente/quic-go/pull/2871))
  - use consistent version numbers in client test ([lucas-clemente/quic-go#2870](https://github.com/lucas-clemente/quic-go/pull/2870))
  - replace the RWMutex with a Mutex in the flow controller ([lucas-clemente/quic-go#2865](https://github.com/lucas-clemente/quic-go/pull/2865))
  - replace the RWMutex with a Mutex in the packet handler map ([lucas-clemente/quic-go#2864](https://github.com/lucas-clemente/quic-go/pull/2864))
  - wait until the handshake is complete before updating the connection ID ([lucas-clemente/quic-go#2856](https://github.com/lucas-clemente/quic-go/pull/2856))
  - only check the SCID for Initial packets ([lucas-clemente/quic-go#2857](https://github.com/lucas-clemente/quic-go/pull/2857))
  - add the NO_VIABLE_PATH error ([lucas-clemente/quic-go#2861](https://github.com/lucas-clemente/quic-go/pull/2861))
  - implement qlogging of the preferred address in the transport parameters ([lucas-clemente/quic-go#2853](https://github.com/lucas-clemente/quic-go/pull/2853))
  - explicitly set the supported versions in the HTTP/3 server test ([lucas-clemente/quic-go#2854](https://github.com/lucas-clemente/quic-go/pull/2854))
  - allow an amplification factor of 3.x ([lucas-clemente/quic-go#2862](https://github.com/lucas-clemente/quic-go/pull/2862))
  - only allow the HTTP/3 client to dial with a single QUIC version ([lucas-clemente/quic-go#2848](https://github.com/lucas-clemente/quic-go/pull/2848))
  - send STREAMS_BLOCKED frame when MAX_STREAMS frame allows too few streams  ([lucas-clemente/quic-go#2828](https://github.com/lucas-clemente/quic-go/pull/2828))
  - set the ALPN based on the QUIC version in the HTTP3 server ([lucas-clemente/quic-go#2847](https://github.com/lucas-clemente/quic-go/pull/2847))
  - pad datagrams containing ack-eliciting Initial packets sent by the server ([lucas-clemente/quic-go#2841](https://github.com/lucas-clemente/quic-go/pull/2841))
  - fix OpenStreamSync busy looping ([lucas-clemente/quic-go#2827](https://github.com/lucas-clemente/quic-go/pull/2827))
  - fix deadlock when closing the server and the connection at the same time ([lucas-clemente/quic-go#2849](https://github.com/lucas-clemente/quic-go/pull/2849))
  - run gofumpt, enable the gofumpt linter ([lucas-clemente/quic-go#2839](https://github.com/lucas-clemente/quic-go/pull/2839))
  - prepare for draft-32 ([lucas-clemente/quic-go#2831](https://github.com/lucas-clemente/quic-go/pull/2831))
  - update the invalid packet limit for AES ([lucas-clemente/quic-go#2825](https://github.com/lucas-clemente/quic-go/pull/2825))
  - increase UDP receive buffer size ([lucas-clemente/quic-go#2791](https://github.com/lucas-clemente/quic-go/pull/2791))
  - listen on both IPv4 and IPv6 in the interop runner server ([lucas-clemente/quic-go#2822](https://github.com/lucas-clemente/quic-go/pull/2822))
  - only send Version Negotiation packets for packets larger than 1200 bytes ([lucas-clemente/quic-go#2820](https://github.com/lucas-clemente/quic-go/pull/2820))
  - don't send a version negotiation packet in response to a version negotiation packet ([lucas-clemente/quic-go#2818](https://github.com/lucas-clemente/quic-go/pull/2818))
  - client: Add DialEarlyContext and DialAddrEarlyContext API ([lucas-clemente/quic-go#2814](https://github.com/lucas-clemente/quic-go/pull/2814))
  - qlog the key phase bit ([lucas-clemente/quic-go#2817](https://github.com/lucas-clemente/quic-go/pull/2817))
  - only include quic-trace when the quictrace build flag is set ([lucas-clemente/quic-go#2799](https://github.com/lucas-clemente/quic-go/pull/2799))
  - fix error handling when receiving post handshake messages ([lucas-clemente/quic-go#2807](https://github.com/lucas-clemente/quic-go/pull/2807))
  - add support for the ChaCha20 test on the server side ([lucas-clemente/quic-go#2816](https://github.com/lucas-clemente/quic-go/pull/2816))
  - allow the first key update immediately after handshake confirmation ([lucas-clemente/quic-go#2811](https://github.com/lucas-clemente/quic-go/pull/2811))
  - ignore temporary errors when reading from the packet conn ([lucas-clemente/quic-go#2806](https://github.com/lucas-clemente/quic-go/pull/2806))
  - fix linting error on OSX ([lucas-clemente/quic-go#2813](https://github.com/lucas-clemente/quic-go/pull/2813))
  - add the exhaustive linter, replace panics by return values in logging stringers ([lucas-clemente/quic-go#2729](https://github.com/lucas-clemente/quic-go/pull/2729))
  - include the error code in the string for CRYPTO_ERRORs ([lucas-clemente/quic-go#2805](https://github.com/lucas-clemente/quic-go/pull/2805))
  - fail the handshake if the quic_transport_parameter extension is missing ([lucas-clemente/quic-go#2804](https://github.com/lucas-clemente/quic-go/pull/2804))
  - fix logging of received Retry packets ([lucas-clemente/quic-go#2803](https://github.com/lucas-clemente/quic-go/pull/2803))
  - fix deadlock in crypto setup when it is closed while handling a message ([lucas-clemente/quic-go#2802](https://github.com/lucas-clemente/quic-go/pull/2802))
  - make the key update integration test more rigorous ([lucas-clemente/quic-go#2760](https://github.com/lucas-clemente/quic-go/pull/2760))
  - add support for the new keyupdate interop runner test case ([lucas-clemente/quic-go#2782](https://github.com/lucas-clemente/quic-go/pull/2782))
  - remove unneeded mutex in the client ([lucas-clemente/quic-go#2798](https://github.com/lucas-clemente/quic-go/pull/2798))
  - correctly handle key updates within the 3 PTO period ([lucas-clemente/quic-go#2787](https://github.com/lucas-clemente/quic-go/pull/2787))
  - introduce an ECNCapablePacketConn interface to determine ECN support ([lucas-clemente/quic-go#2788](https://github.com/lucas-clemente/quic-go/pull/2788))
  - use certificates from /certs directory for the server ([lucas-clemente/quic-go#2794](https://github.com/lucas-clemente/quic-go/pull/2794))
  - remove support for the ECN test case ([lucas-clemente/quic-go#2793](https://github.com/lucas-clemente/quic-go/pull/2793))
  - check that the peer updated its keys when acknowledging a key update ([lucas-clemente/quic-go#2781](https://github.com/lucas-clemente/quic-go/pull/2781))
  - fix flaky packet number skipping test ([lucas-clemente/quic-go#2786](https://github.com/lucas-clemente/quic-go/pull/2786))
  - read ECN bits and send ECN counters in ACK frames ([lucas-clemente/quic-go#2741](https://github.com/lucas-clemente/quic-go/pull/2741))
  - implement the limit of unsuccessful decryptions for the AEADs ([lucas-clemente/quic-go#2771](https://github.com/lucas-clemente/quic-go/pull/2771))
  - use the KEY_UPDATE_ERROR ([lucas-clemente/quic-go#2770](https://github.com/lucas-clemente/quic-go/pull/2770))
  - fix dropping of key phase 0 ([lucas-clemente/quic-go#2769](https://github.com/lucas-clemente/quic-go/pull/2769))
  - reduce the handshake timeout to two minutes in the handshake drop tests ([lucas-clemente/quic-go#2768](https://github.com/lucas-clemente/quic-go/pull/2768))
  - fix handling of multiple handshake messages in the case of errors ([lucas-clemente/quic-go#2777](https://github.com/lucas-clemente/quic-go/pull/2777))
  - enable more linters, update golangci-lint to v1.31 ([lucas-clemente/quic-go#2775](https://github.com/lucas-clemente/quic-go/pull/2775))
  - increase the threshold for the receive stream deadline test ([lucas-clemente/quic-go#2774](https://github.com/lucas-clemente/quic-go/pull/2774))
  - add an assertion that bytes_in_flight never becomes negative ([lucas-clemente/quic-go#2779](https://github.com/lucas-clemente/quic-go/pull/2779))
  - fix race condition in handshake fuzz code ([lucas-clemente/quic-go#2778](https://github.com/lucas-clemente/quic-go/pull/2778))
  - use more tls.Config options in the handshake fuzzer ([lucas-clemente/quic-go#2746](https://github.com/lucas-clemente/quic-go/pull/2746))
  - run two handshakes in the handshake fuzzer ([lucas-clemente/quic-go#2743](https://github.com/lucas-clemente/quic-go/pull/2743))
  - send post-handshake message in the handshake fuzzer ([lucas-clemente/quic-go#2742](https://github.com/lucas-clemente/quic-go/pull/2742))
  - skip a packet number when sending a 1-RTT PTO packet ([lucas-clemente/quic-go#2754](https://github.com/lucas-clemente/quic-go/pull/2754))
  - save dummy packets in the packet history when skipping packet numbers ([lucas-clemente/quic-go#2753](https://github.com/lucas-clemente/quic-go/pull/2753))
  - delete unacknowledged packets from the packet history after 3 PTOs ([lucas-clemente/quic-go#2750](https://github.com/lucas-clemente/quic-go/pull/2750))
  - add support for the HTTP CONNECT method (#2761) ([lucas-clemente/quic-go#2761](https://github.com/lucas-clemente/quic-go/pull/2761))
  - don't drop keys for key phase N before receiving a N+1-protected packet ([lucas-clemente/quic-go#2762](https://github.com/lucas-clemente/quic-go/pull/2762))
  - close session on errors unpacking errors other than decryption errors ([lucas-clemente/quic-go#2756](https://github.com/lucas-clemente/quic-go/pull/2756))
  - log when an old 1-RTT key is retired ([lucas-clemente/quic-go#2765](https://github.com/lucas-clemente/quic-go/pull/2765))
  - only return an invalid first key phase error for decryptable packets ([lucas-clemente/quic-go#2757](https://github.com/lucas-clemente/quic-go/pull/2757))
  - fix logging of locally initiated key updates ([lucas-clemente/quic-go#2764](https://github.com/lucas-clemente/quic-go/pull/2764))
  - test that both endpoints time out in the timeout integration test ([lucas-clemente/quic-go#2744](https://github.com/lucas-clemente/quic-go/pull/2744))
  - refactor RTT measurements to simplify the sentPacketHistory ([lucas-clemente/quic-go#2747](https://github.com/lucas-clemente/quic-go/pull/2747))
  - fix dropping of 0-RTT packets ([lucas-clemente/quic-go#2752](https://github.com/lucas-clemente/quic-go/pull/2752))
  - always qlog the generation of 1-RTT key updates ([lucas-clemente/quic-go#2763](https://github.com/lucas-clemente/quic-go/pull/2763))
  - move the PacketHeader struct from logging to qlog package ([lucas-clemente/quic-go#2766](https://github.com/lucas-clemente/quic-go/pull/2766))
  - use a uint8 for the EncryptionLevel ([lucas-clemente/quic-go#2751](https://github.com/lucas-clemente/quic-go/pull/2751))
  - make sure to only pass handshake messages that keys are available for ([lucas-clemente/quic-go#2739](https://github.com/lucas-clemente/quic-go/pull/2739))
  - only close the handshake fuzz runner once ([lucas-clemente/quic-go#2740](https://github.com/lucas-clemente/quic-go/pull/2740))
  - generate a self-signed certificate for the handshake fuzzer ([lucas-clemente/quic-go#2738](https://github.com/lucas-clemente/quic-go/pull/2738))
  - use the os.ErrDeadlineExceeded for stream deadline errors on Go 1.15 ([lucas-clemente/quic-go#2734](https://github.com/lucas-clemente/quic-go/pull/2734))
  - use GitHub Actions to run unit tests ([lucas-clemente/quic-go#2732](https://github.com/lucas-clemente/quic-go/pull/2732))
  - add a basic fuzzer for the handshake ([lucas-clemente/quic-go#2733](https://github.com/lucas-clemente/quic-go/pull/2733))
  - export seed corpus files using the SHA1 of the content as the filename ([lucas-clemente/quic-go#2731](https://github.com/lucas-clemente/quic-go/pull/2731))
  - add a fuzz target for the token generator ([lucas-clemente/quic-go#2730](https://github.com/lucas-clemente/quic-go/pull/2730))
  - fix typo in error message in sent packet handler
  - fix missing OnLost callback for frames sent in 0-RTT packets ([lucas-clemente/quic-go#2728](https://github.com/lucas-clemente/quic-go/pull/2728))
  - fix overflow of the max_ack_delay when parsing transport parameters ([lucas-clemente/quic-go#2725](https://github.com/lucas-clemente/quic-go/pull/2725))
- github.com/marten-seemann/qpack (v0.2.0 -> v0.2.1):
  - run gofumpt, add a few more linters ([marten-seemann/qpack#21](https://github.com/marten-seemann/qpack/pull/21))
  - fix static table entry 80 ([marten-seemann/qpack#20](https://github.com/marten-seemann/qpack/pull/20))
- github.com/marten-seemann/qtls-go1-15 (v0.1.0 -> v0.1.1):
  - use a prefix for client session cache keys
  - add callbacks to store and restore app data along a session state
  - don't use TLS 1.3 compatibility mode when using alternative record layer
  - delete the session ticket after attempting 0-RTT
  - reject 0-RTT when a different ALPN is chosen
  - encode the ALPN into the session ticket
  - add a field to the ConnectionState to tell if 0-RTT was used
  - add a callback to tell the client about rejection of 0-RTT
  - don't offer 0-RTT after a HelloRetryRequest
  - add Accept0RTT to Config callback to decide if 0-RTT should be accepted
  - add the option to encode application data into the session ticket
  - export the 0-RTT write key
  - abuse the nonce field of ClientSessionState to save max_early_data_size
  - export the 0-RTT read key
  - close connection if client attempts 0-RTT, but ticket didn't allow it
  - encode the max early data size into the session ticket
  - implement parsing of the early_data extension in the EncryptedExtensions
  - add a tls.Config.MaxEarlyData option to enable 0-RTT
  - accept TLS 1.3 cipher suites in Config.CipherSuites
  - introduce a function on the connection to generate a session ticket
  - add a config option to enforce selection of an application protocol
  - export Conn.HandlePostHandshakeMessage
  - export Alert
  - reject Configs that set MaxVersion < 1.3 when using a record layer
  - enforce TLS 1.3 when using an alternative record layer
- github.com/multiformats/go-multistream (v0.1.2 -> v0.2.0):
  - improve negotiation flushing ([multiformats/go-multistream#52](https://github.com/multiformats/go-multistream/pull/52))
- github.com/whyrusleeping/cbor-gen (v0.0.0-20200402171437-3d27c146c105 -> v0.0.0-20200710004633-5379fc63235d):
  - correctly map typegen to cbg in all cases ([whyrusleeping/cbor-gen#26](https://github.com/whyrusleeping/cbor-gen/pull/26))
  - fix: clear struct state on unmarshal ([whyrusleeping/cbor-gen#22](https://github.com/whyrusleeping/cbor-gen/pull/22))
  - deferred: restrict max length ([whyrusleeping/cbor-gen#25](https://github.com/whyrusleeping/cbor-gen/pull/25))
  - reduce number of allocations in ScanForLinks ([whyrusleeping/cbor-gen#24](https://github.com/whyrusleeping/cbor-gen/pull/24))
  - attempt to allocate less by using shared buffers ([whyrusleeping/cbor-gen#18](https://github.com/whyrusleeping/cbor-gen/pull/18))
  - add benchmark
  - use new cid methods for less allocs ([whyrusleeping/cbor-gen#17](https://github.com/whyrusleeping/cbor-gen/pull/17))
  - properly handle roundtripping Deferred with 'null' value ([whyrusleeping/cbor-gen#16](https://github.com/whyrusleeping/cbor-gen/pull/16))
  - Support array types ([whyrusleeping/cbor-gen#15](https://github.com/whyrusleeping/cbor-gen/pull/15))
- github.com/whyrusleeping/tar-utils (v0.0.0-20180509141711-8c6c8ba81d5c -> v0.0.0-20201201191210-20a61371de5b):
  - more closely match default tar errors (GNU + BSD binaries)

Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Eric Myhre | 180 | +26453/-11032 | 883 |
| Marten Seemann | 212 | +14876/-9352 | 794 |
| hannahhoward | 41 | +9195/-3113 | 186 |
| Alex Cruikshank | 5 | +3323/-1895 | 58 |
| Andrew Gillis | 3 | +3792/-581 | 21 |
| vyzo | 49 | +2675/-949 | 95 |
| Adin Schmahmann | 57 | +1473/-837 | 90 |
| Steven Allen | 43 | +1252/-780 | 99 |
| Petar Maymounkov | 3 | +1755/-113 | 17 |
| Marcin Rataj | 35 | +979/-210 | 61 |
| Paul Wolneykien | 2 | +670/-338 | 9 |
| Jeromy Johnson | 9 | +525/-221 | 21 |
| gammazero | 11 | +366/-101 | 26 |
| Hector Sanjuan | 7 | +312/-0 | 11 |
| Dirk McCormick | 4 | +190/-90 | 15 |
| Will Scott | 1 | +252/-0 | 1 |
| Oli Evans | 1 | +201/-0 | 1 |
| Tomasz Zdybał | 2 | +182/-3 | 6 |
| Daniel Martí | 6 | +104/-66 | 35 |
| Sam | 3 | +76/-59 | 5 |
| Łukasz Magiera | 2 | +92/-3 | 5 |
| whyrusleeping | 3 | +77/-15 | 3 |
| nisdas | 3 | +76/-15 | 4 |
| Raúl Kripalani | 3 | +59/-31 | 5 |
| Lucas Molas | 1 | +66/-3 | 2 |
| Alex Towle | 1 | +52/-8 | 2 |
| Dennis Trautwein | 1 | +58/-0 | 2 |
| Adrian Lanzafame | 2 | +49/-7 | 4 |
| klzgrad | 1 | +49/-5 | 2 |
| Fazlul Shahriar | 1 | +35/-14 | 17 |
| Yingrong Zhao | 1 | +45/-2 | 2 |
| Jakub Sztandera | 2 | +22/-13 | 2 |
| Chaitanya | 8 | +16/-16 | 8 |
| Aarsh Shah | 1 | +27/-1 | 3 |
| Rod Vagg | 1 | +23/-4 | 2 |
| M. Hawn | 4 | +11/-11 | 8 |
| Will | 1 | +12/-2 | 1 |
| frrist | 1 | +7/-0 | 1 |
| Rafael Ramalho | 2 | +5/-2 | 2 |
| dependabot[bot] | 1 | +3/-3 | 1 |
| Zaurbek Zhakupov | 1 | +3/-3 | 1 |
| Tom Worrall | 1 | +4/-2 | 1 |
| Jorropo | 2 | +5/-1 | 2 |
| Chaitanya Raju | 1 | +3/-3 | 2 |
| Egon Elbre | 1 | +0/-5 | 1 |
| incognitomode | 1 | +2/-2 | 1 |
| achingbrain | 1 | +2/-2 | 1 |
| Michael Burns | 1 | +2/-2 | 1 |
| David Florness | 2 | +2/-2 | 2 |
| RubenKelevra | 1 | +2/-1 | 1 |
| Andrew Nesbitt | 2 | +2/-1 | 2 |
| Tarun Bansal | 1 | +1/-1 | 1 |
| Max Inden | 1 | +1/-1 | 1 |
| K | 1 | +2/-0 | 1 |
| Jacob Heun | 1 | +1/-1 | 1 |
| Henrique Dias | 1 | +1/-1 | 1 |
| Bryan White | 1 | +1/-1 | 1 |
| Bryan Stenson | 1 | +1/-1 | 1 |

## v0.7.0 2020-09-22

### Highlights

#### Secio is now disabled by default

As part of deprecating and removing support for the Secio security transport, we have disabled it by default. TLS1.3 will remain the default security transport with fallback to Noise. You can read more about the deprecation in the blog post, https://blog.ipfs.io/2020-08-07-deprecating-secio/. If you're running IPFS older than 0.5, this may start to impact your performance on the public network.

#### Ed25519 keys are now used by default

Previously go-ipfs generated 2048 bit RSA keys for new nodes, but it will now use ed25519 keys by default. This will not affect any existing keys, but newly created keys will be ed25519 by default. The main benefit of using ed25519 keys over RSA is that ed25519 keys have an inline public key. This means that someone only needs your PeerId to verify things you've signed, which means we don't have to worry about storing those bulky RSA public keys.

##### Rotating keys

Along with switching the default, we've added support for rotating keys. If you would like to change the key type of your IPFS node, you can now do so with the rotate command. **NOTE: This will affect your Peer Id, so be sure you want to do this!** Your existing identity key will be backed up in the Keystore.

```bash
ipfs key rotate -o my-old-key -t ed25519
```

#### Key export/import

We've added commands to allow you to export and import keys from the IPFS Keystore to a local .key file. This does not apply to the IPFS identity key, `self`.

```bash
ipfs key gen mykey
ipfs key export -o mykey.key mykey # ./<name>.key is the default path
ipfs key import mykey mykey.key # on another node
```

#### IPNS paths now encode the key name as a base36 CIDv1 by default

Previously go-ipfs encoded the key names for IPNS paths as base58btc multihashes (e.g. Qmabc...). We now encode them as base36 encoded CIDv1s as defined in the [peerID spec](https://github.com/libp2p/specs/blob/master/peer-ids/peer-ids.md#string-representation) (e.g. k51xyz...) which also deals with encoding of public keys. This is nice because it means that IPNS keys will by default be case-insensitive and that they will fit into DNS labels (e.g. k51xyz...ipns.localhost) and therefore that subdomain gateway redirections (e.g. from localhost:8080/ipns/{key} to {key}.ipns.localhost) will look better to users in the default case.

Many commands will accept a `--ipns-base` option that allows changing command outputs to use a particular encoding (i.e.  base58btc multihash, or CIDv1 encoded in any supported base)

#### Multiaddresses now accept PeerIDs encoded as CIDv1

In preparation for eventually changing the default PeerID representation multiaddresses can now contain strings like `/p2p/k51xyz...` in addition to the default `/p2p/Qmabc...`. There is a corresponding `--peerid-base` option to many functions that output peerIDs.

#### `dag stat`

Initial support has been added for the `ipfs dag stat` command. Running this command will traverse the DAG for the given root CID and report statistics. By default, progress will be shown as the DAG is traversed. Supported statistics currently include DAG size and number of blocks.

```bash
ipfs dag stat bafybeihpetclqvwb4qnmumvcn7nh4pxrtugrlpw4jgjpqicdxsv7opdm6e # the IPFS webui
Size: 30362191, NumBlocks: 346
```

#### Plugin build changes

We have changed the build flags used by the official binary distributions on dist.ipfs.io (or `/ipns/dist.ipfs.io`) to use the simpler and more reliable `-trimpath` flag instead of the more complicated and brittle `-asmflags=all=-trimpath="$(GOPATH)" -gcflags=all=-trimpath="$(GOPATH)"` flags, however the build flags used by default in go-ipfs remain the same.

The scripts in https://github.com/ipfs/go-ipfs-example-plugin have been updated to reflect this change. This is a breaking change to how people have been building plugins against the dist.ipfs.io binary of go-ipfs and plugins should update their build processes accordingly see https://github.com/ipfs/go-ipfs-example-plugin/pull/9 for details.

### Changelog

- github.com/ipfs/go-ipfs:
  - chore: bump webui version
  - fix: remove the (empty) alias for --peerid-base
  - Release v0.7.0-rc2
  - fix: use override GOFLAGS changes from 480defab689610550ee3d346e31441a2bb881fcb but keep trimpath usage as is
  - Revert "fix: override GOFLAGS"
  - fix: remove the (empty) alias for --ipns-base
  - refactor: put all --ipns-base options in one place
  - docs: update config to indicate SECIO deprecation
  - fix: ipfs dht put/get commands now work on keys encoded as peerIDs and fail early for namespaces other than /pk or /ipns
  - Release v0.7.0-rc1
  - chore: cleanup ([ipfs/go-ipfs#7628](https://github.com/ipfs/go-ipfs/pull/7628))
  - namesys: fixed IPNS republisher to not overwrite IPNS record lifetimes ([ipfs/go-ipfs#7627](https://github.com/ipfs/go-ipfs/pull/7627))
  - Fix #7624: Do not fetch dag nodes when checking if a pin exists ([ipfs/go-ipfs#7625](https://github.com/ipfs/go-ipfs/pull/7625))
  - chore: update dependencies ([ipfs/go-ipfs#7610](https://github.com/ipfs/go-ipfs/pull/7610))
  - use t.Cleanup() to reduce the need to clean up servers in tests ([ipfs/go-ipfs#7550](https://github.com/ipfs/go-ipfs/pull/7550))
  - fix: ipfs pin ls - ignore pins that have errors ([ipfs/go-ipfs#7612](https://github.com/ipfs/go-ipfs/pull/7612))
  - docs(config): fix Peering header ([ipfs/go-ipfs#7623](https://github.com/ipfs/go-ipfs/pull/7623))
  - sharness: use dnsaddr example in ipfs p2p command tests ([ipfs/go-ipfs#7620](https://github.com/ipfs/go-ipfs/pull/7620))
  - fix(key): dont allow backup key to be named 'self' ([ipfs/go-ipfs#7615](https://github.com/ipfs/go-ipfs/pull/7615))
  - [BOUNTY] Directory page UI improvements ([ipfs/go-ipfs#7536](https://github.com/ipfs/go-ipfs/pull/7536))
  - fix: make assets deterministic ([ipfs/go-ipfs#7609](https://github.com/ipfs/go-ipfs/pull/7609))
  - use ed25519 keys by default ([ipfs/go-ipfs#7579](https://github.com/ipfs/go-ipfs/pull/7579))
  - feat: wildcard support for public gateways ([ipfs/go-ipfs#7319](https://github.com/ipfs/go-ipfs/pull/7319))
  - fix: fix go-bindata import path ([ipfs/go-ipfs#7605](https://github.com/ipfs/go-ipfs/pull/7605))
  - Upgrade graphsync deps ([ipfs/go-ipfs#7598](https://github.com/ipfs/go-ipfs/pull/7598))
  - Add --peerid-base to ipfs id command ([ipfs/go-ipfs#7591](https://github.com/ipfs/go-ipfs/pull/7591))
  - use b36 keys by default for keys and IPNS ([ipfs/go-ipfs#7582](https://github.com/ipfs/go-ipfs/pull/7582))
  - add ipfs dag stat command (#7553) ([ipfs/go-ipfs#7553](https://github.com/ipfs/go-ipfs/pull/7553))
  - Move key rotation command to ipfs key rotate ([ipfs/go-ipfs#7599](https://github.com/ipfs/go-ipfs/pull/7599))
  - Disable secio by default ([ipfs/go-ipfs#7600](https://github.com/ipfs/go-ipfs/pull/7600))
  - Stop searching for public keys before doing an IPNS Get (#7549) ([ipfs/go-ipfs#7549](https://github.com/ipfs/go-ipfs/pull/7549))
  - feat: return supported protocols in id output ([ipfs/go-ipfs#7409](https://github.com/ipfs/go-ipfs/pull/7409))
  - docs: fix typo in default swarm addrs config docs ([ipfs/go-ipfs#7585](https://github.com/ipfs/go-ipfs/pull/7585))
  - feat: nice errors when failing to load plugins ([ipfs/go-ipfs#7429](https://github.com/ipfs/go-ipfs/pull/7429))
  - doc: document reverse proxy bug ([ipfs/go-ipfs#7478](https://github.com/ipfs/go-ipfs/pull/7478))
  - fix: ipfs name resolve --dht-record-count flag uses correct type and now works
  - refactor: get rid of cmdDetails awkwardness
  - IPNS format keys in b36cid ([ipfs/go-ipfs#7554](https://github.com/ipfs/go-ipfs/pull/7554))
  - Key import and export cli commands ([ipfs/go-ipfs#7546](https://github.com/ipfs/go-ipfs/pull/7546))
  - feat: add snap package configuration ([ipfs/go-ipfs#7529](https://github.com/ipfs/go-ipfs/pull/7529))
  - chore: bump webui version
  - repeat gateway subdomain test for all key types (#7542) ([ipfs/go-ipfs#7542](https://github.com/ipfs/go-ipfs/pull/7542))
  - fix: override GOFLAGS
  - update QUIC, enable the RetireBugBackwardsCompatibilityMode
  - Document add behavior when the daemon is not running ([ipfs/go-ipfs#7514](https://github.com/ipfs/go-ipfs/pull/7514))
  -  ([ipfs/go-ipfs#7515](https://github.com/ipfs/go-ipfs/pull/7515))
  - Choose Key type at initialization ([ipfs/go-ipfs#7251](https://github.com/ipfs/go-ipfs/pull/7251))
  - feat: add flag to ipfs key and list to output keys in b36/CIDv1 (#7531) ([ipfs/go-ipfs#7531](https://github.com/ipfs/go-ipfs/pull/7531))
  - feat: support ED25519 libp2p-key in subdomains
  - chore: fix a typo
  - docs: document X-Forwarded-Host
  - feat: support X-Forwarded-Host when doing gateway redirect
  - chore: update test deps for graphsync
  - chore: bump test dependencies ([ipfs/go-ipfs#7524](https://github.com/ipfs/go-ipfs/pull/7524))
  - fix: use static binaries in docker container ([ipfs/go-ipfs#7505](https://github.com/ipfs/go-ipfs/pull/7505))
  - chore:bump webui version to 2.10.1 ([ipfs/go-ipfs#7504](https://github.com/ipfs/go-ipfs/pull/7504))
  - chore: bump webui version ([ipfs/go-ipfs#7501](https://github.com/ipfs/go-ipfs/pull/7501))
  - update version to 0.7.0-dev
  - Merge branch 'release' into master
  - systemd: specify repo path, to avoid unnecessary subdirectory ([ipfs/go-ipfs#7472](https://github.com/ipfs/go-ipfs/pull/7472))
  - doc(prod): start documenting production stuff ([ipfs/go-ipfs#7469](https://github.com/ipfs/go-ipfs/pull/7469))
  - Readme: Update link about init systems (and import old readme) ([ipfs/go-ipfs#7473](https://github.com/ipfs/go-ipfs/pull/7473))
  - doc(config): expand peering docs ([ipfs/go-ipfs#7466](https://github.com/ipfs/go-ipfs/pull/7466))
  - fix: Use the -p option in Dockerfile to make parents as needed ([ipfs/go-ipfs#7464](https://github.com/ipfs/go-ipfs/pull/7464))
  - systemd: enable systemd hardening features ([ipfs/go-ipfs#7286](https://github.com/ipfs/go-ipfs/pull/7286))
  - fix(migration): migrate /ipfs/ bootstrappers to /p2p/ ([ipfs/go-ipfs#7450](https://github.com/ipfs/go-ipfs/pull/7450))
  - readme: update go-version ([ipfs/go-ipfs#7447](https://github.com/ipfs/go-ipfs/pull/7447))
  - fix(migration): correctly migrate quic addresses ([ipfs/go-ipfs#7446](https://github.com/ipfs/go-ipfs/pull/7446))
  - chore: add migration to listen on QUIC by default ([ipfs/go-ipfs#7443](https://github.com/ipfs/go-ipfs/pull/7443))
  - go: bump minimal dependency to 1.14.4 ([ipfs/go-ipfs#7419](https://github.com/ipfs/go-ipfs/pull/7419))
  - fix: use bitswap sessions for ipfs refs ([ipfs/go-ipfs#7389](https://github.com/ipfs/go-ipfs/pull/7389))
  - fix(commands): print consistent addresses in ipfs id ([ipfs/go-ipfs#7397](https://github.com/ipfs/go-ipfs/pull/7397))
  - fix two pubsub issues. ([ipfs/go-ipfs#7394](https://github.com/ipfs/go-ipfs/pull/7394))
  - docs: add pacman.store (@RubenKelevra) to the early testers ([ipfs/go-ipfs#7368](https://github.com/ipfs/go-ipfs/pull/7368))
  - Update docs-beta links to final URLs ([ipfs/go-ipfs#7386](https://github.com/ipfs/go-ipfs/pull/7386))
  - feat: webui v2.9.0 ([ipfs/go-ipfs#7387](https://github.com/ipfs/go-ipfs/pull/7387))
  - chore: update WebUI to 2.8.0 ([ipfs/go-ipfs#7380](https://github.com/ipfs/go-ipfs/pull/7380))
  - mailmap support ([ipfs/go-ipfs#7375](https://github.com/ipfs/go-ipfs/pull/7375))
  - doc: update the release template for git flow changes ([ipfs/go-ipfs#7370](https://github.com/ipfs/go-ipfs/pull/7370))
  - chore: update deps ([ipfs/go-ipfs#7369](https://github.com/ipfs/go-ipfs/pull/7369))
- github.com/ipfs/go-bitswap (v0.2.19 -> v0.2.20):
  - fix: don't say we're sending a full wantlist unless we are (#429) ([ipfs/go-bitswap#429](https://github.com/ipfs/go-bitswap/pull/429))
- github.com/ipfs/go-cid (v0.0.6 -> v0.0.7):
  - feat: optimize cid.Prefix ([ipfs/go-cid#109](https://github.com/ipfs/go-cid/pull/109))
- github.com/ipfs/go-datastore (v0.4.4 -> v0.4.5):
  - Add test to ensure that Delete returns no error for missing keys ([ipfs/go-datastore#162](https://github.com/ipfs/go-datastore/pull/162))
  - Fix typo in sync/sync.go ([ipfs/go-datastore#159](https://github.com/ipfs/go-datastore/pull/159))
  - Add the generated flatfs stub, since it cannot be auto-generated ([ipfs/go-datastore#158](https://github.com/ipfs/go-datastore/pull/158))
  - support flatfs fuzzing ([ipfs/go-datastore#157](https://github.com/ipfs/go-datastore/pull/157))
  - fuzzing harness (#153) ([ipfs/go-datastore#153](https://github.com/ipfs/go-datastore/pull/153))
  - feat(mount): don't give up on error ([ipfs/go-datastore#146](https://github.com/ipfs/go-datastore/pull/146))
  - /test: fix bad ElemCount/10 lenght (should not be divided) ([ipfs/go-datastore#152](https://github.com/ipfs/go-datastore/pull/152))
- github.com/ipfs/go-ds-flatfs (v0.4.4 -> v0.4.5):
  - Add os.Rename wrapper for Plan 9 (#87) ([ipfs/go-ds-flatfs#87](https://github.com/ipfs/go-ds-flatfs/pull/87))
- github.com/ipfs/go-fs-lock (v0.0.5 -> v0.0.6):
  - Fix build on Plan 9 ([ipfs/go-fs-lock#17](https://github.com/ipfs/go-fs-lock/pull/17))
- github.com/ipfs/go-graphsync (v0.0.5 -> v0.1.1):
  - docs(CHANGELOG): update for v0.1.1
  - docs(CHANGELOG): update for v0.1.0 release ([ipfs/go-graphsync#84](https://github.com/ipfs/go-graphsync/pull/84))
  - Dedup by key extension (#83) ([ipfs/go-graphsync#83](https://github.com/ipfs/go-graphsync/pull/83))
  - Release infrastructure (#81) ([ipfs/go-graphsync#81](https://github.com/ipfs/go-graphsync/pull/81))
  - feat(persistenceoptions): add unregister ability (#80) ([ipfs/go-graphsync#80](https://github.com/ipfs/go-graphsync/pull/80))
  - fix(message): regen protobuf code (#79) ([ipfs/go-graphsync#79](https://github.com/ipfs/go-graphsync/pull/79))
  - feat(requestmanager): run response hooks on completed requests (#77) ([ipfs/go-graphsync#77](https://github.com/ipfs/go-graphsync/pull/77))
  - Revert "add extensions on complete (#76)"
  - add extensions on complete (#76) ([ipfs/go-graphsync#76](https://github.com/ipfs/go-graphsync/pull/76))
  - All changes to date including pause requests & start paused, along with new adds for cleanups and checking of execution (#75) ([ipfs/go-graphsync#75](https://github.com/ipfs/go-graphsync/pull/75))
  - More fine grained response controls (#71) ([ipfs/go-graphsync#71](https://github.com/ipfs/go-graphsync/pull/71))
  - Refactor request execution and use IPLD SkipMe functionality for proper partial results on a request (#70) ([ipfs/go-graphsync#70](https://github.com/ipfs/go-graphsync/pull/70))
  - feat(graphsync): implement do-no-send-cids extension (#69) ([ipfs/go-graphsync#69](https://github.com/ipfs/go-graphsync/pull/69))
  - Incoming Block Hooks (#68) ([ipfs/go-graphsync#68](https://github.com/ipfs/go-graphsync/pull/68))
  - fix(responsemanager): add nil check (#67) ([ipfs/go-graphsync#67](https://github.com/ipfs/go-graphsync/pull/67))
  - refactor(hooks): use external pubsub (#65) ([ipfs/go-graphsync#65](https://github.com/ipfs/go-graphsync/pull/65))
  - Update of IPLD Prime (#66) ([ipfs/go-graphsync#66](https://github.com/ipfs/go-graphsync/pull/66))
  - feat(responsemanager): add listener for completed responses (#64) ([ipfs/go-graphsync#64](https://github.com/ipfs/go-graphsync/pull/64))
  - Update Requests (#63) ([ipfs/go-graphsync#63](https://github.com/ipfs/go-graphsync/pull/63))
  - Add pausing and unpausing of requests (#62) ([ipfs/go-graphsync#62](https://github.com/ipfs/go-graphsync/pull/62))
  - Outgoing Request Hooks, swapping persistence layers (#61) ([ipfs/go-graphsync#61](https://github.com/ipfs/go-graphsync/pull/61))
  - Feat/request hook loader chooser (#60) ([ipfs/go-graphsync#60](https://github.com/ipfs/go-graphsync/pull/60))
  - Option to Reject requests by default (#58) ([ipfs/go-graphsync#58](https://github.com/ipfs/go-graphsync/pull/58))
  - Testify refactor (#56) ([ipfs/go-graphsync#56](https://github.com/ipfs/go-graphsync/pull/56))
  - Switch To Circle CI (#57) ([ipfs/go-graphsync#57](https://github.com/ipfs/go-graphsync/pull/57))
  - fix(deps): go mod tidy
  - docs(README): remove ipldbridge reference
  - Tech Debt: Remove IPLD Bridge ([ipfs/go-graphsync#55](https://github.com/ipfs/go-graphsync/pull/55))
- github.com/ipfs/go-ipfs-cmds (v0.2.9 -> v0.4.0):
  - fix: allow requests from electron renderer (#201) ([ipfs/go-ipfs-cmds#201](https://github.com/ipfs/go-ipfs-cmds/pull/201))
  - refactor: move external command checks into commands lib (#198) ([ipfs/go-ipfs-cmds#198](https://github.com/ipfs/go-ipfs-cmds/pull/198))
  - Fix build on Plan 9 ([ipfs/go-ipfs-cmds#199](https://github.com/ipfs/go-ipfs-cmds/pull/199))
- github.com/ipfs/go-ipfs-config (v0.8.0 -> v0.9.0):
  - error if bit size specified with ed25519 keys (#105) ([ipfs/go-ipfs-config#105](https://github.com/ipfs/go-ipfs-config/pull/105))
- github.com/ipfs/go-log/v2 (v2.0.8 -> v2.1.1):
  failed to fetch repo
- github.com/ipfs/go-path (v0.0.7 -> v0.0.8):
  - ResolveToLastNode no longer fetches nodes it does not need ([ipfs/go-path#30](https://github.com/ipfs/go-path/pull/30))
  - doc: add a lead maintainer
- github.com/ipfs/interface-go-ipfs-core (v0.3.0 -> v0.4.0):
  - Add ID formatting functions, used by various IPFS cli commands ([ipfs/interface-go-ipfs-core#65](https://github.com/ipfs/interface-go-ipfs-core/pull/65))
- github.com/ipld/go-car (v0.1.0 -> v0.1.1-0.20200429200904-c222d793c339):
  - Update go-ipld-prime to the era of NodeAssembler. ([ipld/go-car#31](https://github.com/ipld/go-car/pull/31))
  - fix: update the cli tool's car dep ([ipld/go-car#30](https://github.com/ipld/go-car/pull/30))
- github.com/ipld/go-ipld-prime (v0.0.2-0.20191108012745-28a82f04c785 -> v0.0.2-0.20200428162820-8b59dc292b8e):
  - Add two basic examples of usage, as go tests.
  - Fix marshalling error ([ipld/go-ipld-prime#53](https://github.com/ipld/go-ipld-prime/pull/53))
  - Add more test specs for list and map nesting.
  - traversal.SkipMe feature ([ipld/go-ipld-prime#51](https://github.com/ipld/go-ipld-prime/pull/51))
  - Improvements to traversal docs.
  - Drop code coverage bot config. ([ipld/go-ipld-prime#50](https://github.com/ipld/go-ipld-prime/pull/50))
  - Promote NodeAssembler/NodeStyle interface rework to core, and use improved basicnode implementation. ([ipld/go-ipld-prime#49](https://github.com/ipld/go-ipld-prime/pull/49))
  - Merge branch 'traversal-benchmarks'
  - Merge branch 'cycle-breaking-and-traversal-benchmarks'
  - Merge branch 'assembler-upgrade-to-codecs'
  - Path clarifications ([ipld/go-ipld-prime#47](https://github.com/ipld/go-ipld-prime/pull/47))
  - Merge branch 'research-admissions'
  - Add a typed link node to allow traversal with code gen'd builders across links ([ipld/go-ipld-prime#41](https://github.com/ipld/go-ipld-prime/pull/41))
  - Merge branch 'research-admissions'
  - Library updates.
  - Feat/add code gen disclaimer ([ipld/go-ipld-prime#39](https://github.com/ipld/go-ipld-prime/pull/39))
  - Readme and key Node interface docs improvements.
  - fix(schema/gen): return value not reference ([ipld/go-ipld-prime#38](https://github.com/ipld/go-ipld-prime/pull/38))
- github.com/ipld/go-ipld-prime-proto (v0.0.0-20191113031812-e32bd156a1e5 -> v0.0.0-20200428191222-c1ffdadc01e1):
  - feat(deps): upgrade to new IPLD prime ([ipld/go-ipld-prime-proto#1](https://github.com/ipld/go-ipld-prime-proto/pull/1))
  - Update to latest ipld before rework ([ipld/go-ipld-prime-proto#2](https://github.com/ipld/go-ipld-prime-proto/pull/2))
- github.com/libp2p/go-libp2p (v0.9.6 -> v0.11.0):
  - Added parsing of IPv6 addresses for incoming mDNS requests ([libp2p/go-libp2p#990](https://github.com/libp2p/go-libp2p/pull/990))
  - Switch from SECIO to Noise ([libp2p/go-libp2p#972](https://github.com/libp2p/go-libp2p/pull/972))
  - fix tests ([libp2p/go-libp2p#995](https://github.com/libp2p/go-libp2p/pull/995))
  - Bump Autonat version & validate fixed call loop in `.Addrs` (#988) ([libp2p/go-libp2p#988](https://github.com/libp2p/go-libp2p/pull/988))
  - fix: use the correct external address when NAT port-mapping ([libp2p/go-libp2p#987](https://github.com/libp2p/go-libp2p/pull/987))
  - upgrade deps + interoperable uvarint delimited writer/reader. (#985) ([libp2p/go-libp2p#985](https://github.com/libp2p/go-libp2p/pull/985))
  - fix host can be dialed by autonat public addr, but lost the public addr to announce ([libp2p/go-libp2p#983](https://github.com/libp2p/go-libp2p/pull/983))
  - Fix address advertisement bugs (#974) ([libp2p/go-libp2p#974](https://github.com/libp2p/go-libp2p/pull/974))
  - fix: avoid a close deadlock in the natmanager ([libp2p/go-libp2p#971](https://github.com/libp2p/go-libp2p/pull/971))
  - upgrade swarm; add ID() on mock conns and streams. (#970) ([libp2p/go-libp2p#970](https://github.com/libp2p/go-libp2p/pull/970))
- github.com/libp2p/go-libp2p-asn-util (null -> v0.0.0-20200825225859-85005c6cf052):
  - chore: go fmt
  - feat: use deferred initialization of the asnStore ([libp2p/go-libp2p-asn-util#3](https://github.com/libp2p/go-libp2p-asn-util/pull/3))
  - chore: switch to forked cidranger
  - fixed code
  - library for ASN mappings
- github.com/libp2p/go-libp2p-autonat (v0.2.3 -> v0.3.2):
  - static nat shouldn't call host.Addrs()
  - upgrade deps + interoperable uvarint delimited writer/reader. (#95) ([libp2p/go-libp2p-autonat#95](https://github.com/libp2p/go-libp2p-autonat/pull/95))
  - fix: a type switch nit ([libp2p/go-libp2p-autonat#83](https://github.com/libp2p/go-libp2p-autonat/pull/83))
- github.com/libp2p/go-libp2p-blankhost (v0.1.6 -> v0.2.0):
  - call reset where appropriate (and update deps) ([libp2p/go-libp2p-blankhost#52](https://github.com/libp2p/go-libp2p-blankhost/pull/52))
- github.com/libp2p/go-libp2p-circuit (v0.2.3 -> v0.3.1):
  - upgrade deps + interoperable uvarints. (#122) ([libp2p/go-libp2p-circuit#122](https://github.com/libp2p/go-libp2p-circuit/pull/122))
  - Fix/remove deprecated logging ([libp2p/go-libp2p-circuit#85](https://github.com/libp2p/go-libp2p-circuit/pull/85))
- github.com/libp2p/go-libp2p-core (v0.5.7 -> v0.6.1):
  - experimental introspection support (#159) ([libp2p/go-libp2p-core#159](https://github.com/libp2p/go-libp2p-core/pull/159))
- github.com/libp2p/go-libp2p-discovery (v0.4.0 -> v0.5.0):
  - Put period at end of sentence ([libp2p/go-libp2p-discovery#65](https://github.com/libp2p/go-libp2p-discovery/pull/65))
- github.com/libp2p/go-libp2p-kad-dht (v0.8.2 -> v0.9.0):
  - chore: update deps ([libp2p/go-libp2p-kad-dht#689](https://github.com/libp2p/go-libp2p-kad-dht/pull/689))
  - allow overwriting builtin dual DHT options ([libp2p/go-libp2p-kad-dht#688](https://github.com/libp2p/go-libp2p-kad-dht/pull/688))
  - Hardening Improvements: RT diversity and decreased RT churn ([libp2p/go-libp2p-kad-dht#687](https://github.com/libp2p/go-libp2p-kad-dht/pull/687))
  - Fix key log encoding ([libp2p/go-libp2p-kad-dht#682](https://github.com/libp2p/go-libp2p-kad-dht/pull/682))
  - upgrade deps + uvarint delimited writer/reader. (#684) ([libp2p/go-libp2p-kad-dht#684](https://github.com/libp2p/go-libp2p-kad-dht/pull/684))
  - periodicBootstrapInterval should be ticker? (#678) ([libp2p/go-libp2p-kad-dht#678](https://github.com/libp2p/go-libp2p-kad-dht/pull/678))
  - removes duplicate comment ([libp2p/go-libp2p-kad-dht#674](https://github.com/libp2p/go-libp2p-kad-dht/pull/674))
  - Revert "Peer Diversity in the Routing Table (#658)" ([libp2p/go-libp2p-kad-dht#670](https://github.com/libp2p/go-libp2p-kad-dht/pull/670))
  - Fixed problem with refresh logging ([libp2p/go-libp2p-kad-dht#667](https://github.com/libp2p/go-libp2p-kad-dht/pull/667))
  - feat: protect all peers in low buckets, tag everyone else with 5 ([libp2p/go-libp2p-kad-dht#666](https://github.com/libp2p/go-libp2p-kad-dht/pull/666))
  - Peer Diversity in the Routing Table (#658) ([libp2p/go-libp2p-kad-dht#658](https://github.com/libp2p/go-libp2p-kad-dht/pull/658))
- github.com/libp2p/go-libp2p-kbucket (v0.4.2 -> v0.4.7):
  - chore: switch from go-multiaddr-net to go-multiaddr/net
  - Use crypto/rand for generating random prefixes
  - feat: when using the diversity filter for ipv6 addresses if the ASN cannot be found for a particular address then fallback on using the /32 mask of the  address as the group name instead of simply rejecting the peer from routing table
  - simplify filter (#92) ([libp2p/go-libp2p-kbucket#92](https://github.com/libp2p/go-libp2p-kbucket/pull/92))
  - fix: switch to forked cid ranger dep ([libp2p/go-libp2p-kbucket#91](https://github.com/libp2p/go-libp2p-kbucket/pull/91))
  - Reduce Routing Table churn (#90) ([libp2p/go-libp2p-kbucket#90](https://github.com/libp2p/go-libp2p-kbucket/pull/90))
  - Peer Diversity for Routing Table and Querying (#88) ([libp2p/go-libp2p-kbucket#88](https://github.com/libp2p/go-libp2p-kbucket/pull/88))
  - fix bug in peer eviction (#87) ([libp2p/go-libp2p-kbucket#87](https://github.com/libp2p/go-libp2p-kbucket/pull/87))
  - feat: add an AddedAt timestamp (#84) ([libp2p/go-libp2p-kbucket#84](https://github.com/libp2p/go-libp2p-kbucket/pull/84))
- github.com/libp2p/go-libp2p-pubsub (v0.3.1 -> v0.3.5):
  - regenerate protobufs (#381) ([libp2p/go-libp2p-pubsub#381](https://github.com/libp2p/go-libp2p-pubsub/pull/381))
  - track validation time
  - fullfill promise as soon as a message begins validation
  - don't apply penalty in self origin rejections
  - add behaviour penalty threshold
  - Add String() method to Topic.
  - add regression test for issue 371
  - don't add direct peers to fanout
  - reference spec change in comment.
  - fix backoff slack time
  - use the heartbeat interval for slack time
  - add slack time to prune backoff clearance
  - fix: call the correct tracer function in FloodSubRouter.Leave (#373) ([libp2p/go-libp2p-pubsub#373](https://github.com/libp2p/go-libp2p-pubsub/pull/373))
  - downgrade trace buffer overflow log to debug
  - track topics in Reject/Duplicate/Deliver events
  - add topics to Reject/Duplicate/Deliver events
  - fix flaky test
  - refactor ip colocation factor computation that is common for score and inspection
  - better handling of intermediate topic score snapshots
  - disallow duplicate score inspectors
  - make peer score inspect function types aliases
  - extended peer score inspection
  - upgrade deps + interoperable uvarint delimited writer/reader.
  - Add warning about messageIDs
  - Signing policy + optional Signature, From and Seqno ([libp2p/go-libp2p-pubsub#359](https://github.com/libp2p/go-libp2p-pubsub/pull/359))
  - Update pubsub.go
  - Define a public error ErrSubscriptionCancelled.
  - only do PX on leave if PX was enabled in the node
  - drop warning about failure to open stream to a debug log
  - reinstate tagging (now protection) tests
  - disable tests for direct/mesh tags, we don't have an interface to query the connman yet
  - protect direct and mesh peers in the connection manager
  - feat: add direct connect ticks option
- github.com/libp2p/go-libp2p-pubsub-router (v0.3.0 -> v0.3.2):
  - upgrade deps + interoperable uvarint delimited writer/reader. (#79) ([libp2p/go-libp2p-pubsub-router#79](https://github.com/libp2p/go-libp2p-pubsub-router/pull/79))
- github.com/libp2p/go-libp2p-quic-transport (v0.6.0 -> v0.8.0):
  - update quic-go to v0.18.0 (#171) ([libp2p/go-libp2p-quic-transport#171](https://github.com/libp2p/go-libp2p-quic-transport/pull/171))
- github.com/libp2p/go-libp2p-swarm (v0.2.6 -> v0.2.8):
  - slim down dependencies ([libp2p/go-libp2p-swarm#225](https://github.com/libp2p/go-libp2p-swarm/pull/225))
  - `ID()` method on connections and streams + record opening time (#224) ([libp2p/go-libp2p-swarm#224](https://github.com/libp2p/go-libp2p-swarm/pull/224))
- github.com/libp2p/go-libp2p-testing (v0.1.1 -> v0.2.0):
  - Add net benchmark harness ([libp2p/go-libp2p-testing#21](https://github.com/libp2p/go-libp2p-testing/pull/21))
  - Update suite to check that streams respect mux.ErrReset. ([libp2p/go-libp2p-testing#16](https://github.com/libp2p/go-libp2p-testing/pull/16))
- github.com/libp2p/go-maddr-filter (v0.0.5 -> v0.1.0):
  - deprecate this package; moved to multiformats/go-multiaddr. (#23) ([libp2p/go-maddr-filter#23](https://github.com/libp2p/go-maddr-filter/pull/23))
  - chore(dep): update ([libp2p/go-maddr-filter#18](https://github.com/libp2p/go-maddr-filter/pull/18))
- github.com/libp2p/go-msgio (v0.0.4 -> v0.0.6):
  - interoperable uvarints. (#21) ([libp2p/go-msgio#21](https://github.com/libp2p/go-msgio/pull/21))
  - upgrade deps + interoperable uvarint delimited writer/reader. (#20) ([libp2p/go-msgio#20](https://github.com/libp2p/go-msgio/pull/20))
- github.com/libp2p/go-netroute (v0.1.2 -> v0.1.3):
  - add Plan 9 support
- github.com/libp2p/go-openssl (v0.0.5 -> v0.0.7):
  - make ed25519 less special ([libp2p/go-openssl#7](https://github.com/libp2p/go-openssl/pull/7))
  - Add required bindings to support openssl in libp2p-tls ([libp2p/go-openssl#6](https://github.com/libp2p/go-openssl/pull/6))
- github.com/libp2p/go-reuseport (v0.0.1 -> v0.0.2):
  - Fix build on Plan 9 ([libp2p/go-reuseport#79](https://github.com/libp2p/go-reuseport/pull/79))
  - farewell gx; thanks for serving us well.
  - update readme badges
  - remove Jenkinsfile.
- github.com/libp2p/go-reuseport-transport (v0.0.3 -> v0.0.4):
  - Update go-netroute and go-reuseport for Plan 9 support
  - Fix build on Plan 9
- github.com/lucas-clemente/quic-go (v0.16.2 -> v0.18.0):
  - create a milestone version for v0.18.x
  - add Changelog entries for v0.17 ([lucas-clemente/quic-go#2726](https://github.com/lucas-clemente/quic-go/pull/2726))
  - regenerate the testdata certificate with SAN instead of CommonName ([lucas-clemente/quic-go#2723](https://github.com/lucas-clemente/quic-go/pull/2723))
  - make it possible to use multiple qtls versions at the same time, add support for Go 1.15 ([lucas-clemente/quic-go#2720](https://github.com/lucas-clemente/quic-go/pull/2720))
  - add fuzzing for transport parameters ([lucas-clemente/quic-go#2713](https://github.com/lucas-clemente/quic-go/pull/2713))
  - run golangci-lint on Github Actions ([lucas-clemente/quic-go#2700](https://github.com/lucas-clemente/quic-go/pull/2700))
  - disallow values above 2^60 for Config.MaxIncoming{Uni}Streams ([lucas-clemente/quic-go#2711](https://github.com/lucas-clemente/quic-go/pull/2711))
  - never send a value larger than 2^60 in MAX_STREAMS frames ([lucas-clemente/quic-go#2710](https://github.com/lucas-clemente/quic-go/pull/2710))
  - run the check for go generated files on Github Actions instead of Travis ([lucas-clemente/quic-go#2703](https://github.com/lucas-clemente/quic-go/pull/2703))
  - update QUIC draft version information in README ([lucas-clemente/quic-go#2715](https://github.com/lucas-clemente/quic-go/pull/2715))
  - remove Fuzzit badge from README ([lucas-clemente/quic-go#2714](https://github.com/lucas-clemente/quic-go/pull/2714))
  - use the correct return values in Fuzz() functions ([lucas-clemente/quic-go#2705](https://github.com/lucas-clemente/quic-go/pull/2705))
  - simplify the connection, rename it to sendConn ([lucas-clemente/quic-go#2707](https://github.com/lucas-clemente/quic-go/pull/2707))
  - update qpack to v0.2.0 ([lucas-clemente/quic-go#2704](https://github.com/lucas-clemente/quic-go/pull/2704))
  - remove redundant error check in the stream ([lucas-clemente/quic-go#2718](https://github.com/lucas-clemente/quic-go/pull/2718))
  - put back the packet buffer when parsing the connection ID fails ([lucas-clemente/quic-go#2708](https://github.com/lucas-clemente/quic-go/pull/2708))
  - update fuzzing code for oss-fuzz ([lucas-clemente/quic-go#2702](https://github.com/lucas-clemente/quic-go/pull/2702))
  - fix travis script ([lucas-clemente/quic-go#2701](https://github.com/lucas-clemente/quic-go/pull/2701))
  - remove Fuzzit from Travis config ([lucas-clemente/quic-go#2699](https://github.com/lucas-clemente/quic-go/pull/2699))
  - add a script to check if go generated files are correct ([lucas-clemente/quic-go#2692](https://github.com/lucas-clemente/quic-go/pull/2692))
  - only arm the application data PTO timer after the handshake is confirmed ([lucas-clemente/quic-go#2689](https://github.com/lucas-clemente/quic-go/pull/2689))
  - fix tracing of congestion state updates ([lucas-clemente/quic-go#2691](https://github.com/lucas-clemente/quic-go/pull/2691))
  - fix reading of flag values in integration tests ([lucas-clemente/quic-go#2690](https://github.com/lucas-clemente/quic-go/pull/2690))
  - remove ACK decimation ([lucas-clemente/quic-go#2599](https://github.com/lucas-clemente/quic-go/pull/2599))
  - add a metric for PTOs ([lucas-clemente/quic-go#2686](https://github.com/lucas-clemente/quic-go/pull/2686))
  - remove the H3_EARLY_RESPONSE error ([lucas-clemente/quic-go#2687](https://github.com/lucas-clemente/quic-go/pull/2687))
  - implement tracing for congestion state changes ([lucas-clemente/quic-go#2684](https://github.com/lucas-clemente/quic-go/pull/2684))
  - remove the N connection simulation from the Reno code ([lucas-clemente/quic-go#2682](https://github.com/lucas-clemente/quic-go/pull/2682))
  - remove the SSLR (slow start large reduction) experiment ([lucas-clemente/quic-go#2680](https://github.com/lucas-clemente/quic-go/pull/2680))
  - remove unused connectionStats counters from the Reno implementation ([lucas-clemente/quic-go#2683](https://github.com/lucas-clemente/quic-go/pull/2683))
  - add an integration test that randomly sets tracers ([lucas-clemente/quic-go#2679](https://github.com/lucas-clemente/quic-go/pull/2679))
  - privatize some methods in the congestion controller package ([lucas-clemente/quic-go#2681](https://github.com/lucas-clemente/quic-go/pull/2681))
  - fix out-of-bounds read when creating a multiplexed tracer ([lucas-clemente/quic-go#2678](https://github.com/lucas-clemente/quic-go/pull/2678))
  - run integration tests with qlog and metrics on CircleCI ([lucas-clemente/quic-go#2677](https://github.com/lucas-clemente/quic-go/pull/2677))
  - add a metric for closed connections ([lucas-clemente/quic-go#2676](https://github.com/lucas-clemente/quic-go/pull/2676))
  - trace packets that are sent outside of a connection ([lucas-clemente/quic-go#2675](https://github.com/lucas-clemente/quic-go/pull/2675))
  - trace dropped packets that are dropped before they are passed to any session ([lucas-clemente/quic-go#2670](https://github.com/lucas-clemente/quic-go/pull/2670))
  - add a metric for sent packets ([lucas-clemente/quic-go#2673](https://github.com/lucas-clemente/quic-go/pull/2673))
  - add a metric for lost packets ([lucas-clemente/quic-go#2672](https://github.com/lucas-clemente/quic-go/pull/2672))
  - simplify the Tracer interface by combining the TracerFor... methods ([lucas-clemente/quic-go#2671](https://github.com/lucas-clemente/quic-go/pull/2671))
  - add a metrics package using OpenCensus, trace connections ([lucas-clemente/quic-go#2646](https://github.com/lucas-clemente/quic-go/pull/2646))
  - add a multiplexer for the tracer ([lucas-clemente/quic-go#2665](https://github.com/lucas-clemente/quic-go/pull/2665))
  - introduce a type for stateless reset tokens ([lucas-clemente/quic-go#2668](https://github.com/lucas-clemente/quic-go/pull/2668))
  - log all reasons why a connection is closed ([lucas-clemente/quic-go#2669](https://github.com/lucas-clemente/quic-go/pull/2669))
  - add integration tests using faulty packet conns ([lucas-clemente/quic-go#2663](https://github.com/lucas-clemente/quic-go/pull/2663))
  - don't block sendQueue.Send() if the runloop already exited. ([lucas-clemente/quic-go#2656](https://github.com/lucas-clemente/quic-go/pull/2656))
  - move the SupportedVersions slice out of the wire.Header ([lucas-clemente/quic-go#2664](https://github.com/lucas-clemente/quic-go/pull/2664))
  - add a flag to disable conn ID generation and the check for retired conn IDs ([lucas-clemente/quic-go#2660](https://github.com/lucas-clemente/quic-go/pull/2660))
  - put the session in the packet handler map directly (for client sessions) ([lucas-clemente/quic-go#2667](https://github.com/lucas-clemente/quic-go/pull/2667))
  - don't send write error in CONNECTION_CLOSE frames ([lucas-clemente/quic-go#2666](https://github.com/lucas-clemente/quic-go/pull/2666))
  - reset the PTO count before setting the timer when dropping a PN space ([lucas-clemente/quic-go#2657](https://github.com/lucas-clemente/quic-go/pull/2657))
  - enforce that a connection ID is not retired in a packet that uses that connection ID ([lucas-clemente/quic-go#2651](https://github.com/lucas-clemente/quic-go/pull/2651))
  - don't retire the conn ID that's in use when receiving a retransmission ([lucas-clemente/quic-go#2652](https://github.com/lucas-clemente/quic-go/pull/2652))
  - fix flaky cancelation integration test ([lucas-clemente/quic-go#2649](https://github.com/lucas-clemente/quic-go/pull/2649))
  - fix crash when the qlog callbacks returns a nil io.WriteCloser ([lucas-clemente/quic-go#2648](https://github.com/lucas-clemente/quic-go/pull/2648))
  - fix flaky server test on Travis ([lucas-clemente/quic-go#2645](https://github.com/lucas-clemente/quic-go/pull/2645))
  - fix a typo in the logging package test suite
  - introduce type aliases in the logging package ([lucas-clemente/quic-go#2643](https://github.com/lucas-clemente/quic-go/pull/2643))
  - rename frame fields to the names used in the draft ([lucas-clemente/quic-go#2644](https://github.com/lucas-clemente/quic-go/pull/2644))
  - split the qlog package into a logging and a qlog package, use a tracer interface in the quic.Config ([lucas-clemente/quic-go#2638](https://github.com/lucas-clemente/quic-go/pull/2638))
  - fix HTTP request writing if the Request.Body reads data and returns EOF ([lucas-clemente/quic-go#2642](https://github.com/lucas-clemente/quic-go/pull/2642))
  - handle Version Negotiation packets in the session ([lucas-clemente/quic-go#2640](https://github.com/lucas-clemente/quic-go/pull/2640))
  - increase the packet size of the client's Initial packet ([lucas-clemente/quic-go#2634](https://github.com/lucas-clemente/quic-go/pull/2634))
  - introduce an assertion in the server ([lucas-clemente/quic-go#2637](https://github.com/lucas-clemente/quic-go/pull/2637))
  - use the new qtls interface for (re)storing app data with a session state ([lucas-clemente/quic-go#2631](https://github.com/lucas-clemente/quic-go/pull/2631))
  - remove buffering of HTTP requests ([lucas-clemente/quic-go#2626](https://github.com/lucas-clemente/quic-go/pull/2626))
  - remove superfluous parameters logged when not doing 0-RTT ([lucas-clemente/quic-go#2632](https://github.com/lucas-clemente/quic-go/pull/2632))
  - return an infinite bandwidth if the RTT is zero ([lucas-clemente/quic-go#2636](https://github.com/lucas-clemente/quic-go/pull/2636))
  - drop support for Go 1.13 ([lucas-clemente/quic-go#2628](https://github.com/lucas-clemente/quic-go/pull/2628))
  - remove superfluos handleResetStreamFrame method on the stream ([lucas-clemente/quic-go#2623](https://github.com/lucas-clemente/quic-go/pull/2623))
  - implement a token-bucket pacing algorithm ([lucas-clemente/quic-go#2615](https://github.com/lucas-clemente/quic-go/pull/2615))
  - gracefully handle concurrent stream writes and cancellations ([lucas-clemente/quic-go#2624](https://github.com/lucas-clemente/quic-go/pull/2624))
  - log sent packets right before sending them out ([lucas-clemente/quic-go#2613](https://github.com/lucas-clemente/quic-go/pull/2613))
  - remove unused packet counter in the receivedPacketTracker ([lucas-clemente/quic-go#2611](https://github.com/lucas-clemente/quic-go/pull/2611))
  - rewrite the proxy to avoid packet reordering ([lucas-clemente/quic-go#2617](https://github.com/lucas-clemente/quic-go/pull/2617))
  - fix flaky INVALID_TOKEN integration test ([lucas-clemente/quic-go#2610](https://github.com/lucas-clemente/quic-go/pull/2610))
  - make DialEarly return EarlySession ([lucas-clemente/quic-go#2621](https://github.com/lucas-clemente/quic-go/pull/2621))
  - add debug logging to the packet handler map ([lucas-clemente/quic-go#2608](https://github.com/lucas-clemente/quic-go/pull/2608))
  - increase the minimum pacing delay to 1ms ([lucas-clemente/quic-go#2605](https://github.com/lucas-clemente/quic-go/pull/2605))
- github.com/marten-seemann/qpack (v0.1.0 -> v0.2.0):
  - don't reuse the encoder in the integration tests ([marten-seemann/qpack#18](https://github.com/marten-seemann/qpack/pull/18))
  - use Huffman encoding for field names and values ([marten-seemann/qpack#16](https://github.com/marten-seemann/qpack/pull/16))
  - add more tests for encoding using the static table ([marten-seemann/qpack#15](https://github.com/marten-seemann/qpack/pull/15))
  - Encoder uses the static table. ([marten-seemann/qpack#10](https://github.com/marten-seemann/qpack/pull/10))
  - add gofmt to golangci-lint
  - update qifs to the current version ([marten-seemann/qpack#14](https://github.com/marten-seemann/qpack/pull/14))
  - use golangci-lint for linting ([marten-seemann/qpack#12](https://github.com/marten-seemann/qpack/pull/12))
  - add fuzzing ([marten-seemann/qpack#9](https://github.com/marten-seemann/qpack/pull/9))
  - update qifs
  - use https protocol for submodule clone ([marten-seemann/qpack#7](https://github.com/marten-seemann/qpack/pull/7))
- github.com/marten-seemann/qtls (v0.9.1 -> v0.10.0):
  - add callbacks to store and restore app data along a session state
  - remove support for Go 1.13
- github.com/marten-seemann/qtls-go1-15 (null -> v0.1.0):
  - use a prefix for client session cache keys
  - add callbacks to store and restore app data along a session state
  - don't use TLS 1.3 compatibility mode when using alternative record layer
  - delete the session ticket after attempting 0-RTT
  - reject 0-RTT when a different ALPN is chosen
  - encode the ALPN into the session ticket
  - add a field to the ConnectionState to tell if 0-RTT was used
  - add a callback to tell the client about rejection of 0-RTT
  - don't offer 0-RTT after a HelloRetryRequest
  - add Accept0RTT to Config callback to decide if 0-RTT should be accepted
  - add the option to encode application data into the session ticket
  - export the 0-RTT write key
  - abuse the nonce field of ClientSessionState to save max_early_data_size
  - export the 0-RTT read key
  - close connection if client attempts 0-RTT, but ticket didn't allow it
  - encode the max early data size into the session ticket
  - implement parsing of the early_data extension in the EncryptedExtensions
  - add a tls.Config.MaxEarlyData option to enable 0-RTT
  - accept TLS 1.3 cipher suites in Config.CipherSuites
  - introduce a function on the connection to generate a session ticket
  - add a config option to enforce selection of an application protocol
  - export Conn.HandlePostHandshakeMessage
  - export Alert
  - reject Configs that set MaxVersion < 1.3 when using a record layer
  - enforce TLS 1.3 when using an alternative record layer
- github.com/multiformats/go-multiaddr (v0.2.2 -> v0.3.1):
  - dep: add "codependencies" for handling version conflicts ([multiformats/go-multiaddr#132](https://github.com/multiformats/go-multiaddr/pull/132))
  - Support /p2p addresses encoded as CIDs ([multiformats/go-multiaddr#130](https://github.com/multiformats/go-multiaddr/pull/130))
  - Merge go-multiaddr-net
- github.com/multiformats/go-multiaddr-net (v0.1.5 -> v0.2.0):
  - Deprecate ([multiformats/go-multiaddr-net#72](https://github.com/multiformats/go-multiaddr-net/pull/72))
- github.com/multiformats/go-multihash (v0.0.13 -> v0.0.14):
  - fix: only register one blake2s length ([multiformats/go-multihash#129](https://github.com/multiformats/go-multihash/pull/129))
  - feat: add two filecoin hashes, without Sum() implementations ([multiformats/go-multihash#128](https://github.com/multiformats/go-multihash/pull/128))
  - feat: reduce blake2b allocations by special-casing the 256/512 variants ([multiformats/go-multihash#126](https://github.com/multiformats/go-multihash/pull/126))
- github.com/multiformats/go-multistream (v0.1.1 -> v0.1.2):
  - upgrade deps + interoperable varints. (#51) ([multiformats/go-multistream#51](https://github.com/multiformats/go-multistream/pull/51))
- github.com/multiformats/go-varint (v0.0.5 -> v0.0.6):
  - fix minor interoperability issues. (#6) ([multiformats/go-varint#6](https://github.com/multiformats/go-varint/pull/6))
- github.com/warpfork/go-wish (v0.0.0-20190328234359-8b3e70f8e830 -> v0.0.0-20200122115046-b9ea61034e4a):
  - Add ShouldBeSameTypeAs checker.
  - Integration test update for go versions.
- github.com/whyrusleeping/cbor-gen (v0.0.0-20200123233031-1cdf64d27158 -> v0.0.0-20200402171437-3d27c146c105):
  - Handle Nil values for cbg.Deferred ([whyrusleeping/cbor-gen#14](https://github.com/whyrusleeping/cbor-gen/pull/14))
  - add name of struct field to error messages
  - Support uint64 pointers ([whyrusleeping/cbor-gen#13](https://github.com/whyrusleeping/cbor-gen/pull/13))
  - int64 support in map encoders ([whyrusleeping/cbor-gen#12](https://github.com/whyrusleeping/cbor-gen/pull/12))
  - Fix uint64 typed array gen ([whyrusleeping/cbor-gen#10](https://github.com/whyrusleeping/cbor-gen/pull/10))
  - Fix cbg self referencing import path ([whyrusleeping/cbor-gen#8](https://github.com/whyrusleeping/cbor-gen/pull/8))

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Marten Seemann | 156 | +16428/-42621 | 979 |
| hannahhoward | 42 | +15132/-9819 | 467 |
| Eric Myhre | 114 | +13709/-6898 | 586 |
| Steven Allen | 55 | +1211/-2714 | 95 |
| Adin Schmahmann | 54 | +1660/-783 | 117 |
| Petar Maymounkov | 23 | +1677/-671 | 75 |
| Aarsh Shah | 10 | +1926/-341 | 39 |
| Raúl Kripalani | 17 | +1134/-537 | 53 |
| Will | 1 | +841/-0 | 9 |
| rendaw | 3 | +425/-195 | 12 |
| Will Scott | 8 | +302/-229 | 15 |
| vyzo | 22 | +345/-166 | 23 |
| Fazlul Shahriar | 7 | +452/-44 | 19 |
| Peter Rabbitson | 1 | +353/-118 | 5 |
| Hector Sanjuan | 10 | +451/-3 | 14 |
| Marcin Rataj | 9 | +298/-106 | 16 |
| Łukasz Magiera | 4 | +329/-51 | 12 |
| RubenKelevra | 9 | +331/-7 | 12 |
| Michael Muré | 2 | +259/-69 | 6 |
| jstordeur | 1 | +252/-2 | 5 |
| Diederik Loerakker | 1 | +168/-35 | 7 |
| Tiger | 3 | +138/-52 | 8 |
| Kevin Neaton | 3 | +103/-21 | 9 |
| Rod Vagg | 1 | +50/-40 | 4 |
| Oli Evans | 4 | +60/-9 | 6 |
| achingbrain | 4 | +30/-30 | 5 |
| Cyril Fougeray | 2 | +34/-24 | 2 |
| Luke Tucker | 1 | +31/-1 | 2 |
| sandman | 2 | +23/-7 | 3 |
| Alan Shaw | 1 | +18/-9 | 2 |
| Jacob Heun | 4 | +13/-3 | 4 |
| Jessica Schilling | 3 | +7/-7 | 3 |
| Rafael Ramalho | 4 | +9/-4 | 4 |
| Jeromy Johnson | 2 | +6/-6 | 4 |
| Nick Cabatoff | 1 | +7/-2 | 1 |
| Stephen Solka | 1 | +1/-7 | 1 |
| Preston Van Loon | 2 | +6/-2 | 2 |
| Jakub Sztandera | 2 | +5/-2 | 2 |
| llx | 1 | +3/-3 | 1 |
| Adrian Lanzafame | 1 | +3/-3 | 1 |
| Yusef Napora | 1 | +3/-2 | 1 |
| Louis Thibault | 1 | +5/-0 | 1 |
| Martín Triay | 1 | +4/-0 | 1 |
| Hlib | 1 | +2/-2 | 1 |
| Shotaro Yamada | 1 | +2/-1 | 1 |
| phuslu | 1 | +1/-1 | 1 |
| Zero King | 1 | +1/-1 | 1 |
| Rüdiger Klaehn | 1 | +2/-0 | 1 |
| Nex | 1 | +1/-1 | 1 |
| Mark Gaiser | 1 | +1/-1 | 1 |
| Luflosi | 1 | +1/-1 | 1 |
| David Florness | 1 | +1/-1 | 1 |
| Dean Eigenmann | 1 | +0/-1 | 1 |

## v0.6.0 2020-06-19

This is a relatively small release in terms of code changes, but it contains some significant changes to the IPFS protocol.

### Highlights

The highlights in this release include:

* The QUIC transport is enabled by default. Furthermore, go-ipfs will automatically run a migration to listen on the QUIC transport (on the same address/port as the TCP transport) to make this upgrade process seamless.
* The new NOISE security transport is now supported but won't be selected by default. This transport will replace SECIO as the default cross-language interoperability security transport. TLS 1.3 will still remain the default security transport between go-ipfs nodes for now.

**MIGRATION:** This release contains a small config migration to enable listening on the QUIC transport in addition the TCP transport. This migration will:

* Normalize multiaddrs in the bootstrap list to use the `/p2p/Qm...` syntax for multiaddrs instead of the `/ipfs/Qm...` syntax.
* Add QUIC addresses for the default bootstrapers, as necessary. If you've removed the default bootstrappers from your bootstrap config, the migration won't add them back.
* Add a QUIC listener address to mirror any TCP addresses present in your config. For example, if you're listening on `/ip4/0.0.0.0/tcp/1234`, this migration will add a listen address for `/ip4/0.0.0.0/udp/1234/quic`.

#### QUIC by default

This release enables the QUIC transport (draft 28) by default for both inbound and outbound connections. When connecting to new peers, libp2p will continue to dial all advertised addresses (tcp + quic) in parallel so if the QUIC connection fails for some reason, the connection should still succeed.

The QUIC transport has several key benefits over the current TCP based transports:

* It takes fewer round-trips to establish a connection. With the QUIC transport, the IPFS handshake takes two round trips (one to establish the QUIC connection, one for the libp2p handshake). In the future, we should be able to reduce this to one round trip for the initial connection, and zero round trips for subsequent connections to a previously seen peer. This is especially important for DHT requests that contact many new peers.
* Because it's UDP based instead of TCP based, it uses fewer file descriptors. The QUIC transport will open one UDP socket per listen address instead of one socket per connection. This should, in the future, allow us to keep more connections open.
* Because QUIC connections don't consume file descriptors, we're able to remove the rate limit on outbound QUIC connections, further speeding up DHT queries.

Unfortunately, this change isn't without drawbacks: the QUIC transport may not be able to max out some links (usually due to [poorly tuned kernel parameters](https://github.com/lucas-clemente/quic-go/issues/2586#issuecomment-639247615)). On the other hand, it may also be _faster_ in some cases

If you hit this performance issue on Linux, you should tune the `net.core.rmem_default` and `net.core.rmem_max` sysctl parameters to increase your UDP receive buffer sizes.

If necessary, you can disable the QUIC transport by running:

```bash
> ipfs config --json Swarm.Transports.Network.QUIC false
```

**NOTE:** The QUIC transport included in this release is backwards incompatible with the experimental QUIC transport included in previous releases. Unfortunately, the QUIC protocol underwent some significant breaking changes and supporting multiple versions wasn't an option. In practice this degrades gracefully as go-ipfs will simply fall back on the TCP transport when dialing nodes with incompatible QUIC versions.

#### Noise Transport

This go-ipfs release introduces a new security transport: [libp2p Noise](https://github.com/libp2p/specs/tree/master/noise) (built from the [Noise Protocol Framework](http://www.noiseprotocol.org/)). While TLS1.3 remains the default go-ipfs security transport, Noise is simpler to implement from scratch and will be the standard cross-platform libp2p security transport going forward.

This brings us one step closer to deprecating and removing support for SECIO.

While enabled by default, Noise won't actually be _used_ by default it's negotiated. Given that TLS1.3 is still the default security transport for go-ipfs, this usually won't happen. If you'd like to prefer Noise over other security transports, you can change its priority in the [config](./docs/config.md) (`Swarm.Transports.Security.Noise`).

#### Gateway

This release brings two gateway-relevant features: custom 404 pages and base36 support.

##### Custom 404

You can now customize `404 Not Found` error pages by including an `ipfs-404.html` file somewhere in the request path. When a requested file isn't found, go-ipfs will look for an `ipfs-404.html` in the same directory as the requested file, and in each ancestor directory. If found, this file will be returned (with a 404 status code) instead of the usual error message.

##### Support for Base36

This release adds support for a new multibase encoding: base36. Base36 is an optimally efficient case-insensitive alphanumeric encoding. Case-insensitive alphanumeric encodings are important for the subdomain gateway as domain names are case insensitive.

While base32 (the current default encoding used in subdomains) is simpler than base36, it's not optimally efficient and base36 Ed25519 IPNS keys are 2 characters too big to fit into the 63 character subdomain length limit. The extra efficiency from base36 brings us under this limit and allows Ed25519 IPNS keys to work with the subdomain gateway.

This release adds support for base36 but won't use it by default. If you'd like to re-encode an Ed25519 IPNS key into base36, you can use the `ipfs cid format` command:

```sh
$ ipfs cid format -v 1 --codec libp2p-key -b base36 bafzaajaiaejca4syrpdu6gdx4wsdnokxkprgzxf4wrstuc34gxw5k5jrag2so5gk k51qzi5uqu5dj16qyiq0tajolkojyl9qdkr254920wxv7ghtuwcz593tp69z9m
```

#### Gossipsub Upgrade

This release brings a new gossipsub protocol version: 1.1. You can read about it in the [blog post](https://blog.ipfs.io/2020-05-20-gossipsub-v1.1/).

#### Connectivity

This release introduces a new ["peering"](./docs/config.md#peering) feature. The peering subsystem configures go-ipfs to connect to, remain connected to, and reconnect to a set of nodes. Nodes should use this subsystem to create "sticky" links between frequently useful peers to improve reliability.

Use-cases:

* An IPFS gateway connected to an IPFS cluster should peer to ensure that the gateway can always fetch content from the cluster.
* A dapp may peer embedded go-ipfs nodes with a set of pinning services or textile cafes/hubs.
* A set of friends may peer to ensure that they can always fetch each other's content.

### Changelog

- github.com/ipfs/go-ipfs:
  - fix 3 bugs responsible for a goroutine leak (plus one other bug) ([ipfs/go-ipfs#7491](https://github.com/ipfs/go-ipfs/pull/7491))
  - docs(config): update toc ([ipfs/go-ipfs#7483](https://github.com/ipfs/go-ipfs/pull/7483))
  - feat: transport config ([ipfs/go-ipfs#7479](https://github.com/ipfs/go-ipfs/pull/7479))
  - fix the minimal go version under 'Build from Source' ([ipfs/go-ipfs#7459](https://github.com/ipfs/go-ipfs/pull/7459))
  - fix(migration): migrate /ipfs/ bootstrappers to /p2p/
  - fix(migration): correctly migrate quic addresses
  - chore: add migration to listen on QUIC by default
  - backport fixes ([ipfs/go-ipfs#7405](https://github.com/ipfs/go-ipfs/pull/7405))
    - Use bitswap sessions for `ipfs refs`.
    - Update to webui 2.9.0
  - feat: add noise support ([ipfs/go-ipfs#7365](https://github.com/ipfs/go-ipfs/pull/7365))
  - feat: implement peering service ([ipfs/go-ipfs#7362](https://github.com/ipfs/go-ipfs/pull/7362))
  - Include the git blob id of the dir-index bundle in the ETag ([ipfs/go-ipfs#7360](https://github.com/ipfs/go-ipfs/pull/7360))
  - feat: bootstrap in dht when the routing table is empty ([ipfs/go-ipfs#7340](https://github.com/ipfs/go-ipfs/pull/7340))
  - quic: remove experimental status and add it to the default config ([ipfs/go-ipfs#7349](https://github.com/ipfs/go-ipfs/pull/7349))
  - fix: support directory listings even if a 404 page is present ([ipfs/go-ipfs#7339](https://github.com/ipfs/go-ipfs/pull/7339))
  - doc(plugin): document plugin config ([ipfs/go-ipfs#7309](https://github.com/ipfs/go-ipfs/pull/7309))
  - test(sharness): fix fuse tests ([ipfs/go-ipfs#7320](https://github.com/ipfs/go-ipfs/pull/7320))
  - docs: update experimental-features doc with IPNS over pubsub changes. ([ipfs/go-ipfs#7334](https://github.com/ipfs/go-ipfs/pull/7334))
  - docs: cleanup config formatting ([ipfs/go-ipfs#7336](https://github.com/ipfs/go-ipfs/pull/7336))
  - fix(gateway): ensure directory listings have Content-Type text/html ([ipfs/go-ipfs#7330](https://github.com/ipfs/go-ipfs/pull/7330))
  - test(sharness): test the local symlink ([ipfs/go-ipfs#7332](https://github.com/ipfs/go-ipfs/pull/7332))
  - misc config/experimental-features doc fixes ([ipfs/go-ipfs#7333](https://github.com/ipfs/go-ipfs/pull/7333))
  - fix: correctly trim resolved IPNS addresses ([ipfs/go-ipfs#7331](https://github.com/ipfs/go-ipfs/pull/7331))
  - Gateway renders pretty 404 pages if available ([ipfs/go-ipfs#4233](https://github.com/ipfs/go-ipfs/pull/4233))
  - feat: add a dht stat command ([ipfs/go-ipfs#7221](https://github.com/ipfs/go-ipfs/pull/7221))
  - fix: update dists url for OpenBSD support ([ipfs/go-ipfs#7311](https://github.com/ipfs/go-ipfs/pull/7311))
  - docs: X-Forwarded-Proto: https ([ipfs/go-ipfs#7306](https://github.com/ipfs/go-ipfs/pull/7306))
  - fix(mkreleaselog): make robust against running in different working directories ([ipfs/go-ipfs#7310](https://github.com/ipfs/go-ipfs/pull/7310))
  - fix(mkreleasenotes): include commits directly to master ([ipfs/go-ipfs#7296](https://github.com/ipfs/go-ipfs/pull/7296))
  - write api file automically ([ipfs/go-ipfs#7282](https://github.com/ipfs/go-ipfs/pull/7282))
  - systemd: disable swap-usage for ipfs ([ipfs/go-ipfs#7299](https://github.com/ipfs/go-ipfs/pull/7299))
  - systemd: add helptext ([ipfs/go-ipfs#7265](https://github.com/ipfs/go-ipfs/pull/7265))
  - systemd: add the link to the docs ([ipfs/go-ipfs#7287](https://github.com/ipfs/go-ipfs/pull/7287))
  - systemd: add state directory setting ([ipfs/go-ipfs#7288](https://github.com/ipfs/go-ipfs/pull/7288))
  - Update go version required to build ([ipfs/go-ipfs#7289](https://github.com/ipfs/go-ipfs/pull/7289))
  - pin: implement pin/ls with only CoreApi ([ipfs/go-ipfs#6774](https://github.com/ipfs/go-ipfs/pull/6774))
  - update go-libp2p-quic-transport to v0.3.7 ([ipfs/go-ipfs#7278](https://github.com/ipfs/go-ipfs/pull/7278))
  - Docs: Delete section headers for removed features ([ipfs/go-ipfs#7277](https://github.com/ipfs/go-ipfs/pull/7277))
  - README.md: typo ([ipfs/go-ipfs#7061](https://github.com/ipfs/go-ipfs/pull/7061))
  - PR autocomment: Only comment for first-time contributors ([ipfs/go-ipfs#7270](https://github.com/ipfs/go-ipfs/pull/7270))
  - Fixed typo in config.md ([ipfs/go-ipfs#7267](https://github.com/ipfs/go-ipfs/pull/7267))
  - Fixes #7252 - Uses gabriel-vasile/mimetype to support additional content types ([ipfs/go-ipfs#7262](https://github.com/ipfs/go-ipfs/pull/7262))
  - update go-libp2p-quic-transport to v0.3.6 ([ipfs/go-ipfs#7266](https://github.com/ipfs/go-ipfs/pull/7266))
  - Updates bash completions to be compatible with zsh ([ipfs/go-ipfs#7261](https://github.com/ipfs/go-ipfs/pull/7261))
  - systemd service enhancements + run as system user ([ipfs/go-ipfs#7259](https://github.com/ipfs/go-ipfs/pull/7259))
  - upgrade to go 1.14.2 ([ipfs/go-ipfs#7130](https://github.com/ipfs/go-ipfs/pull/7130))
  - Add module files for go-ipfs-as-a-library example ([ipfs/go-ipfs#7146](https://github.com/ipfs/go-ipfs/pull/7146))
  - feat(gateway): show the absolute path and CID every time ([ipfs/go-ipfs#7219](https://github.com/ipfs/go-ipfs/pull/7219))
  - fix: do not use hard coded IPNS Publish maximum timeout duration ([ipfs/go-ipfs#7256](https://github.com/ipfs/go-ipfs/pull/7256))
  - Auto-comment on submitted PRs ([ipfs/go-ipfs#7248](https://github.com/ipfs/go-ipfs/pull/7248))
  - Fixes Github link. ([ipfs/go-ipfs#7239](https://github.com/ipfs/go-ipfs/pull/7239))
  - docs: fix subdomain examples in CHANGELOG ([ipfs/go-ipfs#7240](https://github.com/ipfs/go-ipfs/pull/7240))
  - doc: add snap to the release checklist ([ipfs/go-ipfs#7253](https://github.com/ipfs/go-ipfs/pull/7253))
  - Welcome message for users opening their first issue ([ipfs/go-ipfs#7247](https://github.com/ipfs/go-ipfs/pull/7247))
  - feat: bump to 0.6.0-dev ([ipfs/go-ipfs#7249](https://github.com/ipfs/go-ipfs/pull/7249))
- github.com/ipfs/go-bitswap (v0.2.13 -> v0.2.19):
  - fix want gauge calculation ([ipfs/go-bitswap#416](https://github.com/ipfs/go-bitswap/pull/416))
  - Fix PeerManager signalAvailabiity() race ([ipfs/go-bitswap#417](https://github.com/ipfs/go-bitswap/pull/417))
  - fix: avoid taking accessing the peerQueues without taking the lock ([ipfs/go-bitswap#412](https://github.com/ipfs/go-bitswap/pull/412))
  - fix: update circleci ci-go ([ipfs/go-bitswap#396](https://github.com/ipfs/go-bitswap/pull/396))
  - fix: only track useful received data in the ledger (#411) ([ipfs/go-bitswap#411](https://github.com/ipfs/go-bitswap/pull/411))
  - If peer is first to send a block to session, protect connection ([ipfs/go-bitswap#406](https://github.com/ipfs/go-bitswap/pull/406))
  - Ensure sessions register with PeerManager ([ipfs/go-bitswap#405](https://github.com/ipfs/go-bitswap/pull/405))
  - Total wants gauge (#402) ([ipfs/go-bitswap#402](https://github.com/ipfs/go-bitswap/pull/402))
  - Improve peer manager performance ([ipfs/go-bitswap#395](https://github.com/ipfs/go-bitswap/pull/395))
  - fix: return wants from engine.WantlistForPeer() ([ipfs/go-bitswap#390](https://github.com/ipfs/go-bitswap/pull/390))
  - Add autocomment configuration
  - calculate message latency ([ipfs/go-bitswap#386](https://github.com/ipfs/go-bitswap/pull/386))
  - fix: use one less go-routine per session (#377) ([ipfs/go-bitswap#377](https://github.com/ipfs/go-bitswap/pull/377))
  - Add standard issue template
- github.com/ipfs/go-cid (v0.0.5 -> v0.0.6):
  - feat: add Filecoin multicodecs ([ipfs/go-cid#104](https://github.com/ipfs/go-cid/pull/104))
  - Add autocomment configuration
  - avoid calling the method WriteTo if we don't satisfy its contract ([ipfs/go-cid#103](https://github.com/ipfs/go-cid/pull/103))
  - add a couple useful methods ([ipfs/go-cid#102](https://github.com/ipfs/go-cid/pull/102))
  - Add standard issue template
- github.com/ipfs/go-fs-lock (v0.0.4 -> v0.0.5):
  - chore: remove xerrors ([ipfs/go-fs-lock#15](https://github.com/ipfs/go-fs-lock/pull/15))
  - Add autocomment configuration
  - Add standard issue template
- github.com/ipfs/go-ipfs-cmds (v0.2.2 -> v0.2.9):
  - build(deps): bump github.com/ipfs/go-log from 1.0.3 to 1.0.4 ([ipfs/go-ipfs-cmds#194](https://github.com/ipfs/go-ipfs-cmds/pull/194))
  - Fix go-ipfs#7242: Remove "HEAD" from Allow methods ([ipfs/go-ipfs-cmds#195](https://github.com/ipfs/go-ipfs-cmds/pull/195))
  - Staticcheck fixes (#196) ([ipfs/go-ipfs-cmds#196](https://github.com/ipfs/go-ipfs-cmds/pull/196))
  - doc: update docs for interface changes ([ipfs/go-ipfs-cmds#197](https://github.com/ipfs/go-ipfs-cmds/pull/197))
  - Add standard issue template
- github.com/ipfs/go-ipfs-config (v0.5.3 -> v0.8.0):
  - feat: add a transports section for enabling/disabling transports ([ipfs/go-ipfs-config#102](https://github.com/ipfs/go-ipfs-config/pull/102))
  - feat: add an option for security transport experiments ([ipfs/go-ipfs-config#97](https://github.com/ipfs/go-ipfs-config/pull/97))
  - feat: add peering service config section ([ipfs/go-ipfs-config#96](https://github.com/ipfs/go-ipfs-config/pull/96))
  - fix: include key size in key init method ([ipfs/go-ipfs-config#95](https://github.com/ipfs/go-ipfs-config/pull/95))
  - QUIC: remove experimental config option ([ipfs/go-ipfs-config#93](https://github.com/ipfs/go-ipfs-config/pull/93))
  - fix boostrap peers ([ipfs/go-ipfs-config#94](https://github.com/ipfs/go-ipfs-config/pull/94))
  - default config: add QUIC listening ports + quic to mars.i.ipfs.io ([ipfs/go-ipfs-config#91](https://github.com/ipfs/go-ipfs-config/pull/91))
  - feat: remove strict signing pubsub option. ([ipfs/go-ipfs-config#90](https://github.com/ipfs/go-ipfs-config/pull/90))
  - Add autocomment configuration
  - Add Init Alternative allowing specification of ED25519 key ([ipfs/go-ipfs-config#78](https://github.com/ipfs/go-ipfs-config/pull/78))
- github.com/ipfs/go-mfs (v0.1.1 -> v0.1.2):
  - Fix incorrect mutex unlock call in File.Open ([ipfs/go-mfs#82](https://github.com/ipfs/go-mfs/pull/82))
  - Add autocomment configuration
  - Add standard issue template
  - test: add Directory.ListNames test ([ipfs/go-mfs#81](https://github.com/ipfs/go-mfs/pull/81))
  - doc: add a lead maintainer
  - Update README.md with newer travis badge ([ipfs/go-mfs#78](https://github.com/ipfs/go-mfs/pull/78))
- github.com/ipfs/interface-go-ipfs-core (v0.2.7 -> v0.3.0):
  - add Pin.IsPinned(..) ([ipfs/interface-go-ipfs-core#50](https://github.com/ipfs/interface-go-ipfs-core/pull/50))
  - Add autocomment configuration
  - Add standard issue template
  - extra time for dht spin-up ([ipfs/interface-go-ipfs-core#61](https://github.com/ipfs/interface-go-ipfs-core/pull/61))
  - feat: make the CoreAPI expose a streaming pin interface ([ipfs/interface-go-ipfs-core#49](https://github.com/ipfs/interface-go-ipfs-core/pull/49))
  - test: fail early on err to avoid an unrelated panic ([ipfs/interface-go-ipfs-core#57](https://github.com/ipfs/interface-go-ipfs-core/pull/57))
- github.com/jbenet/go-is-domain (v1.0.3 -> v1.0.5):
  - Add OpenNIC domains to extended TLDs. ([jbenet/go-is-domain#15](https://github.com/jbenet/go-is-domain/pull/15))
  - feat: add .crypto and .zil from UnstoppableDomains ([jbenet/go-is-domain#17](https://github.com/jbenet/go-is-domain/pull/17))
  - chore: update IANA TLDs to version 2020051300 ([jbenet/go-is-domain#18](https://github.com/jbenet/go-is-domain/pull/18))
- github.com/libp2p/go-addr-util (v0.0.1 -> v0.0.2):
  - fix discuss badge
  - add discuss link to readme
  - fix: fdcostly should take only the prefix into account ([libp2p/go-addr-util#5](https://github.com/libp2p/go-addr-util/pull/5))
  - add gomod support // tag v0.0.1 ([libp2p/go-addr-util#17](https://github.com/libp2p/go-addr-util/pull/17))
- github.com/libp2p/go-libp2p (v0.8.3 -> v0.9.6):
  - fix(nat): use the right addresses when nat port mapping ([libp2p/go-libp2p#966](https://github.com/libp2p/go-libp2p/pull/966))
  - chore: update deps ([libp2p/go-libp2p#967](https://github.com/libp2p/go-libp2p/pull/967))
  - Fix peer handler race ([libp2p/go-libp2p#965](https://github.com/libp2p/go-libp2p/pull/965))
  - optimize numInbound count ([libp2p/go-libp2p#960](https://github.com/libp2p/go-libp2p/pull/960))
  - update go-libp2p-circuit ([libp2p/go-libp2p#962](https://github.com/libp2p/go-libp2p/pull/962))
  - Chunking large Identify responses with Signed Records ([libp2p/go-libp2p#958](https://github.com/libp2p/go-libp2p/pull/958))
  - gomod: update dependencies ([libp2p/go-libp2p#959](https://github.com/libp2p/go-libp2p/pull/959))
  - fixed compilation error (#956) ([libp2p/go-libp2p#956](https://github.com/libp2p/go-libp2p/pull/956))
  - Filter Interface Addresses (#936) ([libp2p/go-libp2p#936](https://github.com/libp2p/go-libp2p/pull/936))
  - fix: remove old addresses in identify immediately ([libp2p/go-libp2p#953](https://github.com/libp2p/go-libp2p/pull/953))
  - fix flaky test (#952) ([libp2p/go-libp2p#952](https://github.com/libp2p/go-libp2p/pull/952))
  - fix: group observations by zeroing port ([libp2p/go-libp2p#949](https://github.com/libp2p/go-libp2p/pull/949))
  - fix: fix connection gater in transport constructor ([libp2p/go-libp2p#948](https://github.com/libp2p/go-libp2p/pull/948))
  - Fix potential flakiness in TestIDService ([libp2p/go-libp2p#945](https://github.com/libp2p/go-libp2p/pull/945))
  - make the {F=>f}iltersConnectionGater private. (#946) ([libp2p/go-libp2p#946](https://github.com/libp2p/go-libp2p/pull/946))
  - Filter observed addresses (#917) ([libp2p/go-libp2p#917](https://github.com/libp2p/go-libp2p/pull/917))
  - fix: don't try to marshal a nil record ([libp2p/go-libp2p#943](https://github.com/libp2p/go-libp2p/pull/943))
  - add test to demo missing peer records after listen ([libp2p/go-libp2p#941](https://github.com/libp2p/go-libp2p/pull/941))
  - fix: don't leak a goroutine if a peer connects and immediately disconnects ([libp2p/go-libp2p#942](https://github.com/libp2p/go-libp2p/pull/942))
  - no signed peer records for mocknets (#934) ([libp2p/go-libp2p#934](https://github.com/libp2p/go-libp2p/pull/934))
  - implement connection gating at the top level (#881) ([libp2p/go-libp2p#881](https://github.com/libp2p/go-libp2p/pull/881))
  - various identify fixes and nits (#922) ([libp2p/go-libp2p#922](https://github.com/libp2p/go-libp2p/pull/922))
  - Remove race between ID, Push & Delta (#907) ([libp2p/go-libp2p#907](https://github.com/libp2p/go-libp2p/pull/907))
  - fix a compilation error introduced in 077a818. (#919) ([libp2p/go-libp2p#919](https://github.com/libp2p/go-libp2p/pull/919))
  - exchange signed routing records in identify (#747) ([libp2p/go-libp2p#747](https://github.com/libp2p/go-libp2p/pull/747))
- github.com/libp2p/go-libp2p-autonat (v0.2.2 -> v0.2.3):
  - react to incoming events ([libp2p/go-libp2p-autonat#65](https://github.com/libp2p/go-libp2p-autonat/pull/65))
- github.com/libp2p/go-libp2p-blankhost (v0.1.4 -> v0.1.6):
  - subscribe connmgr to net notifications ([libp2p/go-libp2p-blankhost#45](https://github.com/libp2p/go-libp2p-blankhost/pull/45))
  - add WithConnectionManager option to blankhost ([libp2p/go-libp2p-blankhost#44](https://github.com/libp2p/go-libp2p-blankhost/pull/44))
  - Blank host should support signed records ([libp2p/go-libp2p-blankhost#42](https://github.com/libp2p/go-libp2p-blankhost/pull/42))
- github.com/libp2p/go-libp2p-circuit (v0.2.2 -> v0.2.3):
  - Use a fixed connection manager weight for peers with relay connections ([libp2p/go-libp2p-circuit#119](https://github.com/libp2p/go-libp2p-circuit/pull/119))
- github.com/libp2p/go-libp2p-connmgr (v0.2.1 -> v0.2.4):
  - Implement IsProtected interface ([libp2p/go-libp2p-connmgr#76](https://github.com/libp2p/go-libp2p-connmgr/pull/76))
  - decaying tags: support removal and closure. (#72) ([libp2p/go-libp2p-connmgr#72](https://github.com/libp2p/go-libp2p-connmgr/pull/72))
  - implement decaying tags. (#61) ([libp2p/go-libp2p-connmgr#61](https://github.com/libp2p/go-libp2p-connmgr/pull/61))
- github.com/libp2p/go-libp2p-core (v0.5.3 -> v0.5.7):
  - connmgr: add IsProtected interface (#158) ([libp2p/go-libp2p-core#158](https://github.com/libp2p/go-libp2p-core/pull/158))
  - eventbus: add wildcard subscription type; getter to enumerate known types (#153) ([libp2p/go-libp2p-core#153](https://github.com/libp2p/go-libp2p-core/pull/153))
  - events: add a generic DHT event. (#154) ([libp2p/go-libp2p-core#154](https://github.com/libp2p/go-libp2p-core/pull/154))
  - decaying tags: support removal and closure. (#151) ([libp2p/go-libp2p-core#151](https://github.com/libp2p/go-libp2p-core/pull/151))
  - implement Stringer for network.{Direction,Connectedness,Reachability}. (#150) ([libp2p/go-libp2p-core#150](https://github.com/libp2p/go-libp2p-core/pull/150))
  - connmgr: introduce abstractions and functions for decaying tags. (#104) ([libp2p/go-libp2p-core#104](https://github.com/libp2p/go-libp2p-core/pull/104))
  - Interface to verify if a peer supports a protocol without making allocations. ([libp2p/go-libp2p-core#148](https://github.com/libp2p/go-libp2p-core/pull/148))
  - add connection gating interfaces and types. (#139) ([libp2p/go-libp2p-core#139](https://github.com/libp2p/go-libp2p-core/pull/139))
- github.com/libp2p/go-libp2p-kad-dht (v0.7.11 -> v0.8.2):
  - feat: protect all peers in low buckets, tag everyone else with 5
  - fix: lookup context cancellation race condition ([libp2p/go-libp2p-kad-dht#656](https://github.com/libp2p/go-libp2p-kad-dht/pull/656))
  - fix: protect useful peers in low buckets ([libp2p/go-libp2p-kad-dht#634](https://github.com/libp2p/go-libp2p-kad-dht/pull/634))
  - Double the usefulness interval for peers in the Routing Table (#651) ([libp2p/go-libp2p-kad-dht#651](https://github.com/libp2p/go-libp2p-kad-dht/pull/651))
  - enhancement/remove-unused-variable ([libp2p/go-libp2p-kad-dht#633](https://github.com/libp2p/go-libp2p-kad-dht/pull/633))
  - Put back TestSelfWalkOnAddressChange ([libp2p/go-libp2p-kad-dht#648](https://github.com/libp2p/go-libp2p-kad-dht/pull/648))
  - Routing Table Refresh manager (#601) ([libp2p/go-libp2p-kad-dht#601](https://github.com/libp2p/go-libp2p-kad-dht/pull/601))
  - Boostrap empty RT and Optimize allocs when we discover new peers (#631) ([libp2p/go-libp2p-kad-dht#631](https://github.com/libp2p/go-libp2p-kad-dht/pull/631))
  - fix all flaky tests ([libp2p/go-libp2p-kad-dht#628](https://github.com/libp2p/go-libp2p-kad-dht/pull/628))
  - Update default concurrency parameter ([libp2p/go-libp2p-kad-dht#605](https://github.com/libp2p/go-libp2p-kad-dht/pull/605))
  - clean up a channel that was dangling ([libp2p/go-libp2p-kad-dht#620](https://github.com/libp2p/go-libp2p-kad-dht/pull/620))
- github.com/libp2p/go-libp2p-kbucket (v0.4.1 -> v0.4.2):
  - Reduce allocs in AddPeer (#81) ([libp2p/go-libp2p-kbucket#81](https://github.com/libp2p/go-libp2p-kbucket/pull/81))
  - NPeersForCpl and collapse empty buckets (#77) ([libp2p/go-libp2p-kbucket#77](https://github.com/libp2p/go-libp2p-kbucket/pull/77))
- github.com/libp2p/go-libp2p-peerstore (v0.2.3 -> v0.2.6):
  - fix two bugs in signed address handling ([libp2p/go-libp2p-peerstore#155](https://github.com/libp2p/go-libp2p-peerstore/pull/155))
  - addrbook: fix races ([libp2p/go-libp2p-peerstore#154](https://github.com/libp2p/go-libp2p-peerstore/pull/154))
  - Implement the FirstSupportedProtocol API. ([libp2p/go-libp2p-peerstore#147](https://github.com/libp2p/go-libp2p-peerstore/pull/147))
- github.com/libp2p/go-libp2p-pubsub (v0.2.7 -> v0.3.1):
  - fix outbound constraint satisfaction in oversubscription pruning
  - Gossipsub v0.3.0
  - set sendTo to remote peer id in trace events ([libp2p/go-libp2p-pubsub#268](https://github.com/libp2p/go-libp2p-pubsub/pull/268))
  - make wire protocol message size configurable. (#261) ([libp2p/go-libp2p-pubsub#261](https://github.com/libp2p/go-libp2p-pubsub/pull/261))
- github.com/libp2p/go-libp2p-pubsub-router (v0.2.1 -> v0.3.0):
  - feat: update pubsub ([libp2p/go-libp2p-pubsub-router#76](https://github.com/libp2p/go-libp2p-pubsub-router/pull/76))
- github.com/libp2p/go-libp2p-quic-transport (v0.3.7 -> v0.5.1):
  - close the connection when it is refused by InterceptSecured ([libp2p/go-libp2p-quic-transport#157](https://github.com/libp2p/go-libp2p-quic-transport/pull/157))
  - gate QUIC connections via new ConnectionGater (#152) ([libp2p/go-libp2p-quic-transport#152](https://github.com/libp2p/go-libp2p-quic-transport/pull/152))
- github.com/libp2p/go-libp2p-record (v0.1.2 -> v0.1.3):
  - feat: add a better record error ([libp2p/go-libp2p-record#39](https://github.com/libp2p/go-libp2p-record/pull/39))
- github.com/libp2p/go-libp2p-swarm (v0.2.3 -> v0.2.6):
  - Configure private key for test swarm ([libp2p/go-libp2p-swarm#223](https://github.com/libp2p/go-libp2p-swarm/pull/223))
  - Rank Dial addresses (#212) ([libp2p/go-libp2p-swarm#212](https://github.com/libp2p/go-libp2p-swarm/pull/212))
  - implement connection gating support: intercept peer, address dials, upgraded conns (#201) ([libp2p/go-libp2p-swarm#201](https://github.com/libp2p/go-libp2p-swarm/pull/201))
  - fix: avoid calling AddChild after the process may shutdown. ([libp2p/go-libp2p-swarm#207](https://github.com/libp2p/go-libp2p-swarm/pull/207))
- github.com/libp2p/go-libp2p-transport-upgrader (v0.2.0 -> v0.3.0):
  - call the connection gater when accepting connections and after crypto handshake (#55) ([libp2p/go-libp2p-transport-upgrader#55](https://github.com/libp2p/go-libp2p-transport-upgrader/pull/55))
- github.com/libp2p/go-openssl (v0.0.4 -> v0.0.5):
  - add binding for OBJ_create ([libp2p/go-openssl#5](https://github.com/libp2p/go-openssl/pull/5))
- github.com/libp2p/go-yamux (v1.3.5 -> v1.3.7):
  - tighten lock around appending new chunks of read data in stream ([libp2p/go-yamux#28](https://github.com/libp2p/go-yamux/pull/28))
  - fix: unlock recvLock in all cases. ([libp2p/go-yamux#25](https://github.com/libp2p/go-yamux/pull/25))
- github.com/lucas-clemente/quic-go (v0.15.7 -> v0.16.2):
  - make it possible to use the transport with both draft-28 and draft-29
  - update the ALPN for draft-29 ([lucas-clemente/quic-go#2600](https://github.com/lucas-clemente/quic-go/pull/2600))
  - update initial salts and test vectors for draft-29 ([lucas-clemente/quic-go#2587](https://github.com/lucas-clemente/quic-go/pull/2587))
  - rename the SERVER_BUSY error to CONNECTION_REFUSED ([lucas-clemente/quic-go#2596](https://github.com/lucas-clemente/quic-go/pull/2596))
  - reduce calls to time.Now() from the flow controller ([lucas-clemente/quic-go#2591](https://github.com/lucas-clemente/quic-go/pull/2591))
  - remove redundant parenthesis and type conversion in flow controller ([lucas-clemente/quic-go#2592](https://github.com/lucas-clemente/quic-go/pull/2592))
  - use the receipt of a Retry packet to get a first RTT estimate ([lucas-clemente/quic-go#2588](https://github.com/lucas-clemente/quic-go/pull/2588))
  - fix debug message when returning an early session ([lucas-clemente/quic-go#2594](https://github.com/lucas-clemente/quic-go/pull/2594))
  - fix closing of the http.Request.Body ([lucas-clemente/quic-go#2584](https://github.com/lucas-clemente/quic-go/pull/2584))
  - split PTO calculation into a separate function ([lucas-clemente/quic-go#2576](https://github.com/lucas-clemente/quic-go/pull/2576))
  - add a unit test using the ChaCha20 test vector from the draft ([lucas-clemente/quic-go#2585](https://github.com/lucas-clemente/quic-go/pull/2585))
  - fix seed generation in frame sorter tests ([lucas-clemente/quic-go#2583](https://github.com/lucas-clemente/quic-go/pull/2583))
  - make sure that ACK frames are bundled with data ([lucas-clemente/quic-go#2543](https://github.com/lucas-clemente/quic-go/pull/2543))
  - add a Changelog for v0.16 ([lucas-clemente/quic-go#2582](https://github.com/lucas-clemente/quic-go/pull/2582))
  - authenticate connection IDs ([lucas-clemente/quic-go#2567](https://github.com/lucas-clemente/quic-go/pull/2567))
  - don't switch to PTO mode after using early loss detection ([lucas-clemente/quic-go#2581](https://github.com/lucas-clemente/quic-go/pull/2581))
  - only create a single session for duplicate Initials ([lucas-clemente/quic-go#2580](https://github.com/lucas-clemente/quic-go/pull/2580))
  - fix broken unit test in ackhandler
  - update the ALPN tokens to draft-28 ([lucas-clemente/quic-go#2570](https://github.com/lucas-clemente/quic-go/pull/2570))
  - drop duplicate packets ([lucas-clemente/quic-go#2569](https://github.com/lucas-clemente/quic-go/pull/2569))
  - remove noisy log statement in frame sorter test ([lucas-clemente/quic-go#2571](https://github.com/lucas-clemente/quic-go/pull/2571))
  - fix flaky qlog unit tests ([lucas-clemente/quic-go#2572](https://github.com/lucas-clemente/quic-go/pull/2572))
  - implement the 3x amplification limit ([lucas-clemente/quic-go#2536](https://github.com/lucas-clemente/quic-go/pull/2536))
  - rewrite the frame sorter ([lucas-clemente/quic-go#2561](https://github.com/lucas-clemente/quic-go/pull/2561))
  - retire conn IDs with sequence numbers smaller than the currently active ([lucas-clemente/quic-go#2563](https://github.com/lucas-clemente/quic-go/pull/2563))
  - remove unused readOffset member variable in receiveStream ([lucas-clemente/quic-go#2559](https://github.com/lucas-clemente/quic-go/pull/2559))
  - fix int overflow when parsing the transport parameters ([lucas-clemente/quic-go#2564](https://github.com/lucas-clemente/quic-go/pull/2564))
  - use struct{} instead of bool in window update queue ([lucas-clemente/quic-go#2555](https://github.com/lucas-clemente/quic-go/pull/2555))
  - update the protobuf library to google.golang.org/protobuf/proto ([lucas-clemente/quic-go#2554](https://github.com/lucas-clemente/quic-go/pull/2554))
  - use the correct error code for crypto stream errors ([lucas-clemente/quic-go#2546](https://github.com/lucas-clemente/quic-go/pull/2546))
  - bundle small writes on streams ([lucas-clemente/quic-go#2538](https://github.com/lucas-clemente/quic-go/pull/2538))
  - reduce the length of the unprocessed packet chan in the session ([lucas-clemente/quic-go#2534](https://github.com/lucas-clemente/quic-go/pull/2534))
  - fix flaky session unit test ([lucas-clemente/quic-go#2537](https://github.com/lucas-clemente/quic-go/pull/2537))
  - add a send stream test that randomly acknowledges and loses data ([lucas-clemente/quic-go#2535](https://github.com/lucas-clemente/quic-go/pull/2535))
  - fix size calculation for version negotiation packets ([lucas-clemente/quic-go#2542](https://github.com/lucas-clemente/quic-go/pull/2542))
  - run all unit tests with race detector ([lucas-clemente/quic-go#2528](https://github.com/lucas-clemente/quic-go/pull/2528))
  - add support for the ChaCha20 interop test case ([lucas-clemente/quic-go#2517](https://github.com/lucas-clemente/quic-go/pull/2517))
  - fix buffer use after it was released when sending an INVALID_TOKEN error ([lucas-clemente/quic-go#2524](https://github.com/lucas-clemente/quic-go/pull/2524))
  - run the internal and http3 tests with race detector on Travis ([lucas-clemente/quic-go#2385](https://github.com/lucas-clemente/quic-go/pull/2385))
  - reset the PTO when dropping a packet number space ([lucas-clemente/quic-go#2527](https://github.com/lucas-clemente/quic-go/pull/2527))
  - stop the deadline timer in Stream.Read and Write ([lucas-clemente/quic-go#2519](https://github.com/lucas-clemente/quic-go/pull/2519))
  - don't reset pto_count on Initial ACKs ([lucas-clemente/quic-go#2513](https://github.com/lucas-clemente/quic-go/pull/2513))
  - fix all race conditions in the session tests ([lucas-clemente/quic-go#2525](https://github.com/lucas-clemente/quic-go/pull/2525))
  - make sure that the server's run loop returned when closing ([lucas-clemente/quic-go#2526](https://github.com/lucas-clemente/quic-go/pull/2526))
  - fix flaky proxy test ([lucas-clemente/quic-go#2522](https://github.com/lucas-clemente/quic-go/pull/2522))
  - stop the timer when the session's run loop returns ([lucas-clemente/quic-go#2516](https://github.com/lucas-clemente/quic-go/pull/2516))
  - make it more likely that a STREAM frame is bundled with the FIN ([lucas-clemente/quic-go#2504](https://github.com/lucas-clemente/quic-go/pull/2504))
- github.com/multiformats/go-multiaddr (v0.2.1 -> v0.2.2):
  - absorb go-maddr-filter; rm stale Makefile targets; upgrade deps (#124) ([multiformats/go-multiaddr#124](https://github.com/multiformats/go-multiaddr/pull/124))
- github.com/multiformats/go-multibase (v0.0.2 -> v0.0.3):
  - Base36 implementation ([multiformats/go-multibase#36](https://github.com/multiformats/go-multibase/pull/36))
  - Even more tests/benchmarks, less repetition in-code ([multiformats/go-multibase#34](https://github.com/multiformats/go-multibase/pull/34))
  - Beef up tests before adding new codec ([multiformats/go-multibase#32](https://github.com/multiformats/go-multibase/pull/32))
  - Remove GX, bump spec submodule, fix tests ([multiformats/go-multibase#31](https://github.com/multiformats/go-multibase/pull/31))

### Contributors

| Contributor             | Commits | Lines ±     | Files Changed |
|-------------------------|---------|-------------|---------------|
| vyzo                    | 224     | +8016/-2810 | 304           |
| Marten Seemann          | 87      | +6081/-2607 | 215           |
| Steven Allen            | 157     | +4763/-1628 | 266           |
| Aarsh Shah              | 33      | +4619/-1634 | 128           |
| Dirk McCormick          | 26      | +3596/-1156 | 69            |
| Yusef Napora            | 66      | +2622/-785  | 98            |
| Raúl Kripalani          | 24      | +2424/-782  | 61            |
| Hector Sanjuan          | 30      | +999/-177   | 61            |
| Louis Thibault          | 2       | +1111/-4    | 4             |
| Will Scott              | 15      | +717/-219   | 31            |
| dependabot-preview[bot] | 53      | +640/-64    | 106           |
| Michael Muré            | 7       | +456/-213   | 17            |
| David Dias              | 11      | +426/-88    | 15            |
| Peter Rabbitson         | 11      | +254/-189   | 31            |
| Lukasz Zimnoch          | 9       | +361/-49    | 13            |
| Jakub Sztandera         | 4       | +157/-104   | 9             |
| Rod Vagg                | 1       | +91/-83     | 2             |
| RubenKelevra            | 13      | +84/-84     | 30            |
| JP Hastings-Spital      | 1       | +145/-0     | 2             |
| Adin Schmahmann         | 11      | +67/-37     | 15            |
| Marcin Rataj            | 11      | +41/-43     | 11            |
| Tiger                   | 5       | +53/-8      | 6             |
| Akira                   | 2       | +35/-19     | 2             |
| Casey Chance            | 2       | +31/-22     | 2             |
| Alan Shaw               | 1       | +44/-0      | 2             |
| Jessica Schilling       | 4       | +20/-19     | 7             |
| Gowtham G               | 4       | +22/-14     | 6             |
| Jeromy Johnson          | 3       | +24/-6      | 3             |
| Edgar Aroutiounian      | 3       | +16/-8      | 3             |
| Peter Wu                | 2       | +12/-9      | 2             |
| Sawood Alam             | 2       | +7/-7       | 2             |
| Command                 | 1       | +12/-0      | 1             |
| Eric Myhre              | 1       | +9/-2       | 1             |
| mawei                   | 2       | +5/-5       | 2             |
| decanus                 | 1       | +5/-5       | 1             |
| Ignacio Hagopian        | 2       | +7/-2       | 2             |
| Alfonso Montero         | 1       | +1/-5       | 1             |
| Volker Mische           | 1       | +2/-2       | 1             |
| Shotaro Yamada          | 1       | +2/-1       | 1             |
| Mark Gaiser             | 1       | +1/-1       | 1             |
| Johnny                  | 1       | +1/-1       | 1             |
| Ganesh Prasad Kumble    | 1       | +1/-1       | 1             |
| Dominic Della Valle     | 1       | +1/-1       | 1             |
| Corbin Page             | 1       | +1/-1       | 1             |
| Bryan Stenson           | 1       | +1/-1       | 1             |
| Bernhard M. Wiedemann   | 1       | +1/-1       | 1             |

## 0.5.1 2020-05-08

Hot on the heels of 0.5.0 is 0.5.1 with some important but small bug fixes. This release:

1. Removes the 1 minute timeout for IPNS publishes (fixes #7244).
2. Backport a DHT fix to reduce CPU usage for canceled requests.
3. Fixes some timer leaks in the QUIC transport ([ipfs/go-ipfs#2515](https://github.com/lucas-clemente/quic-go/issues/2515)).

### Changelog

- github.com/ipfs/go-ipfs:
  - IPNS timeout patch from master ([ipfs/go-ipfs#7276](https://github.com/ipfs/go-ipfs/pull/7276))
- github.com/libp2p/go-libp2p-core (v0.5.2 -> v0.5.3):
  - feat: add a function to tell if a context subscribes to query events ([libp2p/go-libp2p-core#147](https://github.com/libp2p/go-libp2p-core/pull/147))
- github.com/libp2p/go-libp2p-kad-dht (v0.7.10 -> v0.7.11):
  - fix: optimize for the case where we're not subscribing to query events ([libp2p/go-libp2p-kad-dht#624](https://github.com/libp2p/go-libp2p-kad-dht/pull/624))
  - fix: don't spin when the event channel is closed ([libp2p/go-libp2p-kad-dht#622](https://github.com/libp2p/go-libp2p-kad-dht/pull/622))
- github.com/libp2p/go-libp2p-routing-helpers (v0.2.2 -> v0.2.3):
  - fix: avoid subscribing to query events unless necessary ([libp2p/go-libp2p-routing-helpers#43](https://github.com/libp2p/go-libp2p-routing-helpers/pull/43))
- github.com/lucas-clemente/quic-go (v0.15.5 -> v0.15.7):
  - reset the PTO when dropping a packet number space
  - move deadlineTimer declaration out of the Read loop
  - stop the deadline timer in Stream.Read and Write
  - fix buffer use after it was released when sending an INVALID_TOKEN error
  - create the session timer at the beginning of the run loop
  - stop the timer when the session's run loop returns

### Contributors

| Contributor             | Commits | Lines ± | Files Changed |
|-------------------------|---------|---------|---------------|
| Marten Seemann          |      10 | +81/-62 |            19 |
| Steven Allen            |       5 | +42/-18 |            10 |
| Adin Schmahmann         |       1 | +2/-8   |             1 |
| dependabot-preview[bot] |       2 | +6/-2   |             4 |

## 0.5.0 2020-04-28

We're excited to announce go-ipfs 0.5.0! This is by far the largest go-ipfs release with ~2500 commits, 98 contributors, and over 650 PRs across ipfs, libp2p, and multiformats.

### Highlights

#### Content Routing

The primary focus of this release was on improving content routing. That is, advertising and finding content. To that end, this release heavily focuses on improving the DHT.

##### Improved DHT

The distributed hash table (DHT) is how IPFS nodes keep track of who has what data. The DHT implementation has been almost completely rewritten in this release. Providing, finding content, and resolving IPNS records are now all much faster. However, there are risks involved with this update due to the significant amount of changes that have gone into this feature.

The current DHT suffers from three core issues addressed in this release:

- Most peers in the DHT cannot be dialed (e.g., due to firewalls and NATs). Much of a DHT query time is wasted trying to connect to peers that cannot be reached.
- The DHT query logic doesn't properly terminate when it hits the end of the query and, instead, aggressively keeps on searching.
- The routing tables are poorly maintained. This can cause search performance to slow down linearly with network size, instead of logarithmically as expected.

###### Reachability

We have addressed the problem of undialable nodes by having nodes wait to join the DHT as _server_ nodes until they've confirmed that they are reachable from the public internet.

To ensure that nodes which are not publicly reachable (ex behind VPNs, offline LANs, etc.) can still coordinate and share data, go-ipfs 0.5 will run two DHTs: one for private networks and one for the public internet. Every node will participate in a LAN DHT and a public WAN DHT. See [Dual DHT](#dual-dht) for more details.

###### Dual DHT

All IPFS nodes will now run two DHTs: one for the public internet WAN, and one for their local network LAN.

1. When connected to the public internet, IPFS will use both DHTs for finding peers, content, and IPNS records. Nodes only publish provider and IPNS records to the WAN DHT to avoid flooding the local network.
2. When not connected to the public internet, nodes publish provider and IPNS records to the LAN DHT.

The WAN DHT includes all peers with at least one public IP address. This release will only consider an IPv6 address public if it is in the [public internet range `2000::/3`](https://www.iana.org/assignments/ipv6-address-space/ipv6-address-space.xhtml).

This feature should not have any noticeable impact on go-ipfs, performance, or otherwise. Everything should continue to work in all the currently supported network configurations: VPNs, disconnected LANs, public internet, etc.

###### Query Logic

We've improved the DHT query logic to more closely follow Kademlia. This should significantly speed up:

- Publishing IPNS & provider records.
- Resolving IPNS addresses.

Previously, nodes would continue searching until they timed out or ran out of peers before stopping (putting or returning data found). Now, nodes will now stop as soon as they find the closest peers.

###### Routing Tables

Finally, we've addressed the poorly maintained routing tables by:

- Reducing the likelihood that the connection manager will kill connections to peers in the routing table.
- Keeping peers in the routing table, even if we get disconnected from them.
- Actively and frequently querying the DHT to keep our routing table full.
- Prioritizing useful peers that respond to queries quickly.

##### Testing

The DHT rewrite was made possible by [Testground](https://github.com/ipfs/testground/), our new testing framework. Testground allows us to spin up multi-thousand node tests with simulated real-world network conditions. By combining Testground and some custom analysis tools, we were able to gain confidence that the new DHT implementation behaves correctly.

##### Provider Record Changes

When you add content to your IPFS node, you advertise this content to the network by announcing it in the DHT. We call this _providing_.

However, go-ipfs has multiple ways to address the same underlying bytes. Specifically, we address content by content ID (CID) and the same underlying bytes can be addressed using (a) two different versions of CIDs (CIDv0 and CIDv1) and (b) with different _codecs_ depending on how we're interpreting the data.

Prior to go-ipfs 0.5.0, we used the content id (CID) in the DHT when sending out provider records for content. Unfortunately, this meant that users trying to find data announced using one CID wouldn't find nodes providing the content under a different CID.

In go-ipfs 0.5.0, we're announcing data by _multihash_, not _CID_. This way, regardless of the CID version used by the peer adding the content, the peer trying to download the content should still be able to find it.

**Warning:** as part of the network, this could impact finding content added with CIDv1. Because go-ipfs 0.5.0 will announce and search for content using the bare multihash (equivalent to the v0 CID), go-ipfs 0.5.0 will be unable to find CIDv1 content published by nodes prior to go-ipfs 0.5.0 and vice-versa. As CIDv1 is _not_ enabled by default so we believe this will have minimal impact. However, users are _strongly_ encouraged to upgrade as soon as possible.

#### Content Transfer

A secondary focus in this release was improving content _transfer_, our data exchange protocols.

##### Refactored Bitswap

This release includes a major [Bitswap refactor](https://blog.ipfs.io/2020-02-14-improved-bitswap-for-container-distribution/), running a new and backward compatible Bitswap protocol. We expect these changes to improve performance significantly.

With the refactored Bitswap, we expect:

- Few to no duplicate blocks when fetching data from other nodes speaking the _new_ protocol.
- Better parallelism when fetching from multiple peers.

The new Bitswap won't magically make downloading content any faster until both seeds and leaches have updated. If you're one of the first to upgrade to `0.5.0` and try downloading from peers that haven't upgraded, you're unlikely to see much of a performance improvement.

[bitswap-refactor]: https://blog.ipfs.io/2020-02-14-improved-bitswap-for-container-distribution/

##### Server-Side Graphsync Support (Experimental)

Graphsync is a new exchange protocol that operates at the IPLD Graph layer instead of the Block layer like bitswap.

For example, to download "/ipfs/QmExample/index.html":

* Bitswap would download QmFoo, lookup "index.html" in the directory named by
QmFoo, resolving it to a CID QmIndex. Finally, bitswap would download QmIndex.
* Graphsync would ask peers for "/ipfs/QmFoo/index.html". Specifically, it would ask for the child named "index.html" of the object named by "QmFoo".

This saves us round-trips in exchange for some extra protocol complexity. Moreover, this protocol allows specifying more powerful queries like "give me everything under QmFoo". This can be used to quickly download a large amount of data with few round-trips.

At the moment, go-ipfs cannot use this protocol to download content from other peers. However, if enabled, go-ipfs can _serve_ content to other peers over this protocol. This may be useful for pinning services that wish to quickly replicate client data.

To enable, run:

```bash
> ipfs config --json Experimental.GraphsyncEnabled true
```

#### Datastores

Continuing with the of improving our core data handling subsystems, both of the datastores used in go-ipfs, Badger and flatfs, have received important updates in this release:

##### Badger

Badger has been in go-ipfs for over a year as an experimental feature, and we're promoting it to stable (but not default). For this release, we've switched from writing to disk synchronously to explicitly syncing where appropriate, significantly increasing write throughput.

The current and default datastore used by go-ipfs is [FlatFS](https://github.com/ipfs/go-ds-flatfs). FlatFS essentially stores blocks of data as individual files on your file system. However, there are lots of optimizations a specialized database can do that a standard file system can not.

The benefit of Badger is that adding/fetching data to/from Badger is significantly faster than adding/fetching data to/from the default datastore, FlatFS. In some tests, adding data to Badger is 32x faster than FlatFS (in this release).

###### Enable Badger

In this release, we're marking the badger datastore as stable. However, we're not yet enabling it by default. You can enable it at initialization by running: `ipfs init --profile=badgerds`

###### Issues with Badger

While Badger is a great solution, there are some issues you should consider before enabling it.

Badger is complicated. FlatFS pushes all the complexity down into the filesystem itself. That means that FlatFS is only likely to lose your data if your underlying filesystem gets corrupted while there are more opportunities for Badger itself to get corrupted.

Badger can use a lot of memory. In this release, we've tuned Badger to use `~20MB` of memory by default. However, it can still produce spikes as large as [`1GiB` of data](https://github.com/dgraph-io/badger/issues/1292) in memory usage when garbage collecting.

Finally, Badger isn't very aggressive when it comes to garbage collection, and we're still investigating ways to get it to more aggressively clean up after itself.

We suggest you use Badger if:

- Performance is your main requirement.
- You rarely delete anything.
- You have some memory to spare.

##### Flatfs

In the flatfs datastore, we've fixed an issue where temporary files could be left behind in some cases. While this release will avoid leaving behind temporary files, you may want to remove any left behind by previous releases:

```bash
> rm ~/.ipfs/blocks/*/put-*
> rm ~/.ipfs/blocks/du-*
```

We've also hardened several edge-cases in flatfs to reduce the impact of file descriptor limits, spurious crashes, etc.

#### Libp2p

Many improvements and bug fixes were made to libp2p over the course of this release. These release notes only include the most important and those most relevant to the content routing improvements.

##### Improved Backoff Logic

When we fail to connect to a peer, we "backoff" and refuse to re-connect to that peer for a short period of time. This prevents us from wasting resources repeatedly failing to connect to the same unreachable peer.

Unfortunately, the old backoff logic was flawed: if we failed to connect to a peer and entered the "backoff" state, we wouldn't try to re-connect to that peer even if we had learned new and potentially working addresses for the peer. We've fixed this by applying backoff to each _address_ instead of to the peer as a whole. This achieves the same result as we'll stop repeatedly trying to connect to the peer at known-bad addresses, but it allows us to reach the peer if we later learn about a good address.

##### AutoNAT

This release uses Automatic NAT Detection (AutoNAT) - determining if the node is _reachable_ from the public internet - to make decisions about how to participate in IPFS. This subsystem is used to determine if the node should store some of the public DHT, and if it needs to use relays to be reached by others. In short:

1. An AutoNAT client asks a node running an AutoNAT service if it can be reached at one of a set of guessed addresses.
2. The AutoNAT service attempts to _dial back_ those addresses, with some restrictions. We won't dial back to a different IP address, for example.
3. If the AutoNAT service succeeds, it reports back the address it successfully dialed, and the AutoNAT client knows that it is reachable from the public internet.

All nodes act as AutoNAT clients to determine if they should switch into DHT server mode. As of this release, nodes will by default run the service side of AutoNAT - verifying connectivity - for up to 30 peers every minute. This service should have minimal overhead and will be disabled for nodes in the `lowpower` configuration profile, and those which believe they are not publicly reachable.

In addition to enabling the AutoNAT service by default, this release changes the AutoNAT config options:

1. The `Swarm.EnableAutoNATService` option has been removed.
2. A new AutoNAT section has been added to the config. This section is empty by default.


##### IPFS/Libp2p Address Format

If you've ever run a command like `ipfs swarm peers`, you've likely seen paths that look like `/ip4/193.45.1.24/tcp/4001/ipfs/QmSomePeerID`. These paths are _not_ file paths, they're multiaddrs; addresses of peers on the network.

Unfortunately, `/ipfs/Qm...` is _also_ the same path format we use for files. This release, changes the multiaddr format from <code>/ip4/193.45.1.24/tcp/4001/<b>ipfs</b>/QmSomePeerID</code> to <code>/ip4/193.45.1.24/tcp/4001/<b>p2p</b>/QmSomePeerID</code> to make the distinction clear.

What this means for users:

* Old-style multiaddrs will still be accepted as inputs to IPFS.
* If you were using a multiaddr library (go, js, etc.) to name _files_ because `/ipfs/QmSomePeerID` looks like `/ipfs/QmSomeFile`, your tool may break if you upgrade this library.
* If you're manually parsing multiaddrs and are searching for the string `/ipfs/`..., you'll need to search for `/p2p/...`.

##### Minimum RSA Key Size

Previously, IPFS did not enforce a minimum RSA key size. In this release, we've introduced a minimum 2048 bit RSA key size. IPFS generates 2048 bit RSA keys by default so this shouldn't be an issue for anyone in practice. However, users who explicitly chose a smaller key size will not be able to communicate with new nodes.

Unfortunately, some of the bootstrap peers _did_ intentionally generate 1024 bit RSA keys so they'd have vanity peer addresses (starting with QmSoL for "solar net"). All IPFS nodes should _also_ have peers with >= 2048 bit RSA keys in their bootstrap list, but we've introduced a migration to ensure this.

We implemented this change to follow security best practices and to remove a potential foot-gun. However, in practice, the security impact of allowing insecure RSA keys should have been next to none because IPFS doesn't trust other peers on the network anyways.

##### TLS By Default

In this release, we're switching TLS to be the _default_ transport. This means we'll try to encrypt the connection with TLS before re-trying with SECIO.

Contrary to the announcement in the go-ipfs 0.4.23 release notes, this release does not remove SECIO support to maintain compatibility with js-ipfs.

Note: The `Experimental.PreferTLS` configuration option is now ignored.

##### SECIO Deprecation Notice

SECIO should be considered to be well on the way to deprecation and will be
completely disabled in either the next release (0.6.0, ~mid May) or the one
following that (0.7.0, ~end of June). Before SECIO is disabled, support will be
added for the NOISE transport for compatibility with other IPFS implementations.

##### QUIC Upgrade

If you've been using the experimental QUIC support, this release upgrades to a new and _incompatible_ version of the QUIC protocol (draft 27). Old and new go-ipfs nodes will still interoperate, but not over the QUIC transport.

We intend to standardize on this draft of the QUIC protocol and enable QUIC by default in the next release if all goes well.

NOTE: QUIC does not yet support [private networks](./docs/experimental-features.md#private-networks).

#### Gateway

In addition to a bunch of bug fixes, we've made two improvements to the gateway.

You can play with both of these features by visiting:

> http://bafybeia6po64b6tfqq73lckadrhpihg2oubaxgqaoushquhcek46y3zumm.ipfs.localhost:8080

##### Subdomain Gateway

First up, we've changed how URLs in the IPFS gateway work for better browser
security. The gateway will now redirect from
`http://localhost:8080/ipfs/CID/...` to `http://CID.ipfs.localhost:8080/...` by
default. This:

* Ensures that every dapp gets its own browser origin.
* Makes it easier to write websites that "just work" with IPFS because absolute paths will now work (though you should still use relative links because they're better).
  
Paths addressing the gateway by IP address (`http://127.0.0.1:5001/ipfs/CID`) will not be altered as IP addresses can't have subdomains.

Note: cURL doesn't follow redirects by default. To avoid breaking cURL and other clients that don't support redirects, go-ipfs will return the requested file along with the redirect. Browsers will follow the redirect and abort the download while cURL will ignore the redirect and finish the download.

##### Directory Listing

The second feature is a face-lift to the directory listing theme and color palette.

> http://bafybeia6po64b6tfqq73lckadrhpihg2oubaxgqaoushquhcek46y3zumm.ipfs.localhost:8080

#### IPNS

This release includes several new IPNS and IPNS-related features.

##### ENS

IPFS now resolves [ENS](https://ens.domains/) names (e.g., `/ipns/ipfs.eth`) via DNSLink provided by https://eth.link service.

##### IPNS over PubSub

IPFS has had experimental support for resolving IPNS over pubsub for a while. However, in the past, this feature was passive. When resolving an IPNS name, one would join a pubsub topic for the IPNS name and subscribe to _future_ updates. Unfortunately, this wouldn't speed-up initial IPNS lookups.

In this release, we've introduced a new "record fetch" protocol to speedup the initial lookup. Now, after subscribing to the pubsub topic for the IPNS key, nodes will use this new protocol to "fetch" the last-seen IPNS record from all peers subscribed to the topic.

This feature will be enabled by default in 0.6.0.

##### IPNS with base32 PIDs

IPNS names can now be expressed as special multibase CIDs. E.g., 

> /ipns/bafzbeibxfjp4gaxc4cdn57257cyvc7jfa4rlp4e5min6geg44m57g6nx7e

Importantly, this allows IPNS names to appear in subdomains in the new [subdomain gateway](#subdomain-gateway) feature.

#### PubSub

We have made two major changes to the pubsub subsystem in this release:

1. Pubsub now more aggressively finds and connects to other peers subscribing to the same topic.
2. Go-ipfs has switched its default pubsub router from "floodsub", an inefficient but simple "flooding" pubsub implementation, to "gossipsub".

PubSub will be stabilized in go-ipfs 0.6.0.

#### CLI & API

The IPFS CLI and API have a couple of new features and changes.

##### POST Only

IPFS has two HTTP APIs:

* Port 5001: http://localhost:5001/api/v0/... - the API
* Port 8080: http://localhost:8080/api/v0/... - a read-only subset of the API, accessible via the gateway

As of this release, the main IPFS API (port 5001) will only accept POST requests. This change is necessary to tighten cross origin security in browsers.

If you're using the go-ipfs API in your application, you may need to change GET calls to POST calls or upgrade your libraries and tools.

* go - go-ipfs-api - v0.0.3
* js-ipfs-http-api - v0.41.1
* orbit-db - v0.24.0 (unreleased)

##### RIP "Error: api not running"

If you've ever seen [the error](https://github.com/ipfs/go-ipfs/issues/5784):

> Error: api not running

when trying to run a command without the daemon running, we have good news! You
should never see this error again. The `ipfs` command now correctly detects that the daemon is not, in fact, running, and directly opens the IPFS repo.

##### RIP `ipfs repo fsck`

The `ipfs repo fsck` now does nothing but print an error message. Previously, it was used to cleanup some lock files: the "api" file that caused the aforementioned "api not running" error and the repo lock. However, this is no longer necessary.

##### Init with config

It's now possible to initialize an IPFS node with an existing IPFS config by running:

```bash
> ipfs init /path/to/existing/config
```

This will re-use the existing configuration in it's entirety (including the private key) and can be useful when:

* Migrating a node's identity between machines without keeping the data.
* Resetting the datastore.

##### Ignoring Files

Files can now be ignored on add by passing the `--ignore` and/or
`--ignore-rules-path` flags.

* `--ignore=PATTERN` will ignore all files matching the gitignore rule PATTERN.
* `--ignore-rules-path=FILENAME` will apply the gitignore rules from the specified file.

For example, to add a git repo while ignoring all files git would ignore, you could run:

```bash
> cd path/to/some/repo
> ipfs add -r --hidden=false --ignore=.git --ignore-rules-path=.gitignore .
```

##### Named Pipes

It's now possible to add data directly from a named pipe:

```bash
> mkfifo foo
> echo -n "hello " > foo &
> echo -n "world" > bar &
> ipfs add foo bar
```

This can be useful when adding data from multiple streaming sources.

NOTE: To avoid surprising users, IPFS will only add data from FIFOs _directly_ named on the command line, not FIFOs in a recursively added directory. Otherwise, `ipfs add` would halt whenever it encountered a FIFO with no data to be read leading to difficult to debug stalls.

##### DAG import/export (.car)

IPFS now allows rapid reading and writing of blocks in [`.car` format](https://github.com/ipld/specs/blob/master/block-layer/content-addressable-archives.md#readme). The functionality is accessible via the experimental `dag import` and `dag export` commands:

```
~$ ipfs dag export QmQPeNsJPyVWPFDVHb77w8G42Fvo15z4bG2X8D2GhfbSXc \
| xz > welcome_to_ipfs.car.xz

 0s  6.73 KiB / ? [-------=-------------------------------------] 5.16 MiB/s 0s 

```
Then on another `ipfs` instance, not even connected to the network:
```
~$ xz -dc welcome_to_ipfs.car.xz | ipfs dag import

Pinned root	QmQPeNsJPyVWPFDVHb77w8G42Fvo15z4bG2X8D2GhfbSXc	success

```

##### Pins

We've made two minor changes to the pinning subsystem:

1. `ipfs pin ls --stream` allows streaming a pin listing.
2. `ipfs pin update` no longer holds the global pin lock while fetching files from the network. This should hopefully make it significantly more useful.

#### Daemon

##### Zap Logging

The go-ipfs daemon has switched to using [Uber's Zap](https://go.uber.org/zap). Unlike our previous logging system, Zap supports _structured_ logging which can make parsing, filtering, and analyzing go-ipfs logs much simpler.

To enable structured logging, set the `IPFS_LOGGING_FMT` environment variable to "json".

Note: while we've switched to using Zap as the logging backend, most of go-ipfs still logs strings.

##### Systemd Support 

For Linux users, this release includes support for two systemd features: socket activation and startup/shutdown notifications. This makes it possible to:

* Start IPFS on demand on first use.
* Wait for IPFS to finish starting before starting services that depend on it.

You can find the new systemd units in the go-ipfs repo under misc/systemd.

##### IPFS API Over Unix Domain Sockets

This release supports exposing the IPFS API over a unix domain socket in the filesystem. You use this feature, run:

```bash
> ipfs config Addresses.API "/unix/path/to/socket/location"
```

##### Docker

We've made a few improvements to our docker image in this release:

* It can now be cross-built for multiple architectures.
* It now builds go-ipfs with OpenSSL support by default for faster libp2p handshakes.
* A private-network "swarm" key can now be passed in to a docker image via either the `IPFS_SWARM_KEY=<inline key>` or `IPFS_SWARM_KEY_FILE=<path/to/key/file>` docker variables. Check out the Docker section of the README for more information.

#### Plugins

go-ipfs plugins allow users to extend go-ipfs without modifying the original source-code. This release includes a few important changes.

See [docs/plugins.md](./docs/plugins.md) for details.

##### MacOS Support

Plugins are now supported on MacOS, in addition to Linux. Unfortunately, Go still doesn't [support plugins on Windows](https://github.com/golang/go/issues/19282).

##### New Plugin Type: `InternalPlugin`

This release introduces a new `InternalPlugin` plugin type. When started, this plugin will be passed a raw `*IpfsNode` object, giving it access to all go-ipfs internals.

This plugin interface is permanently unstable as it has access to internals that can change frequently. However, it should allow power-users to develop deeply integrated extensions to go-ipfs, out-of-tree.

##### Plugin Config

**BREAKING**

Plugins can now be configured and/or disabled via the [ipfs config file](./docs/plugins.md#configuration).

To make this possible, the plugin interface has changed. The `Init` function now takes an `*Environment` object. Specifically, the plugin signature has changed from:

```go
type Plugin interface {
	Name() string
	Version() string
	Init() error
}
```

to 

```go
type Environment struct {
	// Path to the IPFS repo.
	Repo string

	// The plugin's config, if specified.
	Config interface{}
}

type Plugin interface {
	Name() string
	Version() string
	Init(env *Environment) error
}
```

#### Repo Migrations

IPFS uses repo migrations to make structural changes to the "repo" (the config, data storage, etc.) on upgrade.

This release includes two very simple repo migrations: a config migration to ensure that the config contains working bootstrap nodes and a keystore migration to base32 encode all key filenames.

In general, migrations should not require significant manual intervention. However, you should be aware of migrations and plan for them.

* If you update go-ipfs with `ipfs update`, `ipfs update` will run the migration for you. Note: `ipfs update` will refuse to run the migrations while ipfs itself is running.
* If you start the ipfs daemon with `ipfs daemon --migrate`, ipfs will migrate your repo for you on start.

Otherwise, if you want more control over the repo migration process, you can manually install and run the [repo migration tool](http://dist.ipfs.io/#fs-repo-migrations).

##### Bootstrap Peer Changes

**AUTOMATIC MIGRATION REQUIRED**

The first migration will update the bootstrap peer list to:

1. Replace the old bootstrap nodes (ones with peer IDs starting with QmSoL), with new bootstrap nodes (ones with addresses that start with `/dnsaddr/bootstrap.libp2p.io`).
2. Rewrite the address format from `/ipfs/QmPeerID` to `/p2p/QmPeerID`.

We're migrating addresses for a few reasons:

1. We're using DNS to address the new bootstrap nodes so we can change the underlying IP addresses as necessary.
2. The new bootstrap nodes use 2048 bit keys while the old bootstrap nodes use 1024 bit keys.
3. We're normalizing the address format to `/p2p/Qm...`.

Note: This migration won't _add_ the new bootstrap peers to your config if you've explicitly removed the old bootstrap peers. It will also leave custom entries in the list alone. In other words, if you've customized your bootstrap list, this migration won't clobber your changes.

##### Keystore Changes

**AUTOMATIC MIGRATION REQUIRED**

go-ipfs stores additional keys (i.e., all keys other than the "identity" key) in the keystore. You can list these keys with `ipfs key`.

Currently, the keystore stores keys as regular files, named after the key itself. Unfortunately, filename restrictions and case-insensitivity are platform specific. To avoid platform specific issues, we're base32 encoding all key names and renaming all keys on-disk.

#### Windows

As usual, this release contains several Windows specific fixes and improvements:

* Double-clicking `ipfs.exe` will now start the daemon inside a console window.
* `ipfs add -r` now correctly recognizes and ignores hidden files on Windows.
* The default datastore, flatfs, now takes extra precautions to avoid "file in use" errors caused by both go-ipfs and external programs like anti-viruses. If you've ever seen go-ipfs print out an "access denied" or "file in use" error on Windows, this issue was likely the cause.

### Changelog

- github.com/ipfs/go-ipfs:
  - fix: non-blocking peerlog logging ([ipfs/go-ipfs#7232](https://github.com/ipfs/go-ipfs/pull/7232))
  - doc: update go-ipfs docs for 0.5.0 release ([ipfs/go-ipfs#7229](https://github.com/ipfs/go-ipfs/pull/7229))
  - Add additional documentation links to the new issue screen ([ipfs/go-ipfs#7226](https://github.com/ipfs/go-ipfs/pull/7226))
  - docs: note that ShardingEnabled is a global flag ([ipfs/go-ipfs#7218](https://github.com/ipfs/go-ipfs/pull/7218))
  - update log helptext to match actual levels ([ipfs/go-ipfs#7199](https://github.com/ipfs/go-ipfs/pull/7199))
  - Chore/harden car test a bit harder ([ipfs/go-ipfs#7209](https://github.com/ipfs/go-ipfs/pull/7209))
  - fix: fix duplicate block issue in bitswap ([ipfs/go-ipfs#7202](https://github.com/ipfs/go-ipfs/pull/7202))
  - feat: update docker image ([ipfs/go-ipfs#7191](https://github.com/ipfs/go-ipfs/pull/7191))
  - feat: update dir index ([ipfs/go-ipfs#7192](https://github.com/ipfs/go-ipfs/pull/7192))
  - fix: update the dht to fix yggdrasil ([ipfs/go-ipfs#7186](https://github.com/ipfs/go-ipfs/pull/7186))
  - Choose architecture when download tini into docker container ([ipfs/go-ipfs#7187](https://github.com/ipfs/go-ipfs/pull/7187))
  - Fix typos and cleanup ([ipfs/go-ipfs#7181](https://github.com/ipfs/go-ipfs/pull/7181))
  - Fixtypos ([ipfs/go-ipfs#7180](https://github.com/ipfs/go-ipfs/pull/7180))
  - feat: webui 2.7.5 ([ipfs/go-ipfs#7176](https://github.com/ipfs/go-ipfs/pull/7176))
  - integration test for the dual dht ([ipfs/go-ipfs#7151](https://github.com/ipfs/go-ipfs/pull/7151))
  - fix: subdomain redirect for dir CIDs ([ipfs/go-ipfs#7165](https://github.com/ipfs/go-ipfs/pull/7165))
  - add autonat config options ([ipfs/go-ipfs#7162](https://github.com/ipfs/go-ipfs/pull/7162))
  - docs: fix link to version.go ([ipfs/go-ipfs#7157](https://github.com/ipfs/go-ipfs/pull/7157))
  - feat: webui v2.7.4 ([ipfs/go-ipfs#7159](https://github.com/ipfs/go-ipfs/pull/7159))
  - fix the typo in the serveHTTPApi ([ipfs/go-ipfs#7156](https://github.com/ipfs/go-ipfs/pull/7156))
  - test(sharness): improve CAR tests to remove some potential races ([ipfs/go-ipfs#7154](https://github.com/ipfs/go-ipfs/pull/7154))
  - feat: introduce the dual WAN/LAN DHT ([ipfs/go-ipfs#7127](https://github.com/ipfs/go-ipfs/pull/7127))
  - fix: invalidate cache on failed publish ([ipfs/go-ipfs#7152](https://github.com/ipfs/go-ipfs/pull/7152))
  - Temporarily disable gc-race test ([ipfs/go-ipfs#7148](https://github.com/ipfs/go-ipfs/pull/7148))
  - Beef up and harden import/export tests ([ipfs/go-ipfs#7140](https://github.com/ipfs/go-ipfs/pull/7140))
  - Filter dials to blocked subnets, even when using DNS. ([ipfs/go-ipfs#6996](https://github.com/ipfs/go-ipfs/pull/6996))
  - Dag export command, complete ([ipfs/go-ipfs#7036](https://github.com/ipfs/go-ipfs/pull/7036))
  - Adding Fission to IPFS early testers page ([ipfs/go-ipfs#7119](https://github.com/ipfs/go-ipfs/pull/7119))
  - feat: bump version ([ipfs/go-ipfs#7110](https://github.com/ipfs/go-ipfs/pull/7110))
  - feat: initial update to the changelog for 0.5.0 ([ipfs/go-ipfs#6977](https://github.com/ipfs/go-ipfs/pull/6977))
  - feat(dht): update to cypress DHT in backwards compatibility mode ([ipfs/go-ipfs#7103](https://github.com/ipfs/go-ipfs/pull/7103))
  - update bash completion for `ipfs add` ([ipfs/go-ipfs#7102](https://github.com/ipfs/go-ipfs/pull/7102))
  - HTTP API: Only allow POST requests (plus OPTIONS) ([ipfs/go-ipfs#7097](https://github.com/ipfs/go-ipfs/pull/7097))
  - Revert last change (the default is now printed twice) ([ipfs/go-ipfs#7098](https://github.com/ipfs/go-ipfs/pull/7098))
  - Fix #4996: Improve help text for "ipfs files cp" ([ipfs/go-ipfs#7069](https://github.com/ipfs/go-ipfs/pull/7069))
  - changed brew to brew cask ([ipfs/go-ipfs#7072](https://github.com/ipfs/go-ipfs/pull/7072))
  - fix: remove internal relay discovery ([ipfs/go-ipfs#7064](https://github.com/ipfs/go-ipfs/pull/7064))
  - docs/experimental-features.md: typo ([ipfs/go-ipfs#7062](https://github.com/ipfs/go-ipfs/pull/7062))
  - fix: get rid of shutdown errors ([ipfs/go-ipfs#7058](https://github.com/ipfs/go-ipfs/pull/7058))
  - feat: tls by default ([ipfs/go-ipfs#7055](https://github.com/ipfs/go-ipfs/pull/7055))
  - fix: downgrade to go 1.13 ([ipfs/go-ipfs#7054](https://github.com/ipfs/go-ipfs/pull/7054))
  - Keystore: minor maintenance ([ipfs/go-ipfs#7043](https://github.com/ipfs/go-ipfs/pull/7043))
  - fix(keystore): avoid racy filesystem access ([ipfs/go-ipfs#6999](https://github.com/ipfs/go-ipfs/pull/6999))
  - Forgotten go-fmt ([ipfs/go-ipfs#7030](https://github.com/ipfs/go-ipfs/pull/7030))
  - feat: update go-libp2p & go-bitswap ([ipfs/go-ipfs#7028](https://github.com/ipfs/go-ipfs/pull/7028))
  - Introducing EncodedFSKeystore with base32 encoding (#5947) ([ipfs/go-ipfs#6955](https://github.com/ipfs/go-ipfs/pull/6955))
  - feat: improve key lookup ([ipfs/go-ipfs#7023](https://github.com/ipfs/go-ipfs/pull/7023))
  - feat(file-ignore): add ignore opts to add cmd ([ipfs/go-ipfs#7017](https://github.com/ipfs/go-ipfs/pull/7017))
  - feat: gateway subdomains + http proxy mode ([ipfs/go-ipfs#6096](https://github.com/ipfs/go-ipfs/pull/6096))
  - Chore/sharness fixes 2019 03 16 ([ipfs/go-ipfs#6997](https://github.com/ipfs/go-ipfs/pull/6997))
  - Support pipes when named on the cli explicitly ([ipfs/go-ipfs#6998](https://github.com/ipfs/go-ipfs/pull/6998))
  - Fix a typo ([ipfs/go-ipfs#7000](https://github.com/ipfs/go-ipfs/pull/7000))
  - fix: revert changes to the user agent ([ipfs/go-ipfs#6993](https://github.com/ipfs/go-ipfs/pull/6993))
  - feat(peerlog): log protocols/versions ([ipfs/go-ipfs#6972](https://github.com/ipfs/go-ipfs/pull/6972))
  - feat: docker build and tag from ci ([ipfs/go-ipfs#6949](https://github.com/ipfs/go-ipfs/pull/6949))
  - cmd: ipfs handle GUI environment on Windows ([ipfs/go-ipfs#6646](https://github.com/ipfs/go-ipfs/pull/6646))
  - Chore/macos sharness fixes ([ipfs/go-ipfs#6988](https://github.com/ipfs/go-ipfs/pull/6988))
  - Update to go-libp2p 0.6.0 ([ipfs/go-ipfs#6914](https://github.com/ipfs/go-ipfs/pull/6914))
  - mount: switch over to the CoreAPI ([ipfs/go-ipfs#6602](https://github.com/ipfs/go-ipfs/pull/6602))
  - doc(commands): document that `dht put` takes a file ([ipfs/go-ipfs#6960](https://github.com/ipfs/go-ipfs/pull/6960))
  - docs: update licence info in README ([ipfs/go-ipfs#6942](https://github.com/ipfs/go-ipfs/pull/6942))
  - docs: fix example for files.write ([ipfs/go-ipfs#6943](https://github.com/ipfs/go-ipfs/pull/6943))
  - feat(graphsync): mount the graphsync libp2p protocol ([ipfs/go-ipfs#6892](https://github.com/ipfs/go-ipfs/pull/6892))
  - feat: update go in docker container ([ipfs/go-ipfs#6933](https://github.com/ipfs/go-ipfs/pull/6933))
  - remove expired GPG key from README ([ipfs/go-ipfs#6931](https://github.com/ipfs/go-ipfs/pull/6931))
  - test(sharness): test our tests ([ipfs/go-ipfs#6908](https://github.com/ipfs/go-ipfs/pull/6908))
  - fix: broken interop tests ([ipfs/go-ipfs#6899](https://github.com/ipfs/go-ipfs/pull/6899))
  - feat: pass IPFS_PLUGINS to docker build ([ipfs/go-ipfs#6898](https://github.com/ipfs/go-ipfs/pull/6898))
  - doc(add): document hash stability ([ipfs/go-ipfs#6891](https://github.com/ipfs/go-ipfs/pull/6891))
  - feat: add peerlog plugin ([ipfs/go-ipfs#6887](https://github.com/ipfs/go-ipfs/pull/6887))
  - doc(plugin): document internal plugins ([ipfs/go-ipfs#6888](https://github.com/ipfs/go-ipfs/pull/6888))
  - Fix #6878: Improve MFS Cli documentation  ([ipfs/go-ipfs#6882](https://github.com/ipfs/go-ipfs/pull/6882))
  - Update the license distributed with dist builds to the dual one ([ipfs/go-ipfs#6879](https://github.com/ipfs/go-ipfs/pull/6879))
  - doc: add license URLs so go's doc service can detect our license ([ipfs/go-ipfs#6874](https://github.com/ipfs/go-ipfs/pull/6874))
  - doc: rename COPYRIGHT to LICENSE ([ipfs/go-ipfs#6873](https://github.com/ipfs/go-ipfs/pull/6873))
  - fix: fix id addr format ([ipfs/go-ipfs#6872](https://github.com/ipfs/go-ipfs/pull/6872))
  - Help text update for 'ipfs key gen' ([ipfs/go-ipfs#6867](https://github.com/ipfs/go-ipfs/pull/6867))
  - fix: make rsa the default key type ([ipfs/go-ipfs#6864](https://github.com/ipfs/go-ipfs/pull/6864))
  - doc(config): cleanup ([ipfs/go-ipfs#6855](https://github.com/ipfs/go-ipfs/pull/6855))
  - Allow building non-amd64 Docker images ([ipfs/go-ipfs#6854](https://github.com/ipfs/go-ipfs/pull/6854))
  - doc(release): add Charity Engine to the early testers programme ([ipfs/go-ipfs#6850](https://github.com/ipfs/go-ipfs/pull/6850))
  - fix: fix a potential out of bounds issue in fuse ([ipfs/go-ipfs#6847](https://github.com/ipfs/go-ipfs/pull/6847))
  - fix(build): instruct users to use GOTAGS, not GOFLAGS ([ipfs/go-ipfs#6843](https://github.com/ipfs/go-ipfs/pull/6843))
  - doc(release): document how RCs should be communicated ([ipfs/go-ipfs#6845](https://github.com/ipfs/go-ipfs/pull/6845))
  - doc(release): move WebUI from manual tests to automated tests section ([ipfs/go-ipfs#6838](https://github.com/ipfs/go-ipfs/pull/6838))
  - test(sharness): fix typo ([ipfs/go-ipfs#6835](https://github.com/ipfs/go-ipfs/pull/6835))
  - test: E2E tests against ipfs-webui HEAD ([ipfs/go-ipfs#6825](https://github.com/ipfs/go-ipfs/pull/6825))
  - mkreleaslog: improve edge-cases ([ipfs/go-ipfs#6833](https://github.com/ipfs/go-ipfs/pull/6833))
  - fix: dont fail to collect profiles if no ipfs bin ([ipfs/go-ipfs#6829](https://github.com/ipfs/go-ipfs/pull/6829))
  - update dockerfile and use openssl ([ipfs/go-ipfs#6828](https://github.com/ipfs/go-ipfs/pull/6828))
  - docs: define Gateway.PathPrefixes ([ipfs/go-ipfs#6826](https://github.com/ipfs/go-ipfs/pull/6826))
  - fix(badgerds): turn off sync writes by default ([ipfs/go-ipfs#6819](https://github.com/ipfs/go-ipfs/pull/6819))
  - gateway cleanups ([ipfs/go-ipfs#6820](https://github.com/ipfs/go-ipfs/pull/6820))
  - make it possible to change the codec with the `ipfs cid` subcommand ([ipfs/go-ipfs#6817](https://github.com/ipfs/go-ipfs/pull/6817))
  - improve gateway symlink handling ([ipfs/go-ipfs#6680](https://github.com/ipfs/go-ipfs/pull/6680))
  - Inclusion of the presence of the go-ipfs package in Solus ([ipfs/go-ipfs#6809](https://github.com/ipfs/go-ipfs/pull/6809))
  - Fix Typos ([ipfs/go-ipfs#6807](https://github.com/ipfs/go-ipfs/pull/6807))
  - Sharness macos no brainer fixes ([ipfs/go-ipfs#6805](https://github.com/ipfs/go-ipfs/pull/6805))
  - Support Asynchronous Datastores ([ipfs/go-ipfs#6785](https://github.com/ipfs/go-ipfs/pull/6785))
  - update documentation for /ipfs -> /p2p multiaddr switch ([ipfs/go-ipfs#6538](https://github.com/ipfs/go-ipfs/pull/6538))
  - IPNS over PubSub as an Independent Transport ([ipfs/go-ipfs#6758](https://github.com/ipfs/go-ipfs/pull/6758))
  - docs: add information on how to enable experiments ([ipfs/go-ipfs#6792](https://github.com/ipfs/go-ipfs/pull/6792))
  - Change Reporter to BandwidthCounter in IpfsNode ([ipfs/go-ipfs#6793](https://github.com/ipfs/go-ipfs/pull/6793))
  - update go-datastore ([ipfs/go-ipfs#6791](https://github.com/ipfs/go-ipfs/pull/6791))
  - go fmt: go-ipfs-as-a-library ([ipfs/go-ipfs#6784](https://github.com/ipfs/go-ipfs/pull/6784))
  - feat: web ui 2.7.2 ([ipfs/go-ipfs#6778](https://github.com/ipfs/go-ipfs/pull/6778))
  - extract the pinner to go-ipfs-pinner and dagutils into go-merkledag ([ipfs/go-ipfs#6771](https://github.com/ipfs/go-ipfs/pull/6771))
  - fix #2203: omit the charset attribute when Content-Type is text/html ([ipfs/go-ipfs#6743](https://github.com/ipfs/go-ipfs/pull/6743))
  - Pin ls traverses all indirect pins ([ipfs/go-ipfs#6705](https://github.com/ipfs/go-ipfs/pull/6705))
  - fix: ignore nonexistant when force rm ([ipfs/go-ipfs#6773](https://github.com/ipfs/go-ipfs/pull/6773))
  - introduce IpfsNode Plugin ([ipfs/go-ipfs#6719](https://github.com/ipfs/go-ipfs/pull/6719))
  - improve documentation and fix dht put bug ([ipfs/go-ipfs#6750](https://github.com/ipfs/go-ipfs/pull/6750))
  - Adding alias for `ipfs repo stat`. ([ipfs/go-ipfs#6769](https://github.com/ipfs/go-ipfs/pull/6769))
  - doc(gateway): document dnslink ([ipfs/go-ipfs#6767](https://github.com/ipfs/go-ipfs/pull/6767))
  - pin: add context and error return to most of the Pinner functions ([ipfs/go-ipfs#6715](https://github.com/ipfs/go-ipfs/pull/6715))
  - feat: web ui 2.7.1 ([ipfs/go-ipfs#6762](https://github.com/ipfs/go-ipfs/pull/6762))
  - doc(README): document requirements for cross-compiling with OpenSSL support ([ipfs/go-ipfs#6738](https://github.com/ipfs/go-ipfs/pull/6738))
  - feat: web ui 2.6.0 ([ipfs/go-ipfs#6740](https://github.com/ipfs/go-ipfs/pull/6740))
  - Add high-level go-ipfs architecture diagram ([ipfs/go-ipfs#6727](https://github.com/ipfs/go-ipfs/pull/6727))
  - docs: remove extra ) on the example README ([ipfs/go-ipfs#6733](https://github.com/ipfs/go-ipfs/pull/6733))
  - update maintainer label ([ipfs/go-ipfs#6735](https://github.com/ipfs/go-ipfs/pull/6735))
  - ipfs namespace is now being provided to prometheus ([ipfs/go-ipfs#6643](https://github.com/ipfs/go-ipfs/pull/6643))
  - feat: web ui 2.5.8 ([ipfs/go-ipfs#6718](https://github.com/ipfs/go-ipfs/pull/6718))
  - docs: add connmgr to config.md toc ([ipfs/go-ipfs#6712](https://github.com/ipfs/go-ipfs/pull/6712))
  - feat: web ui 2.5.7 ([ipfs/go-ipfs#6707](https://github.com/ipfs/go-ipfs/pull/6707))
  - README: improve build documentation ([ipfs/go-ipfs#6706](https://github.com/ipfs/go-ipfs/pull/6706))
  - Introduce buzhash chunker       ([ipfs/go-ipfs#6701](https://github.com/ipfs/go-ipfs/pull/6701))
  - Pinning interop: Pin ls returns appropriate zero value ([ipfs/go-ipfs#6685](https://github.com/ipfs/go-ipfs/pull/6685))
  - fix(resolve): correctly handle .eth domains ([ipfs/go-ipfs#6700](https://github.com/ipfs/go-ipfs/pull/6700))
  - Update README.md ([ipfs/go-ipfs#6697](https://github.com/ipfs/go-ipfs/pull/6697))
  - daemon: support unix domain sockets for the API/gateway ([ipfs/go-ipfs#6678](https://github.com/ipfs/go-ipfs/pull/6678))
  - docs: guide users to the right locations for questions ([ipfs/go-ipfs#6691](https://github.com/ipfs/go-ipfs/pull/6691))
  - docs: readme improvements ([ipfs/go-ipfs#6693](https://github.com/ipfs/go-ipfs/pull/6693))
  - docs: link remaining docs available, guide people to the right locations ([ipfs/go-ipfs#6694](https://github.com/ipfs/go-ipfs/pull/6694))
  - docs: fix broken url ([ipfs/go-ipfs#6692](https://github.com/ipfs/go-ipfs/pull/6692))
  - add systemd support ([ipfs/go-ipfs#6675](https://github.com/ipfs/go-ipfs/pull/6675))
  - feat: add ipfs version info to prometheus metrics ([ipfs/go-ipfs#6688](https://github.com/ipfs/go-ipfs/pull/6688))
  - Fix typo ([ipfs/go-ipfs#6686](https://github.com/ipfs/go-ipfs/pull/6686))
  - github: migrate actions ([ipfs/go-ipfs#6681](https://github.com/ipfs/go-ipfs/pull/6681))
  - Add bridged chats ([ipfs/go-ipfs#6653](https://github.com/ipfs/go-ipfs/pull/6653))
  - doc(config): improve DisableNatPortMap documentation ([ipfs/go-ipfs#6655](https://github.com/ipfs/go-ipfs/pull/6655))
  - plugins: support Close() for Tracer plugins as well ([ipfs/go-ipfs#6672](https://github.com/ipfs/go-ipfs/pull/6672))
  - fix: make collect-profiles.sh work on mac ([ipfs/go-ipfs#6673](https://github.com/ipfs/go-ipfs/pull/6673))
  - namesys(test): test TTL on publish ([ipfs/go-ipfs#6671](https://github.com/ipfs/go-ipfs/pull/6671))
  - discovery: improve mdns warnings ([ipfs/go-ipfs#6665](https://github.com/ipfs/go-ipfs/pull/6665))
  - feat: web ui 2.5.4 ([ipfs/go-ipfs#6664](https://github.com/ipfs/go-ipfs/pull/6664))
  - cmds(help): fix swarm filter add/rm help text ([ipfs/go-ipfs#6654](https://github.com/ipfs/go-ipfs/pull/6654))
  - feat: webui 2.5.3 ([ipfs/go-ipfs#6638](https://github.com/ipfs/go-ipfs/pull/6638))
  - feat: web ui 2.5.1 ([ipfs/go-ipfs#6630](https://github.com/ipfs/go-ipfs/pull/6630))
  - docs: add multiple gateway and api addrs ([ipfs/go-ipfs#6631](https://github.com/ipfs/go-ipfs/pull/6631))
  - doc: add post-release checklist ([ipfs/go-ipfs#6625](https://github.com/ipfs/go-ipfs/pull/6625))
  - docs: add ship date and next release issue opening time ([ipfs/go-ipfs#6620](https://github.com/ipfs/go-ipfs/pull/6620))
  - docker: libdl dependency ([ipfs/go-ipfs#6624](https://github.com/ipfs/go-ipfs/pull/6624))
  - docs: improvements to the release doc ([ipfs/go-ipfs#6616](https://github.com/ipfs/go-ipfs/pull/6616))
  - plugins: add support for plugin configs ([ipfs/go-ipfs#6613](https://github.com/ipfs/go-ipfs/pull/6613))
  - Update README.md ([ipfs/go-ipfs#6615](https://github.com/ipfs/go-ipfs/pull/6615))
  - doc: remove gmake instructions ([ipfs/go-ipfs#6614](https://github.com/ipfs/go-ipfs/pull/6614))
  - feat: add ability to use existing config during init ([ipfs/go-ipfs#6489](https://github.com/ipfs/go-ipfs/pull/6489))
  - doc: expand and cleanup badger documentation ([ipfs/go-ipfs#6611](https://github.com/ipfs/go-ipfs/pull/6611))
  - feat: improve plugin preload logic ([ipfs/go-ipfs#6576](https://github.com/ipfs/go-ipfs/pull/6576))
  - version: don't print 'VERSION-' if no commit is specified ([ipfs/go-ipfs#6609](https://github.com/ipfs/go-ipfs/pull/6609))
  - Update go-libp2p, fix tests with weak RSA keys ([ipfs/go-ipfs#6555](https://github.com/ipfs/go-ipfs/pull/6555))
  - cmds/refs: fix ipfs refs for sharded directories ([ipfs/go-ipfs#6601](https://github.com/ipfs/go-ipfs/pull/6601))
  - fix: spammy mock when testing ([ipfs/go-ipfs#6583](https://github.com/ipfs/go-ipfs/pull/6583))
  - docker: update the docker image ([ipfs/go-ipfs#6582](https://github.com/ipfs/go-ipfs/pull/6582))
  - add release process graphic ([ipfs/go-ipfs#6568](https://github.com/ipfs/go-ipfs/pull/6568))
  - feat: web ui 2.5.0 ([ipfs/go-ipfs#6566](https://github.com/ipfs/go-ipfs/pull/6566))
  - Add swarm key variables to container daemon ([ipfs/go-ipfs#6554](https://github.com/ipfs/go-ipfs/pull/6554))
  - doc: update the release template ([ipfs/go-ipfs#6561](https://github.com/ipfs/go-ipfs/pull/6561))
  - merge changelog and bump version ([ipfs/go-ipfs#6559](https://github.com/ipfs/go-ipfs/pull/6559))
  - require GNU make ([ipfs/go-ipfs#6551](https://github.com/ipfs/go-ipfs/pull/6551))
  - tweak the release process ([ipfs/go-ipfs#6553](https://github.com/ipfs/go-ipfs/pull/6553))
  - Allow resolution of .eth names via .eth.link ([ipfs/go-ipfs#6448](https://github.com/ipfs/go-ipfs/pull/6448))
  - README: update minimum system requirements and recommend OpenSSL ([ipfs/go-ipfs#6543](https://github.com/ipfs/go-ipfs/pull/6543))
  - fix and improve the writable gateway ([ipfs/go-ipfs#6539](https://github.com/ipfs/go-ipfs/pull/6539))
  - feat: add install instructions for external commands ([ipfs/go-ipfs#6541](https://github.com/ipfs/go-ipfs/pull/6541))
  - fix: slightly faster gc ([ipfs/go-ipfs#6505](https://github.com/ipfs/go-ipfs/pull/6505))
  - fix {net,open}bsd build by disabling fuse on openbsd ([ipfs/go-ipfs#6535](https://github.com/ipfs/go-ipfs/pull/6535))
  - mk: handle stripping paths when GOPATH contains whitespace ([ipfs/go-ipfs#6536](https://github.com/ipfs/go-ipfs/pull/6536))
  - make gossipsub the default routing protocol for pubsub ([ipfs/go-ipfs#6512](https://github.com/ipfs/go-ipfs/pull/6512))
  - doc: align the early testers program description with its goal ([ipfs/go-ipfs#6529](https://github.com/ipfs/go-ipfs/pull/6529))
  - feat: add --long as alias for -l in files.ls ([ipfs/go-ipfs#6528](https://github.com/ipfs/go-ipfs/pull/6528))
  - switch to new merkledag walk functions ([ipfs/go-ipfs#6499](https://github.com/ipfs/go-ipfs/pull/6499))
  - readme: fix CI badge ([ipfs/go-ipfs#6521](https://github.com/ipfs/go-ipfs/pull/6521))
  - Adds Siderus in early testers ([ipfs/go-ipfs#6517](https://github.com/ipfs/go-ipfs/pull/6517))
  - Extract Filestore ([ipfs/go-ipfs#6511](https://github.com/ipfs/go-ipfs/pull/6511))
  - readme: fix scoop bucket command error ([ipfs/go-ipfs#6510](https://github.com/ipfs/go-ipfs/pull/6510))
  - sharness: test pin ls stream ([ipfs/go-ipfs#6504](https://github.com/ipfs/go-ipfs/pull/6504))
  - Improve pin/update description ([ipfs/go-ipfs#6501](https://github.com/ipfs/go-ipfs/pull/6501))
  - pin cmd: stream recursive pins ([ipfs/go-ipfs#6493](https://github.com/ipfs/go-ipfs/pull/6493))
  - Document the AddrFilters option ([ipfs/go-ipfs#6459](https://github.com/ipfs/go-ipfs/pull/6459))
  - feat: make it easier to load custom plugins ([ipfs/go-ipfs#6474](https://github.com/ipfs/go-ipfs/pull/6474))
  - document the debug script ([ipfs/go-ipfs#6486](https://github.com/ipfs/go-ipfs/pull/6486))
  - Extract provider module to `go-ipfs-provider` ([ipfs/go-ipfs#6421](https://github.com/ipfs/go-ipfs/pull/6421))
  - ignore stale API files and deprecate ipfs repo fsck ([ipfs/go-ipfs#6478](https://github.com/ipfs/go-ipfs/pull/6478))
  - Fix node construction queue error ([ipfs/go-ipfs#6480](https://github.com/ipfs/go-ipfs/pull/6480))
  - Update the required go version in the README ([ipfs/go-ipfs#6462](https://github.com/ipfs/go-ipfs/pull/6462))
  - gitmodules: use https so we don't need an ssh key ([ipfs/go-ipfs#6450](https://github.com/ipfs/go-ipfs/pull/6450))
  - doc: add another Windows package to README ([ipfs/go-ipfs#6440](https://github.com/ipfs/go-ipfs/pull/6440))
  - Close started plugins when one of them fails to start. ([ipfs/go-ipfs#6438](https://github.com/ipfs/go-ipfs/pull/6438))
  - Load plugins on darwin/macOS ([ipfs/go-ipfs#6439](https://github.com/ipfs/go-ipfs/pull/6439))
  - assets: move away from gx ([ipfs/go-ipfs#6414](https://github.com/ipfs/go-ipfs/pull/6414))
  - Fix a typo ([ipfs/go-ipfs#6432](https://github.com/ipfs/go-ipfs/pull/6432))
  - docs: fix install guide link ([ipfs/go-ipfs#6423](https://github.com/ipfs/go-ipfs/pull/6423))
  - Deps: update go-libp2p-http to its new libp2p location ([ipfs/go-ipfs#6422](https://github.com/ipfs/go-ipfs/pull/6422))
  - install.sh: Fix wrong destination path for ipfs binary ([ipfs/go-ipfs#6424](https://github.com/ipfs/go-ipfs/pull/6424))
  - build: strip GOPATH from build paths ([ipfs/go-ipfs#6412](https://github.com/ipfs/go-ipfs/pull/6412))
  - libp2p: moves discovery after host listen ([ipfs/go-ipfs#6415](https://github.com/ipfs/go-ipfs/pull/6415))
  - remove mentions of gx from windows build docs ([ipfs/go-ipfs#6413](https://github.com/ipfs/go-ipfs/pull/6413))
  - build: use protoc-gen-* from gomod ([ipfs/go-ipfs#6411](https://github.com/ipfs/go-ipfs/pull/6411))
  - add unixfs get metric ([ipfs/go-ipfs#6406](https://github.com/ipfs/go-ipfs/pull/6406))
  - Run JS interop in CircleCI ([ipfs/go-ipfs#6409](https://github.com/ipfs/go-ipfs/pull/6409))
  - Usage of context helper in Blockstore provider ([ipfs/go-ipfs#6399](https://github.com/ipfs/go-ipfs/pull/6399))
  - docs: default value for HashOnRead is false ([ipfs/go-ipfs#6401](https://github.com/ipfs/go-ipfs/pull/6401))
  - block cmd: allow adding multiple blocks at once ([ipfs/go-ipfs#6331](https://github.com/ipfs/go-ipfs/pull/6331))
  - Remove Repo from routing fx provider parameter ([ipfs/go-ipfs#6395](https://github.com/ipfs/go-ipfs/pull/6395))
  - migrate to go-libp2p-core. ([ipfs/go-ipfs#6384](https://github.com/ipfs/go-ipfs/pull/6384))
  - feat: update Web UI to v2.4.6 ([ipfs/go-ipfs#6392](https://github.com/ipfs/go-ipfs/pull/6392))
  - Introduce first strategic provider: do nothing ([ipfs/go-ipfs#6292](https://github.com/ipfs/go-ipfs/pull/6292))
- github.com/ipfs/go-bitswap (v0.0.8-e37498cf10d6 -> v0.2.13):
  - refactor: remove WantManager ([ipfs/go-bitswap#374](https://github.com/ipfs/go-bitswap/pull/374))
  - Send CANCELs when session context is cancelled ([ipfs/go-bitswap#375](https://github.com/ipfs/go-bitswap/pull/375))
  - refactor: remove unused code ([ipfs/go-bitswap#373](https://github.com/ipfs/go-bitswap/pull/373))
  - Change timing for DONT_HAVE timeouts to be more conservative ([ipfs/go-bitswap#371](https://github.com/ipfs/go-bitswap/pull/371))
  - fix: avoid calling ctx.SetDeadline() every time we send a message ([ipfs/go-bitswap#369](https://github.com/ipfs/go-bitswap/pull/369))
  - feat: optimize entry sorting in MessageQueue ([ipfs/go-bitswap#356](https://github.com/ipfs/go-bitswap/pull/356))
  - Move connection management into networking layer ([ipfs/go-bitswap#351](https://github.com/ipfs/go-bitswap/pull/351))
  - refactor: simplify messageQueue onSent ([ipfs/go-bitswap#349](https://github.com/ipfs/go-bitswap/pull/349))
  - feat: prioritize more important wants ([ipfs/go-bitswap#346](https://github.com/ipfs/go-bitswap/pull/346))
  - fix: in message queue only send cancel if want was sent ([ipfs/go-bitswap#345](https://github.com/ipfs/go-bitswap/pull/345))
  - fix: ensure wantlist gauge gets decremented on disconnect ([ipfs/go-bitswap#332](https://github.com/ipfs/go-bitswap/pull/332))
  - avoid copying messages and improve logging ([ipfs/go-bitswap#326](https://github.com/ipfs/go-bitswap/pull/326))
  - fix: log unexpected condition in peerWantManager.prepareSendWants() ([ipfs/go-bitswap#325](https://github.com/ipfs/go-bitswap/pull/325))
  - wait for sessionWantSender to shutdown before completing session shutdown ([ipfs/go-bitswap#317](https://github.com/ipfs/go-bitswap/pull/317))
  - Perf/message queue ([ipfs/go-bitswap#307](https://github.com/ipfs/go-bitswap/pull/307))
  - feat: add a custom CID type ([ipfs/go-bitswap#308](https://github.com/ipfs/go-bitswap/pull/308))
  - feat: expose the full wantlist through GetWantlist ([ipfs/go-bitswap#300](https://github.com/ipfs/go-bitswap/pull/300))
  - Clean up logs ([ipfs/go-bitswap#299](https://github.com/ipfs/go-bitswap/pull/299))
  - Fix order of session broadcast wants ([ipfs/go-bitswap#291](https://github.com/ipfs/go-bitswap/pull/291))
  - fix flaky TestRateLimitingRequests ([ipfs/go-bitswap#296](https://github.com/ipfs/go-bitswap/pull/296))
  - fix flaky TestDontHaveTimeoutMgrTimeout ([ipfs/go-bitswap#293](https://github.com/ipfs/go-bitswap/pull/293))
  - fix: re-export testinstance/testnet ([ipfs/go-bitswap#289](https://github.com/ipfs/go-bitswap/pull/289))
  - Simulate DONT_HAVE when peer doesn't respond to want-block (new peers) ([ipfs/go-bitswap#284](https://github.com/ipfs/go-bitswap/pull/284))
  - Be less aggressive when pruning peers from session ([ipfs/go-bitswap#276](https://github.com/ipfs/go-bitswap/pull/276))
  - fix: races in tests ([ipfs/go-bitswap#279](https://github.com/ipfs/go-bitswap/pull/279))
  - Refactor: simplify session peer management ([ipfs/go-bitswap#275](https://github.com/ipfs/go-bitswap/pull/275))
  - Prune peers that send too many consecutive DONT_HAVEs ([ipfs/go-bitswap#261](https://github.com/ipfs/go-bitswap/pull/261))
  - feat: debounce wants manually ([ipfs/go-bitswap#255](https://github.com/ipfs/go-bitswap/pull/255))
  - Fix bug with signaling peer availability to sessions ([ipfs/go-bitswap#247](https://github.com/ipfs/go-bitswap/pull/247))
  - feat: move internals to an internal package ([ipfs/go-bitswap#242](https://github.com/ipfs/go-bitswap/pull/242))
  - PoC of Bitswap protocol extensions implementation ([ipfs/go-bitswap#189](https://github.com/ipfs/go-bitswap/pull/189))
  - fix: abort when the context is canceled while getting blocks ([ipfs/go-bitswap#240](https://github.com/ipfs/go-bitswap/pull/240))
  - Add bridged chats ([ipfs/go-bitswap#198](https://github.com/ipfs/go-bitswap/pull/198))
  - reduce session contention ([ipfs/go-bitswap#188](https://github.com/ipfs/go-bitswap/pull/188))
  - Fix: don't ignore received blocks for pending wants ([ipfs/go-bitswap#174](https://github.com/ipfs/go-bitswap/pull/174))
  - Test: fix flakey session peer manager tests ([ipfs/go-bitswap#185](https://github.com/ipfs/go-bitswap/pull/185))
  - Refactor: use global pubsub notifier ([ipfs/go-bitswap#177](https://github.com/ipfs/go-bitswap/pull/177))
  - network: Allow specifying protocol prefix ([ipfs/go-bitswap#171](https://github.com/ipfs/go-bitswap/pull/171))
  - fix: memory leak in latency tracker on timeout after cancel ([ipfs/go-bitswap#164](https://github.com/ipfs/go-bitswap/pull/164))
  - Fix typo ([ipfs/go-bitswap#158](https://github.com/ipfs/go-bitswap/pull/158))
  - Feat: Track Session Peer Latency More Accurately ([ipfs/go-bitswap#149](https://github.com/ipfs/go-bitswap/pull/149))
  - ci(circleci): add benchmark comparisons ([ipfs/go-bitswap#147](https://github.com/ipfs/go-bitswap/pull/147))
  - aggressively free memory ([ipfs/go-bitswap#143](https://github.com/ipfs/go-bitswap/pull/143))
  - Enchanced logging for bitswap ([ipfs/go-bitswap#137](https://github.com/ipfs/go-bitswap/pull/137))
  - fix: rand.Intn(0) panics ([ipfs/go-bitswap#144](https://github.com/ipfs/go-bitswap/pull/144))
  - fix some naming nits and broadcast on search ([ipfs/go-bitswap#139](https://github.com/ipfs/go-bitswap/pull/139))
  - feat(sessions): add rebroadcasting, search backoff ([ipfs/go-bitswap#133](https://github.com/ipfs/go-bitswap/pull/133))
  - testutil: fix block generator ([ipfs/go-bitswap#135](https://github.com/ipfs/go-bitswap/pull/135))
  - migrate to go-libp2p-core. ([ipfs/go-bitswap#132](https://github.com/ipfs/go-bitswap/pull/132))
- github.com/ipfs/go-blockservice (v0.0.3 -> v0.1.3):
  - fix ci badge and lints ([ipfs/go-blockservice#52](https://github.com/ipfs/go-blockservice/pull/52))
  - demote warning to debug log ([ipfs/go-blockservice#30](https://github.com/ipfs/go-blockservice/pull/30))
  - nil exchange is okay ([ipfs/go-blockservice#29](https://github.com/ipfs/go-blockservice/pull/29))
  - set the session context ([ipfs/go-blockservice#28](https://github.com/ipfs/go-blockservice/pull/28))
  - make blockservice AddBlocks return more quickly ([ipfs/go-blockservice#10](https://github.com/ipfs/go-blockservice/pull/10))
  - feat(session): instantiated sessions lazily ([ipfs/go-blockservice#27](https://github.com/ipfs/go-blockservice/pull/27))
- github.com/ipfs/go-cid (v0.0.4 -> v0.0.5):
  - fix: enforce minimal encoding ([ipfs/go-cid#99](https://github.com/ipfs/go-cid/pull/99))
- github.com/ipfs/go-datastore (v0.0.5 -> v0.4.4):
  - Fix test log message about number of values put ([ipfs/go-datastore#150](https://github.com/ipfs/go-datastore/pull/150))
  - test suite: Add ElemCount to control how many elements are added. ([ipfs/go-datastore#151](https://github.com/ipfs/go-datastore/pull/151))
  - fix: avoid filtering by prefix unless necessary ([ipfs/go-datastore#147](https://github.com/ipfs/go-datastore/pull/147))
  - feat: add upper-case keys at a known prefix ([ipfs/go-datastore#148](https://github.com/ipfs/go-datastore/pull/148))
  - test(suite): add a bunch of prefix tests for the new behavior ([ipfs/go-datastore#145](https://github.com/ipfs/go-datastore/pull/145))
  - Only count a key as an ancestor if there is a separator ([ipfs/go-datastore#141](https://github.com/ipfs/go-datastore/pull/141))
  - fix go-check path to use "gopkg.in/check.v1" ([ipfs/go-datastore#144](https://github.com/ipfs/go-datastore/pull/144))
  - LogDatastore fulfills the Datastore interface again ([ipfs/go-datastore#142](https://github.com/ipfs/go-datastore/pull/142))
  - Support Asynchronous Writing Datastores ([ipfs/go-datastore#140](https://github.com/ipfs/go-datastore/pull/140))
  - add a Size field to Query's Result ([ipfs/go-datastore#134](https://github.com/ipfs/go-datastore/pull/134))
  - Add clarifying comments on Query#String() ([ipfs/go-datastore#138](https://github.com/ipfs/go-datastore/pull/138))
  - Add a large test suite ([ipfs/go-datastore#136](https://github.com/ipfs/go-datastore/pull/136))
  - doc: add a lead maintainer ([ipfs/go-datastore#135](https://github.com/ipfs/go-datastore/pull/135))
  - feat: make not-found errors discoverable ([ipfs/go-datastore#133](https://github.com/ipfs/go-datastore/pull/133))
  - feat: make delete idempotent ([ipfs/go-datastore#132](https://github.com/ipfs/go-datastore/pull/132))
  - Misc Typo Fixes ([ipfs/go-datastore#131](https://github.com/ipfs/go-datastore/pull/131))
- github.com/ipfs/go-ds-badger (v0.0.5 -> v0.2.4):
  - fix: verify that the datastore is still open when querying ([ipfs/go-ds-badger#87](https://github.com/ipfs/go-ds-badger/pull/87))
  - feat: switch to file io and shrink tables ([ipfs/go-ds-badger#83](https://github.com/ipfs/go-ds-badger/pull/83))
  - fix: update go-datastore ([ipfs/go-ds-badger#80](https://github.com/ipfs/go-ds-badger/pull/80))
  - update datastore Interface ([ipfs/go-ds-badger#77](https://github.com/ipfs/go-ds-badger/pull/77))
  - query: always return the size ([ipfs/go-ds-badger#78](https://github.com/ipfs/go-ds-badger/pull/78))
  - feat(gc): make it possible to disable GC ([ipfs/go-ds-badger#74](https://github.com/ipfs/go-ds-badger/pull/74))
  - feat(gc): improve periodic GC logic ([ipfs/go-ds-badger#73](https://github.com/ipfs/go-ds-badger/pull/73))
  - periodic GC for badger datastore ([ipfs/go-ds-badger#72](https://github.com/ipfs/go-ds-badger/pull/72))
  - Fix combining query filters, offsets, and limits ([ipfs/go-ds-badger#71](https://github.com/ipfs/go-ds-badger/pull/71))
  - doc: add lead maintainer ([ipfs/go-ds-badger#67](https://github.com/ipfs/go-ds-badger/pull/67))
- github.com/ipfs/go-ds-flatfs (v0.0.2 -> v0.4.4):
  - move retries lower and retry rename ops ([ipfs/go-ds-flatfs#82](https://github.com/ipfs/go-ds-flatfs/pull/82))
  - cleanup putMany implementation ([ipfs/go-ds-flatfs#80](https://github.com/ipfs/go-ds-flatfs/pull/80))
  - feat: read harder ([ipfs/go-ds-flatfs#78](https://github.com/ipfs/go-ds-flatfs/pull/78))
  - fix: remove temporary files when multiple write operations conflict ([ipfs/go-ds-flatfs#76](https://github.com/ipfs/go-ds-flatfs/pull/76))
  - Windows CI + Fixes ([ipfs/go-ds-flatfs#73](https://github.com/ipfs/go-ds-flatfs/pull/73))
  - fix: close query when finished moving ([ipfs/go-ds-flatfs#74](https://github.com/ipfs/go-ds-flatfs/pull/74))
  - fix: ensure that we close the diskusage file, even if we fail to rename it ([ipfs/go-ds-flatfs#72](https://github.com/ipfs/go-ds-flatfs/pull/72))
  - feat: put all temporary files in the same directory and clean them up ([ipfs/go-ds-flatfs#69](https://github.com/ipfs/go-ds-flatfs/pull/69))
  - fix: only log when we find a file we don't expect ([ipfs/go-ds-flatfs#68](https://github.com/ipfs/go-ds-flatfs/pull/68))
  - Make flatfs robust ([ipfs/go-ds-flatfs#64](https://github.com/ipfs/go-ds-flatfs/pull/64))
  - Update Datastore Interface ([ipfs/go-ds-flatfs#60](https://github.com/ipfs/go-ds-flatfs/pull/60))
  - query: deny ReturnsSizes and ReturnExpirations instead of returning wrong result ([ipfs/go-ds-flatfs#59](https://github.com/ipfs/go-ds-flatfs/pull/59))
  - doc: add a lead maintainer ([ipfs/go-ds-flatfs#55](https://github.com/ipfs/go-ds-flatfs/pull/55))
  - make delete idempotent ([ipfs/go-ds-flatfs#54](https://github.com/ipfs/go-ds-flatfs/pull/54))
- github.com/ipfs/go-ds-leveldb (v0.0.2 -> v0.4.2):
  - prevent closing concurrently with other operations. ([ipfs/go-ds-leveldb#42](https://github.com/ipfs/go-ds-leveldb/pull/42))
  - feat: update go-datastore ([ipfs/go-ds-leveldb#40](https://github.com/ipfs/go-ds-leveldb/pull/40))
  - update datastore Interface ([ipfs/go-ds-leveldb#36](https://github.com/ipfs/go-ds-leveldb/pull/36))
  - query: always return the size ([ipfs/go-ds-leveldb#35](https://github.com/ipfs/go-ds-leveldb/pull/35))
  - doc: add a lead maintainer ([ipfs/go-ds-leveldb#31](https://github.com/ipfs/go-ds-leveldb/pull/31))
  - make delete idempotent ([ipfs/go-ds-leveldb#30](https://github.com/ipfs/go-ds-leveldb/pull/30))
- github.com/ipfs/go-ds-measure (v0.0.1 -> v0.1.0):
  - update datastore Interface ([ipfs/go-ds-measure#23](https://github.com/ipfs/go-ds-measure/pull/23))
  - Add Datastore Tests ([ipfs/go-ds-measure#24](https://github.com/ipfs/go-ds-measure/pull/24))
  - fix GetSize calls reported as Has ([ipfs/go-ds-measure#20](https://github.com/ipfs/go-ds-measure/pull/20))
- github.com/ipfs/go-fs-lock (v0.0.1 -> v0.0.4):
  - fix: revert small breaking change ([ipfs/go-fs-lock#10](https://github.com/ipfs/go-fs-lock/pull/10))
  - Enh/improve error handling ([ipfs/go-fs-lock#9](https://github.com/ipfs/go-fs-lock/pull/9))
  - Use path/filepath instead of path ([ipfs/go-fs-lock#8](https://github.com/ipfs/go-fs-lock/pull/8))
- github.com/ipfs/go-ipfs-blockstore (v0.0.1 -> v0.1.4):
  - return the correct size when only "has" is cached ([ipfs/go-ipfs-blockstore#36](https://github.com/ipfs/go-ipfs-blockstore/pull/36))
  - cache: switch to 2q ([ipfs/go-ipfs-blockstore#20](https://github.com/ipfs/go-ipfs-blockstore/pull/20))
- github.com/ipfs/go-ipfs-chunker (v0.0.1 -> v0.0.5):
  - fix: don't return an empty block at the end ([ipfs/go-ipfs-chunker#22](https://github.com/ipfs/go-ipfs-chunker/pull/22))
  - Rigorous sizing checks ([ipfs/go-ipfs-chunker#21](https://github.com/ipfs/go-ipfs-chunker/pull/21))
  - Improve performance of buzhash ([ipfs/go-ipfs-chunker#17](https://github.com/ipfs/go-ipfs-chunker/pull/17))
  - Implement buzhash ([ipfs/go-ipfs-chunker#16](https://github.com/ipfs/go-ipfs-chunker/pull/16))
  - Add benchmarks ([ipfs/go-ipfs-chunker#15](https://github.com/ipfs/go-ipfs-chunker/pull/15))
- github.com/ipfs/go-ipfs-cmds (v0.0.8 -> v0.2.2):
  - Fix: disallow POST without Origin nor Referer from specific user agents ([ipfs/go-ipfs-cmds#193](https://github.com/ipfs/go-ipfs-cmds/pull/193))
  - doc: document command fields ([ipfs/go-ipfs-cmds#192](https://github.com/ipfs/go-ipfs-cmds/pull/192))
  - change HandledMethods to AllowGet and cleanup method handling ([ipfs/go-ipfs-cmds#191](https://github.com/ipfs/go-ipfs-cmds/pull/191))
  - remove deprecated log.Warning(f) ([ipfs/go-ipfs-cmds#180](https://github.com/ipfs/go-ipfs-cmds/pull/180))
  - http: configurable allowed request methods for the API. ([ipfs/go-ipfs-cmds#190](https://github.com/ipfs/go-ipfs-cmds/pull/190))
  - #183 refactored the request options conversion code per the ticket requirements ([ipfs/go-ipfs-cmds#187](https://github.com/ipfs/go-ipfs-cmds/pull/187))
  - fix typo ([ipfs/go-ipfs-cmds#188](https://github.com/ipfs/go-ipfs-cmds/pull/188))
  -  ([ipfs/go-ipfs-cmds#183](https://github.com/ipfs/go-ipfs-cmds/pull/183))
  - fix: normalize options when parsing them ([ipfs/go-ipfs-cmds#186](https://github.com/ipfs/go-ipfs-cmds/pull/186))
  - feat:add strings option; re-implement file ignore ([ipfs/go-ipfs-cmds#181](https://github.com/ipfs/go-ipfs-cmds/pull/181))
  - Special-case accepting explicitly supplied named pipes ([ipfs/go-ipfs-cmds#184](https://github.com/ipfs/go-ipfs-cmds/pull/184))
  - Chore/remove gx ([ipfs/go-ipfs-cmds#182](https://github.com/ipfs/go-ipfs-cmds/pull/182))
  - http: allow specifying a custom http client ([ipfs/go-ipfs-cmds#175](https://github.com/ipfs/go-ipfs-cmds/pull/175))
  - http: cleanup http related errors ([ipfs/go-ipfs-cmds#173](https://github.com/ipfs/go-ipfs-cmds/pull/173))
  - fix: too many arguments error text ([ipfs/go-ipfs-cmds#172](https://github.com/ipfs/go-ipfs-cmds/pull/172))
  - fallback executor support ([ipfs/go-ipfs-cmds#171](https://github.com/ipfs/go-ipfs-cmds/pull/171))
  - make ErrorType a valid error and implement Unwrap on Error ([ipfs/go-ipfs-cmds#170](https://github.com/ipfs/go-ipfs-cmds/pull/170))
  - feat: improve error codes ([ipfs/go-ipfs-cmds#168](https://github.com/ipfs/go-ipfs-cmds/pull/168))
  - Fix a typo ([ipfs/go-ipfs-cmds#169](https://github.com/ipfs/go-ipfs-cmds/pull/169))
- github.com/ipfs/go-ipfs-config (v0.0.3 -> v0.5.3):
  - fix: correct the default-datastore config profile ([ipfs/go-ipfs-config#80](https://github.com/ipfs/go-ipfs-config/pull/80))
  - feat: disable autonat service when in lowpower mode ([ipfs/go-ipfs-config#77](https://github.com/ipfs/go-ipfs-config/pull/77))
  - feat: add and use a duration helper type ([ipfs/go-ipfs-config#76](https://github.com/ipfs/go-ipfs-config/pull/76))
  - feat: add an autonat config section ([ipfs/go-ipfs-config#75](https://github.com/ipfs/go-ipfs-config/pull/75))
  - feat: remove Routing.PrivateType ([ipfs/go-ipfs-config#74](https://github.com/ipfs/go-ipfs-config/pull/74))
  - feat: add private routing config field ([ipfs/go-ipfs-config#73](https://github.com/ipfs/go-ipfs-config/pull/73))
  - feat: mark badger as stable ([ipfs/go-ipfs-config#70](https://github.com/ipfs/go-ipfs-config/pull/70))
  - feat: remove PreferTLS experiment ([ipfs/go-ipfs-config#71](https://github.com/ipfs/go-ipfs-config/pull/71))
  - feat: remove old bootstrap peers ([ipfs/go-ipfs-config#67](https://github.com/ipfs/go-ipfs-config/pull/67))
  - add config options for proxy/subdomain ([ipfs/go-ipfs-config#30](https://github.com/ipfs/go-ipfs-config/pull/30))
  - feat: add graphsync option ([ipfs/go-ipfs-config#62](https://github.com/ipfs/go-ipfs-config/pull/62))
  - profile: badger profile now defaults to asynchronous writes ([ipfs/go-ipfs-config#60](https://github.com/ipfs/go-ipfs-config/pull/60))
  - migrate multiaddrs from /ipfs -> /p2p ([ipfs/go-ipfs-config#39](https://github.com/ipfs/go-ipfs-config/pull/39))
  - use key size constraints defined in libp2p ([ipfs/go-ipfs-config#57](https://github.com/ipfs/go-ipfs-config/pull/57))
  - plugins: don't omit empty config values ([ipfs/go-ipfs-config#46](https://github.com/ipfs/go-ipfs-config/pull/46))
  - make it easier to detect an uninitialized repo ([ipfs/go-ipfs-config#45](https://github.com/ipfs/go-ipfs-config/pull/45))
  - nit: omit empty plugin values ([ipfs/go-ipfs-config#44](https://github.com/ipfs/go-ipfs-config/pull/44))
  - add plugins config section ([ipfs/go-ipfs-config#43](https://github.com/ipfs/go-ipfs-config/pull/43))
  - Add very basic (possibly temporary) Provider configs ([ipfs/go-ipfs-config#38](https://github.com/ipfs/go-ipfs-config/pull/38))
  - fix string formatting of bootstrap peers ([ipfs/go-ipfs-config#37](https://github.com/ipfs/go-ipfs-config/pull/37))
  - migrate to the consolidated libp2p ([ipfs/go-ipfs-config#36](https://github.com/ipfs/go-ipfs-config/pull/36))
  - Add strategic provider system experiment flag ([ipfs/go-ipfs-config#33](https://github.com/ipfs/go-ipfs-config/pull/33))
- github.com/ipfs/go-ipfs-files (v0.0.3 -> v0.0.8):
  - skip ignored files when calculating size ([ipfs/go-ipfs-files#30](https://github.com/ipfs/go-ipfs-files/pull/30))
  - Feat/add ignore rules ([ipfs/go-ipfs-files#26](https://github.com/ipfs/go-ipfs-files/pull/26))
  - revert(symlink): keep stat argument ([ipfs/go-ipfs-files#23](https://github.com/ipfs/go-ipfs-files/pull/23))
  - feat: correctly report the size of symlinks ([ipfs/go-ipfs-files#22](https://github.com/ipfs/go-ipfs-files/pull/22))
  - serialfile: fix handling of hidden paths on windows ([ipfs/go-ipfs-files#21](https://github.com/ipfs/go-ipfs-files/pull/21))
  - feat: add WriteTo function ([ipfs/go-ipfs-files#20](https://github.com/ipfs/go-ipfs-files/pull/20))
  - doc: fix formdata documentation ([ipfs/go-ipfs-files#19](https://github.com/ipfs/go-ipfs-files/pull/19))
- github.com/ipfs/go-ipfs-pinner (v0.0.1 -> v0.0.4):
  - fix: don't hold the pin lock while updating pins ([ipfs/go-ipfs-pinner#2](https://github.com/ipfs/go-ipfs-pinner/pull/2))
- github.com/ipfs/go-ipfs-pq (v0.0.1 -> v0.0.2):
  - Remove() ([ipfs/go-ipfs-pq#5](https://github.com/ipfs/go-ipfs-pq/pull/5))
  - Fix Peek() test ([ipfs/go-ipfs-pq#4](https://github.com/ipfs/go-ipfs-pq/pull/4))
  - add Peek() method ([ipfs/go-ipfs-pq#3](https://github.com/ipfs/go-ipfs-pq/pull/3))
  - add gomod support // tag v0.0.1. ([ipfs/go-ipfs-pq#1](https://github.com/ipfs/go-ipfs-pq/pull/1))
- github.com/ipfs/go-ipfs-routing (v0.0.1 -> v0.1.0):
  - migrate to go-libp2p-core ([ipfs/go-ipfs-routing#22](https://github.com/ipfs/go-ipfs-routing/pull/22))
- github.com/ipfs/go-ipld-cbor (v0.0.2 -> v0.0.4):
  - doc: add a lead maintainer ([ipfs/go-ipld-cbor#65](https://github.com/ipfs/go-ipld-cbor/pull/65))
  - fastpath CBOR ([ipfs/go-ipld-cbor#64](https://github.com/ipfs/go-ipld-cbor/pull/64))
- github.com/ipfs/go-ipld-format (v0.0.2 -> v0.2.0):
  - fix: change the batch size to avoid buffering too much ([ipfs/go-ipld-format#56](https://github.com/ipfs/go-ipld-format/pull/56))
  - doc: add a lead maintainer ([ipfs/go-ipld-format#54](https://github.com/ipfs/go-ipld-format/pull/54))
- github.com/ipfs/go-ipld-git (v0.0.2 -> v0.0.3):
  - Use RFC3339 to format dates, fixes #16 ([ipfs/go-ipld-git#32](https://github.com/ipfs/go-ipld-git/pull/32))
  - doc: add a lead maintainer ([ipfs/go-ipld-git#41](https://github.com/ipfs/go-ipld-git/pull/41))
- github.com/ipfs/go-ipns (v0.0.1 -> v0.0.2):
  - readme: add a lead maintainer ([ipfs/go-ipns#25](https://github.com/ipfs/go-ipns/pull/25))
- github.com/ipfs/go-log (v0.0.1 -> v1.0.4):
  - add IPFS_* env vars back for transitionary release of go-log ([ipfs/go-log#67](https://github.com/ipfs/go-log/pull/67))
  - Experimental: zap backend for go-log ([ipfs/go-log#61](https://github.com/ipfs/go-log/pull/61))
  - Spelling fix ([ipfs/go-log#63](https://github.com/ipfs/go-log/pull/63))
  - Deprecate EventLogging and Warning* functions ([ipfs/go-log#62](https://github.com/ipfs/go-log/pull/62))
- github.com/ipfs/go-merkledag (v0.0.3 -> v0.3.2):
  - fix: correctly construct sessions ([ipfs/go-merkledag#56](https://github.com/ipfs/go-merkledag/pull/56))
  - Migrate dagutils from go-ipfs ([ipfs/go-merkledag#50](https://github.com/ipfs/go-merkledag/pull/50))
  - Make getPBNode Public ([ipfs/go-merkledag#49](https://github.com/ipfs/go-merkledag/pull/49))
  - Pull In Upstream Changes ([ipfs/go-merkledag#1](https://github.com/ipfs/go-merkledag/pull/1))
  - fix: slightly reduce memory usage when walking large directory trees ([ipfs/go-merkledag#45](https://github.com/ipfs/go-merkledag/pull/45))
  - fix: return ErrLinkNotFound when the _link_ isn't found ([ipfs/go-merkledag#44](https://github.com/ipfs/go-merkledag/pull/44))
  - fix: include root in searches by default ([ipfs/go-merkledag#43](https://github.com/ipfs/go-merkledag/pull/43))
  - rework the graph walking functions with functional options ([ipfs/go-merkledag#42](https://github.com/ipfs/go-merkledag/pull/42))
  - fix inconsistent EnumerateChildrenAsync behavior ([ipfs/go-merkledag#41](https://github.com/ipfs/go-merkledag/pull/41))
- github.com/ipfs/go-mfs (v0.0.7 -> v0.1.1):
  - migrate to go-libp2p-core ([ipfs/go-mfs#77](https://github.com/ipfs/go-mfs/pull/77))
- github.com/ipfs/go-peertaskqueue (v0.0.5-f09820a0a5b6 -> v0.2.0):
  - Extend peer task queue to work with want-have / want-block ([ipfs/go-peertaskqueue#8](https://github.com/ipfs/go-peertaskqueue/pull/8))
  - migrate to go-libp2p-core ([ipfs/go-peertaskqueue#4](https://github.com/ipfs/go-peertaskqueue/pull/4))
- github.com/ipfs/go-unixfs (v0.0.6 -> v0.2.4):
  - fix: fix a panic when deleting ([ipfs/go-unixfs#81](https://github.com/ipfs/go-unixfs/pull/81))
  - fix(dagreader): remove a buggy workaround for a gateway issue ([ipfs/go-unixfs#80](https://github.com/ipfs/go-unixfs/pull/80))
  - fix: correctly handle symlink file sizes ([ipfs/go-unixfs#78](https://github.com/ipfs/go-unixfs/pull/78))
  - fix: return the correct error from RemoveChild ([ipfs/go-unixfs#76](https://github.com/ipfs/go-unixfs/pull/76))
  - update the the last go-merkledag ([ipfs/go-unixfs#75](https://github.com/ipfs/go-unixfs/pull/75))
  - fix: enumerate children ([ipfs/go-unixfs#74](https://github.com/ipfs/go-unixfs/pull/74))
- github.com/ipfs/interface-go-ipfs-core (v0.0.8 -> v0.2.7):
  - Add pin ls tests for indirect pin traversal and pin type precedence ([ipfs/interface-go-ipfs-core#47](https://github.com/ipfs/interface-go-ipfs-core/pull/47))
  - fix(test): fix a flaky pubsub test ([ipfs/interface-go-ipfs-core#45](https://github.com/ipfs/interface-go-ipfs-core/pull/45))
  - README: stub ([ipfs/interface-go-ipfs-core#44](https://github.com/ipfs/interface-go-ipfs-core/pull/44))
  - test: test ReadAt if implemented ([ipfs/interface-go-ipfs-core#43](https://github.com/ipfs/interface-go-ipfs-core/pull/43))
  - test: fix put with hash test ([ipfs/interface-go-ipfs-core#41](https://github.com/ipfs/interface-go-ipfs-core/pull/41))
  - Bump go-libp2p-core, up test key size to 2048 ([ipfs/interface-go-ipfs-core#39](https://github.com/ipfs/interface-go-ipfs-core/pull/39))
  - migrate to go-libp2p-core. ([ipfs/interface-go-ipfs-core#35](https://github.com/ipfs/interface-go-ipfs-core/pull/35))
  - tests: expose TestSuite ([ipfs/interface-go-ipfs-core#34](https://github.com/ipfs/interface-go-ipfs-core/pull/34))
- github.com/libp2p/go-libp2p (v0.0.32 -> v0.8.2):
  - fix: keep observed addrs alive as long as their associated connections are alive ([libp2p/go-libp2p#899](https://github.com/libp2p/go-libp2p/pull/899))
  - fix: refactor logic for identifying connections ([libp2p/go-libp2p#898](https://github.com/libp2p/go-libp2p/pull/898))
  - fix: reduce log level of a noisy log line ([libp2p/go-libp2p#889](https://github.com/libp2p/go-libp2p/pull/889))
  - [discovery] missing defer .Stop on ticker ([libp2p/go-libp2p#888](https://github.com/libp2p/go-libp2p/pull/888))
  - deprioritize unspecified addresses in mock connections ([libp2p/go-libp2p#887](https://github.com/libp2p/go-libp2p/pull/887))
  - feat: support TLS by default ([libp2p/go-libp2p#884](https://github.com/libp2p/go-libp2p/pull/884))
  - Expose option for setting autonat throttling ([libp2p/go-libp2p#882](https://github.com/libp2p/go-libp2p/pull/882))
  - Clearer naming of nat override options ([libp2p/go-libp2p#878](https://github.com/libp2p/go-libp2p/pull/878))
  - fix: set the private key when constructing the autonat service ([libp2p/go-libp2p#853](https://github.com/libp2p/go-libp2p/pull/853))
  - Signal address change ([libp2p/go-libp2p#851](https://github.com/libp2p/go-libp2p/pull/851))
  - fix multiple issues in the mock tests ([libp2p/go-libp2p#850](https://github.com/libp2p/go-libp2p/pull/850))
  - fix: minimal autonat dialer ([libp2p/go-libp2p#849](https://github.com/libp2p/go-libp2p/pull/849))
  - Trigger Autorelay on NAT events ([libp2p/go-libp2p#807](https://github.com/libp2p/go-libp2p/pull/807))
  - Local addr updated event ([libp2p/go-libp2p#847](https://github.com/libp2p/go-libp2p/pull/847))
  - feat(mock): reliable notifications ([libp2p/go-libp2p#836](https://github.com/libp2p/go-libp2p/pull/836))
  - doc(options): fix autorelay documentation ([libp2p/go-libp2p#835](https://github.com/libp2p/go-libp2p/pull/835))
  - change PrivateNetwork to accept a PSK, update constructor magic ([libp2p/go-libp2p#796](https://github.com/libp2p/go-libp2p/pull/796))
  - docs: Update the README ([libp2p/go-libp2p#827](https://github.com/libp2p/go-libp2p/pull/827))
  - fix: remove an unnecessary goroutine ([libp2p/go-libp2p#820](https://github.com/libp2p/go-libp2p/pull/820))
  - EnableAutoRelay should work without ContentRouting if there are StaticRelays defined ([libp2p/go-libp2p#810](https://github.com/libp2p/go-libp2p/pull/810))
  - Use of mux.ErrReset in mocknet ([libp2p/go-libp2p#815](https://github.com/libp2p/go-libp2p/pull/815))
  - docs: uniform comment sentences ([libp2p/go-libp2p#826](https://github.com/libp2p/go-libp2p/pull/826))
  - enable non-public address port mapping announcement ([libp2p/go-libp2p#771](https://github.com/libp2p/go-libp2p/pull/771))
  - fix: demote stream deadline errors to debug logs ([libp2p/go-libp2p#768](https://github.com/libp2p/go-libp2p/pull/768))
  - small grammar fixes and updates to readme ([libp2p/go-libp2p#743](https://github.com/libp2p/go-libp2p/pull/743))
  - Identify: Make activation threshold configurable ([libp2p/go-libp2p#740](https://github.com/libp2p/go-libp2p/pull/740))
  - better user-agent handling ([libp2p/go-libp2p#702](https://github.com/libp2p/go-libp2p/pull/702))
  - Update deps, mocknet tests ([libp2p/go-libp2p#697](https://github.com/libp2p/go-libp2p/pull/697))
  - autorelay: ensure candidate relays can hop ([libp2p/go-libp2p#696](https://github.com/libp2p/go-libp2p/pull/696))
  - We don't use `cs` here, drop it. ([libp2p/go-libp2p#682](https://github.com/libp2p/go-libp2p/pull/682))
  - Fix racy and failing test cases. ([libp2p/go-libp2p#674](https://github.com/libp2p/go-libp2p/pull/674))
  - fix: use the goprocess for closing ([libp2p/go-libp2p#669](https://github.com/libp2p/go-libp2p/pull/669))
  - update package table after -core refactor ([libp2p/go-libp2p#661](https://github.com/libp2p/go-libp2p/pull/661))
  - basic_host: ensure we close correctly when the context is canceled ([libp2p/go-libp2p#656](https://github.com/libp2p/go-libp2p/pull/656))
  - Add go-libp2p-gostream and go-libp2p-http to readme ([libp2p/go-libp2p#655](https://github.com/libp2p/go-libp2p/pull/655))
- github.com/libp2p/go-libp2p-autonat (v0.0.6 -> v0.2.2):
  - Run Autonat Service while in unknown connectivity mode ([libp2p/go-libp2p-autonat#75](https://github.com/libp2p/go-libp2p-autonat/pull/75))
  - Add option to force nat into a specified reachability state ([libp2p/go-libp2p-autonat#55](https://github.com/libp2p/go-libp2p-autonat/pull/55))
  - Merge Autonat-svc ([libp2p/go-libp2p-autonat#54](https://github.com/libp2p/go-libp2p-autonat/pull/54))
  - change autonat interface to use functional options ([libp2p/go-libp2p-autonat#53](https://github.com/libp2p/go-libp2p-autonat/pull/53))
  - Limiting autonat service responses/startup ([libp2p/go-libp2p-autonat#45](https://github.com/libp2p/go-libp2p-autonat/pull/45))
  - Emit events when NAT status changes ([libp2p/go-libp2p-autonat#37](https://github.com/libp2p/go-libp2p-autonat/pull/37))
  - Take eventbus events to completion ([libp2p/go-libp2p-autonat#38](https://github.com/libp2p/go-libp2p-autonat/pull/38))
  - Add missing syntax to autonat.proto ([libp2p/go-libp2p-autonat#26](https://github.com/libp2p/go-libp2p-autonat/pull/26))
  - full close the autonat stream ([libp2p/go-libp2p-autonat#20](https://github.com/libp2p/go-libp2p-autonat/pull/20))
  - reduce dialback timeout to 15s ([libp2p/go-libp2p-autonat#17](https://github.com/libp2p/go-libp2p-autonat/pull/17))
  - Extract service implementation from go-libp2p-autonat ([libp2p/go-libp2p-autonat#1](https://github.com/libp2p/go-libp2p-autonat/pull/1))
- github.com/libp2p/go-libp2p-circuit (v0.0.9 -> v0.2.2):
  - fix: don't abort accept when accepting a single connection fails ([libp2p/go-libp2p-circuit#107](https://github.com/libp2p/go-libp2p-circuit/pull/107))
  - Revert "feat: functional options" ([libp2p/go-libp2p-circuit#103](https://github.com/libp2p/go-libp2p-circuit/pull/103))
  - feat: remove relay discovery and unspecified relay dialing ([libp2p/go-libp2p-circuit#101](https://github.com/libp2p/go-libp2p-circuit/pull/101))
  - move protocol definitions to go-multiaddr ([libp2p/go-libp2p-circuit#81](https://github.com/libp2p/go-libp2p-circuit/pull/81))
  - return the full address from conn.RemoteMultiaddr ([libp2p/go-libp2p-circuit#80](https://github.com/libp2p/go-libp2p-circuit/pull/80))
  - expose CanHop as a module function ([libp2p/go-libp2p-circuit#79](https://github.com/libp2p/go-libp2p-circuit/pull/79))
- github.com/libp2p/go-libp2p-discovery (v0.0.5 -> v0.4.0):
  - Fix race with reuse of randomness ([libp2p/go-libp2p-discovery#54](https://github.com/libp2p/go-libp2p-discovery/pull/54))
  - Add Backoff Cache Discovery ([libp2p/go-libp2p-discovery#26](https://github.com/libp2p/go-libp2p-discovery/pull/26))
  - Discovery based Content Routing ([libp2p/go-libp2p-discovery#27](https://github.com/libp2p/go-libp2p-discovery/pull/27))
- github.com/libp2p/go-libp2p-kad-dht (v0.0.15 -> v0.7.10):
  - fix: avoid blocking when bootstrapping ([libp2p/go-libp2p-kad-dht#610](https://github.com/libp2p/go-libp2p-kad-dht/pull/610))
  - fix: re-validate peers whenever their state changes ([libp2p/go-libp2p-kad-dht#607](https://github.com/libp2p/go-libp2p-kad-dht/pull/607))
  - intercept failing query events when finding providers ([libp2p/go-libp2p-kad-dht#603](https://github.com/libp2p/go-libp2p-kad-dht/pull/603))
  - feat: set provider manager options ([libp2p/go-libp2p-kad-dht#593](https://github.com/libp2p/go-libp2p-kad-dht/pull/593))
  - fix: optimize debug logging a bit ([libp2p/go-libp2p-kad-dht#598](https://github.com/libp2p/go-libp2p-kad-dht/pull/598))
  - stricter definition of public for DHT ([libp2p/go-libp2p-kad-dht#596](https://github.com/libp2p/go-libp2p-kad-dht/pull/596))
  - feat: reduce allocations ([libp2p/go-libp2p-kad-dht#588](https://github.com/libp2p/go-libp2p-kad-dht/pull/588))
  - query.go: Remove shuffle comment ([libp2p/go-libp2p-kad-dht#586](https://github.com/libp2p/go-libp2p-kad-dht/pull/586))
  - fix: optimize isRelay ([libp2p/go-libp2p-kad-dht#585](https://github.com/libp2p/go-libp2p-kad-dht/pull/585))
  - feat: expose WANActive ([libp2p/go-libp2p-kad-dht#580](https://github.com/libp2p/go-libp2p-kad-dht/pull/580))
  - fix: improve error handling in dual dht ([libp2p/go-libp2p-kad-dht#582](https://github.com/libp2p/go-libp2p-kad-dht/pull/582))
  - fix: dedup addresses ([libp2p/go-libp2p-kad-dht#581](https://github.com/libp2p/go-libp2p-kad-dht/pull/581))
  - Fix bug in periodic peer pinging ([libp2p/go-libp2p-kad-dht#579](https://github.com/libp2p/go-libp2p-kad-dht/pull/579))
  - Dual DHT scaffold ([libp2p/go-libp2p-kad-dht#570](https://github.com/libp2p/go-libp2p-kad-dht/pull/570))
  - fix: linting fixes ([libp2p/go-libp2p-kad-dht#578](https://github.com/libp2p/go-libp2p-kad-dht/pull/578))
  - fix: remove local provider check ([libp2p/go-libp2p-kad-dht#577](https://github.com/libp2p/go-libp2p-kad-dht/pull/577))
  - fix: use the routing table filter ([libp2p/go-libp2p-kad-dht#576](https://github.com/libp2p/go-libp2p-kad-dht/pull/576))
  - fix: handle empty keys ([libp2p/go-libp2p-kad-dht#562](https://github.com/libp2p/go-libp2p-kad-dht/pull/562))
  - Set record handlers for the default protocol prefix ([libp2p/go-libp2p-kad-dht#560](https://github.com/libp2p/go-libp2p-kad-dht/pull/560))
  - fix incorrect error handling during provider record lookups ([libp2p/go-libp2p-kad-dht#554](https://github.com/libp2p/go-libp2p-kad-dht/pull/554))
  - Proposed DHTv2 Changes ([libp2p/go-libp2p-kad-dht#473](https://github.com/libp2p/go-libp2p-kad-dht/pull/473))
  - fix: obey the context when sending messages to peers ([libp2p/go-libp2p-kad-dht#462](https://github.com/libp2p/go-libp2p-kad-dht/pull/462))
  - Close context correctly ([libp2p/go-libp2p-kad-dht#477](https://github.com/libp2p/go-libp2p-kad-dht/pull/477))
  - add benchmark for handleFindPeer ([libp2p/go-libp2p-kad-dht#475](https://github.com/libp2p/go-libp2p-kad-dht/pull/475))
  - give views names again ([libp2p/go-libp2p-kad-dht#474](https://github.com/libp2p/go-libp2p-kad-dht/pull/474))
  - metrics: record message/request event even in case of error ([libp2p/go-libp2p-kad-dht#464](https://github.com/libp2p/go-libp2p-kad-dht/pull/464))
  - fix(dialqueue): fix a timer leak ([libp2p/go-libp2p-kad-dht#466](https://github.com/libp2p/go-libp2p-kad-dht/pull/466))
  - fix(query): cancel the context when the query finishes ([libp2p/go-libp2p-kad-dht#467](https://github.com/libp2p/go-libp2p-kad-dht/pull/467))
  - fix(providers): upgrade warnings to errors ([libp2p/go-libp2p-kad-dht#455](https://github.com/libp2p/go-libp2p-kad-dht/pull/455))
  - Make the Routing Table's latency tolerance configurable. ([libp2p/go-libp2p-kad-dht#454](https://github.com/libp2p/go-libp2p-kad-dht/pull/454))
  - Adjust cluster level while encoding as well ([libp2p/go-libp2p-kad-dht#445](https://github.com/libp2p/go-libp2p-kad-dht/pull/445))
  - Remove incorrect doc ([libp2p/go-libp2p-kad-dht#443](https://github.com/libp2p/go-libp2p-kad-dht/pull/443))
  - feat: reduce stream idle timeout to 1m ([libp2p/go-libp2p-kad-dht#441](https://github.com/libp2p/go-libp2p-kad-dht/pull/441))
  - Provider records use multihashes instead of CIDs ([libp2p/go-libp2p-kad-dht#422](https://github.com/libp2p/go-libp2p-kad-dht/pull/422))
  - Fix flaky TestEmptyTableTest ([libp2p/go-libp2p-kad-dht#433](https://github.com/libp2p/go-libp2p-kad-dht/pull/433))
  - Refresh cpl's in dht ([libp2p/go-libp2p-kad-dht#428](https://github.com/libp2p/go-libp2p-kad-dht/pull/428))
  - fix: always send the result channel when triggering a refresh ([libp2p/go-libp2p-kad-dht#425](https://github.com/libp2p/go-libp2p-kad-dht/pull/425))
  - feat: allow disabling value and provider storage/messages ([libp2p/go-libp2p-kad-dht#400](https://github.com/libp2p/go-libp2p-kad-dht/pull/400))
  - fix: prioritize closer peers ([libp2p/go-libp2p-kad-dht#424](https://github.com/libp2p/go-libp2p-kad-dht/pull/424))
  - fix: try to re-add existing peers when the routing table is empty ([libp2p/go-libp2p-kad-dht#420](https://github.com/libp2p/go-libp2p-kad-dht/pull/420))
  - feat: refresh and wait ([libp2p/go-libp2p-kad-dht#418](https://github.com/libp2p/go-libp2p-kad-dht/pull/418))
  - Make max record age configurable ([libp2p/go-libp2p-kad-dht#410](https://github.com/libp2p/go-libp2p-kad-dht/pull/410))
  - fix and simplify some bootstrapping logic ([libp2p/go-libp2p-kad-dht#405](https://github.com/libp2p/go-libp2p-kad-dht/pull/405))
  - feat(bootstrap): take autobootstrap to completion ([libp2p/go-libp2p-kad-dht#403](https://github.com/libp2p/go-libp2p-kad-dht/pull/403))
  - Feature/correct bootstrapping ([libp2p/go-libp2p-kad-dht#384](https://github.com/libp2p/go-libp2p-kad-dht/pull/384))
  - Update tests to use Ed25519 when acceptable. ([libp2p/go-libp2p-kad-dht#380](https://github.com/libp2p/go-libp2p-kad-dht/pull/380))
  - Add timeout ([libp2p/go-libp2p-kad-dht#351](https://github.com/libp2p/go-libp2p-kad-dht/pull/351))
  - Feat/message size ([libp2p/go-libp2p-kad-dht#353](https://github.com/libp2p/go-libp2p-kad-dht/pull/353))
  - reduce background goroutines ([libp2p/go-libp2p-kad-dht#340](https://github.com/libp2p/go-libp2p-kad-dht/pull/340))
- github.com/libp2p/go-libp2p-kbucket (v0.1.1 -> v0.4.1):
  - fix: use time.Duration for time, not floats ([libp2p/go-libp2p-kbucket#76](https://github.com/libp2p/go-libp2p-kbucket/pull/76))
  - Add LastUsefulAt and LastSuccessfulQueryAt for each peer ([libp2p/go-libp2p-kbucket#75](https://github.com/libp2p/go-libp2p-kbucket/pull/75))
  - fix: correctly track CPLs of never refreshed buckets ([libp2p/go-libp2p-kbucket#71](https://github.com/libp2p/go-libp2p-kbucket/pull/71))
  - Get Peer Infos ([libp2p/go-libp2p-kbucket#69](https://github.com/libp2p/go-libp2p-kbucket/pull/69))
  - fix: use accurate bucket logic ([libp2p/go-libp2p-kbucket#64](https://github.com/libp2p/go-libp2p-kbucket/pull/64))
  - Replace dead peers & increase replacement cache size ([libp2p/go-libp2p-kbucket#59](https://github.com/libp2p/go-libp2p-kbucket/pull/59))
  - Kbucket refactoring for Content Routing ([libp2p/go-libp2p-kbucket#54](https://github.com/libp2p/go-libp2p-kbucket/pull/54))
  - Disassociate RT membership from connectivity ([libp2p/go-libp2p-kbucket#50](https://github.com/libp2p/go-libp2p-kbucket/pull/50))
  - Unit Test for the util.Closer function ([libp2p/go-libp2p-kbucket#48](https://github.com/libp2p/go-libp2p-kbucket/pull/48))
  - Refresh Cpl's, not buckets ([libp2p/go-libp2p-kbucket#46](https://github.com/libp2p/go-libp2p-kbucket/pull/46))
  - Fix NearestPeers Doc ([libp2p/go-libp2p-kbucket#45](https://github.com/libp2p/go-libp2p-kbucket/pull/45))
  - fix: when the target bucket is empty or low, pull from all other buckets ([libp2p/go-libp2p-kbucket#43](https://github.com/libp2p/go-libp2p-kbucket/pull/43))
  - readme: replace IPFS contrib links with libp2p ([libp2p/go-libp2p-kbucket#34](https://github.com/libp2p/go-libp2p-kbucket/pull/34))
  - k-bucket support for peoper kad bootstrapping ([libp2p/go-libp2p-kbucket#38](https://github.com/libp2p/go-libp2p-kbucket/pull/38))
  - Fix bootstrapping id generation logic ([libp2p/go-libp2p-kbucket#1](https://github.com/libp2p/go-libp2p-kbucket/pull/1))
  - fix: avoid hashing under a lock ([libp2p/go-libp2p-kbucket#31](https://github.com/libp2p/go-libp2p-kbucket/pull/31))
  - dep: use a faster sha256 library ([libp2p/go-libp2p-kbucket#32](https://github.com/libp2p/go-libp2p-kbucket/pull/32))
  - Remove a lot of allocations, and fix some ambiguous naming ([libp2p/go-libp2p-kbucket#30](https://github.com/libp2p/go-libp2p-kbucket/pull/30))
- github.com/libp2p/go-libp2p-mplex (v0.1.1 -> v0.2.3):
  - Respect mux.ErrReset ([libp2p/go-libp2p-mplex#9](https://github.com/libp2p/go-libp2p-mplex/pull/9))
- github.com/libp2p/go-libp2p-nat (v0.0.4 -> v0.0.6):
  - typo and changed deprecated method ([libp2p/go-libp2p-nat#26](https://github.com/libp2p/go-libp2p-nat/pull/26))
  - nit: fix log format ([libp2p/go-libp2p-nat#19](https://github.com/libp2p/go-libp2p-nat/pull/19))
  - fix: remove notifier ([libp2p/go-libp2p-nat#18](https://github.com/libp2p/go-libp2p-nat/pull/18))
- github.com/libp2p/go-libp2p-peerstore (v0.0.6 -> v0.2.3):
  - fix: handle nil peer IDs ([libp2p/go-libp2p-peerstore#88](https://github.com/libp2p/go-libp2p-peerstore/pull/88))
  - Fix memory store signed peer record bug ([libp2p/go-libp2p-peerstore#133](https://github.com/libp2p/go-libp2p-peerstore/pull/133))
  - fix: make closing the in-memory peerstore actually close it ([libp2p/go-libp2p-peerstore#131](https://github.com/libp2p/go-libp2p-peerstore/pull/131))
  - Correct path to peer.AddrInfo in deprecation ([libp2p/go-libp2p-peerstore#124](https://github.com/libp2p/go-libp2p-peerstore/pull/124))
  - fix multiple TTL bugs ([libp2p/go-libp2p-peerstore#92](https://github.com/libp2p/go-libp2p-peerstore/pull/92))
  - reduce allocations when adding addrs ([libp2p/go-libp2p-peerstore#86](https://github.com/libp2p/go-libp2p-peerstore/pull/86))
  - test: add metadata test ([libp2p/go-libp2p-peerstore#82](https://github.com/libp2p/go-libp2p-peerstore/pull/82))
  - set map in constructor ([libp2p/go-libp2p-peerstore#81](https://github.com/libp2p/go-libp2p-peerstore/pull/81))
  - improve interning ([libp2p/go-libp2p-peerstore#79](https://github.com/libp2p/go-libp2p-peerstore/pull/79))
- github.com/libp2p/go-libp2p-pnet (v0.0.1 -> v0.2.0):
  - remove key serialization, construct conn from ipnet.PSK ([libp2p/go-libp2p-pnet#32](https://github.com/libp2p/go-libp2p-pnet/pull/32))
  - remove dependency on go-multicodec ([libp2p/go-libp2p-pnet#26](https://github.com/libp2p/go-libp2p-pnet/pull/26))
- github.com/libp2p/go-libp2p-pubsub (v0.0.3 -> v0.2.7):
  - Replace LRU cache blacklist implementation with a time cache ([libp2p/go-libp2p-pubsub#258](https://github.com/libp2p/go-libp2p-pubsub/pull/258))
  - Configurable size of validate queue ([libp2p/go-libp2p-pubsub#255](https://github.com/libp2p/go-libp2p-pubsub/pull/255))
  - Rename VaidatorData to ValidatorData ([libp2p/go-libp2p-pubsub#251](https://github.com/libp2p/go-libp2p-pubsub/pull/251))
  - Configurable message id function ([libp2p/go-libp2p-pubsub#248](https://github.com/libp2p/go-libp2p-pubsub/pull/248))
  - tracing support ([libp2p/go-libp2p-pubsub#227](https://github.com/libp2p/go-libp2p-pubsub/pull/227))
  - add ValidatorData field to Message ([libp2p/go-libp2p-pubsub#231](https://github.com/libp2p/go-libp2p-pubsub/pull/231))
  - Configurable outbound peer queue sizes ([libp2p/go-libp2p-pubsub#230](https://github.com/libp2p/go-libp2p-pubsub/pull/230))
  - Topic handler bug fixes ([libp2p/go-libp2p-pubsub#225](https://github.com/libp2p/go-libp2p-pubsub/pull/225))
  - Add Discovery ([libp2p/go-libp2p-pubsub#184](https://github.com/libp2p/go-libp2p-pubsub/pull/184))
  - Expose the peer that propagates a message to the recipient ([libp2p/go-libp2p-pubsub#218](https://github.com/libp2p/go-libp2p-pubsub/pull/218))
  - gossip methods: renames and predicate adjustment ([libp2p/go-libp2p-pubsub#204](https://github.com/libp2p/go-libp2p-pubsub/pull/204))
  - godocs: clarify config params of MessageCache. ([libp2p/go-libp2p-pubsub#205](https://github.com/libp2p/go-libp2p-pubsub/pull/205))
  - minor bug fix: on join, source peers from gossip[topic] if insufficient peers in fanout[topic] ([libp2p/go-libp2p-pubsub#196](https://github.com/libp2p/go-libp2p-pubsub/pull/196))
  - add PubSub's context to Subscription ([libp2p/go-libp2p-pubsub#201](https://github.com/libp2p/go-libp2p-pubsub/pull/201))
  - Add the ability to handle newly subscribed peers ([libp2p/go-libp2p-pubsub#190](https://github.com/libp2p/go-libp2p-pubsub/pull/190))
  - Fix gossipsub race condition for heartbeat ([libp2p/go-libp2p-pubsub#188](https://github.com/libp2p/go-libp2p-pubsub/pull/188))
- github.com/libp2p/go-libp2p-pubsub-router (v0.0.3 -> v0.2.1):
  - fix: ignore bad peers when fetching the latest value ([libp2p/go-libp2p-pubsub-router#54](https://github.com/libp2p/go-libp2p-pubsub-router/pull/54))
  - fix: rename MinimalPubsub -> Pubsub interface and improve docs ([libp2p/go-libp2p-pubsub-router#52](https://github.com/libp2p/go-libp2p-pubsub-router/pull/52))
  - Use Minimal PubSub Interface Instead Of Full PubSub Router ([libp2p/go-libp2p-pubsub-router#51](https://github.com/libp2p/go-libp2p-pubsub-router/pull/51))
  - Remove bootstrapping code ([libp2p/go-libp2p-pubsub-router#37](https://github.com/libp2p/go-libp2p-pubsub-router/pull/37))
  - readme: replace IPFS contrib links with libp2p ([libp2p/go-libp2p-pubsub-router#34](https://github.com/libp2p/go-libp2p-pubsub-router/pull/34))
  - Add Persistence Layer on top of PubSub ([libp2p/go-libp2p-pubsub-router#33](https://github.com/libp2p/go-libp2p-pubsub-router/pull/33))
  - Subscribe to PubSub topic before Publishing ([libp2p/go-libp2p-pubsub-router#30](https://github.com/libp2p/go-libp2p-pubsub-router/pull/30))
  - PutValue not blocked by Provide during bootstrapping ([libp2p/go-libp2p-pubsub-router#29](https://github.com/libp2p/go-libp2p-pubsub-router/pull/29))
- github.com/libp2p/go-libp2p-quic-transport (v0.0.3 -> v0.3.5):
  - add command line client and server ([libp2p/go-libp2p-quic-transport#139](https://github.com/libp2p/go-libp2p-quic-transport/pull/139))
  - write qlogs to a temporary file first, then rename them when done ([libp2p/go-libp2p-quic-transport#136](https://github.com/libp2p/go-libp2p-quic-transport/pull/136))
  - export qlogs when the QLOGDIR env variable is set ([libp2p/go-libp2p-quic-transport#129](https://github.com/libp2p/go-libp2p-quic-transport/pull/129))
  - fix: avoid dialing/listening on dns addresses ([libp2p/go-libp2p-quic-transport#131](https://github.com/libp2p/go-libp2p-quic-transport/pull/131))
  - use a stateless reset key derived from the private key ([libp2p/go-libp2p-quic-transport#122](https://github.com/libp2p/go-libp2p-quic-transport/pull/122))
  - add support for multiaddr filtering ([libp2p/go-libp2p-quic-transport#125](https://github.com/libp2p/go-libp2p-quic-transport/pull/125))
  - use the resolved address for RemoteMultiaddr() ([libp2p/go-libp2p-quic-transport#127](https://github.com/libp2p/go-libp2p-quic-transport/pull/127))
  - accept a PSK in the transport constructor (and reject it) ([libp2p/go-libp2p-quic-transport#111](https://github.com/libp2p/go-libp2p-quic-transport/pull/111))
  - update quic-go to v0.15.0 ([libp2p/go-libp2p-quic-transport#114](https://github.com/libp2p/go-libp2p-quic-transport/pull/114))
  - increase the stream and connection receive windows ([libp2p/go-libp2p-quic-transport#108](https://github.com/libp2p/go-libp2p-quic-transport/pull/108))
  - fix key comparisons in tests ([libp2p/go-libp2p-quic-transport#110](https://github.com/libp2p/go-libp2p-quic-transport/pull/110))
  - make reuse work on Windows ([libp2p/go-libp2p-quic-transport#83](https://github.com/libp2p/go-libp2p-quic-transport/pull/83))
  - add a LICENSE ([libp2p/go-libp2p-quic-transport#78](https://github.com/libp2p/go-libp2p-quic-transport/pull/78))
  - Use specific netlink families for android ([libp2p/go-libp2p-quic-transport#75](https://github.com/libp2p/go-libp2p-quic-transport/pull/75))
  - implement a garbage-collector for unused reuse connections ([libp2p/go-libp2p-quic-transport#73](https://github.com/libp2p/go-libp2p-quic-transport/pull/73))
  - implement connection reuse ([libp2p/go-libp2p-quic-transport#63](https://github.com/libp2p/go-libp2p-quic-transport/pull/63))
  - update the README ([libp2p/go-libp2p-quic-transport#69](https://github.com/libp2p/go-libp2p-quic-transport/pull/69))
  - use the handshake logic from go-libp2p-tls ([libp2p/go-libp2p-quic-transport#67](https://github.com/libp2p/go-libp2p-quic-transport/pull/67))
  - update quic-go to v0.12.0 (supporting QUIC draft-22) ([libp2p/go-libp2p-quic-transport#68](https://github.com/libp2p/go-libp2p-quic-transport/pull/68))
  - when ListenUDP fails once, try again next time ([libp2p/go-libp2p-quic-transport#59](https://github.com/libp2p/go-libp2p-quic-transport/pull/59))
- github.com/libp2p/go-libp2p-record (v0.0.1 -> v0.1.2):
  - readme: replace IPFS contrib links with libp2p ([libp2p/go-libp2p-record#25](https://github.com/libp2p/go-libp2p-record/pull/25))
  - Use peer ID utilities to go from pubkey to peer ID ([libp2p/go-libp2p-record#26](https://github.com/libp2p/go-libp2p-record/pull/26))
- github.com/libp2p/go-libp2p-routing-helpers (v0.0.2 -> v0.2.2):
  - doc: document all types ([libp2p/go-libp2p-routing-helpers#40](https://github.com/libp2p/go-libp2p-routing-helpers/pull/40))
  - fix: fetch all providers when count is 0 ([libp2p/go-libp2p-routing-helpers#39](https://github.com/libp2p/go-libp2p-routing-helpers/pull/39))
  - feat: implement io.Closer ([libp2p/go-libp2p-routing-helpers#37](https://github.com/libp2p/go-libp2p-routing-helpers/pull/37))
  - readme: replace IPFS contrib links with libp2p ([libp2p/go-libp2p-routing-helpers#21](https://github.com/libp2p/go-libp2p-routing-helpers/pull/21))
- github.com/libp2p/go-libp2p-secio (v0.0.3 -> v0.2.2):
  - feat: remove sha1 hmac ([libp2p/go-libp2p-secio#64](https://github.com/libp2p/go-libp2p-secio/pull/64))
  - readme: add context and links ([libp2p/go-libp2p-secio#55](https://github.com/libp2p/go-libp2p-secio/pull/55))
  - Update to latest go-libp2p-core, update tests ([libp2p/go-libp2p-secio#54](https://github.com/libp2p/go-libp2p-secio/pull/54))
  - Remove support for blowfish ([libp2p/go-libp2p-secio#52](https://github.com/libp2p/go-libp2p-secio/pull/52))
  - fix: wait for handshake to complete before returning ([libp2p/go-libp2p-secio#50](https://github.com/libp2p/go-libp2p-secio/pull/50))
  - avoid holding the message writer longer than necessary ([libp2p/go-libp2p-secio#49](https://github.com/libp2p/go-libp2p-secio/pull/49))
- github.com/libp2p/go-libp2p-swarm (v0.0.7 -> v0.2.3):
  - don't expire backoffs until 2x backoff period ([libp2p/go-libp2p-swarm#193](https://github.com/libp2p/go-libp2p-swarm/pull/193))
  - fix: slightly simplify backoff logic ([libp2p/go-libp2p-swarm#192](https://github.com/libp2p/go-libp2p-swarm/pull/192))
  - change backoffs to per-address ([libp2p/go-libp2p-swarm#191](https://github.com/libp2p/go-libp2p-swarm/pull/191))
  - fix: set teardown after storing the context ([libp2p/go-libp2p-swarm#190](https://github.com/libp2p/go-libp2p-swarm/pull/190))
  - feat: handle no addresses ([libp2p/go-libp2p-swarm#185](https://github.com/libp2p/go-libp2p-swarm/pull/185))
  - fix: make sure to include peer in dial error ([libp2p/go-libp2p-swarm#180](https://github.com/libp2p/go-libp2p-swarm/pull/180))
  - Don't drop connections when simultaneous dialing occurs ([libp2p/go-libp2p-swarm#174](https://github.com/libp2p/go-libp2p-swarm/pull/174))
  - fix: fire a listen close event when closing the listener ([libp2p/go-libp2p-swarm#164](https://github.com/libp2p/go-libp2p-swarm/pull/164))
  - Link to godocs for Host instead of deprecated repo ([libp2p/go-libp2p-swarm#137](https://github.com/libp2p/go-libp2p-swarm/pull/137))
  - improve dial errors ([libp2p/go-libp2p-swarm#145](https://github.com/libp2p/go-libp2p-swarm/pull/145))
  - Minor Docstring correction ([libp2p/go-libp2p-swarm#143](https://github.com/libp2p/go-libp2p-swarm/pull/143))
  - test: close peerstore when closing the test swarm ([libp2p/go-libp2p-swarm#139](https://github.com/libp2p/go-libp2p-swarm/pull/139))
  - fix listen addrs race ([libp2p/go-libp2p-swarm#136](https://github.com/libp2p/go-libp2p-swarm/pull/136))
  - logging: make the swarm less noisy ([libp2p/go-libp2p-swarm#131](https://github.com/libp2p/go-libp2p-swarm/pull/131))
  - feat: cache interface addresses for 1 minute ([libp2p/go-libp2p-swarm#129](https://github.com/libp2p/go-libp2p-swarm/pull/129))
- github.com/libp2p/go-libp2p-tls (v0.0.2 -> v0.1.3):
  - Readme: link to the libp2p-core docs ([libp2p/go-libp2p-tls#36](https://github.com/libp2p/go-libp2p-tls/pull/36))
  - expose the function to derive the peer's public key from the cert chain ([libp2p/go-libp2p-tls#33](https://github.com/libp2p/go-libp2p-tls/pull/33))
  - set an ALPN value in the tls.Config ([libp2p/go-libp2p-tls#32](https://github.com/libp2p/go-libp2p-tls/pull/32))
- github.com/libp2p/go-libp2p-transport-upgrader (v0.0.4 -> v0.2.0):
  - use the ipnet.PSK instead of the ipnet.Protector for private networks ([libp2p/go-libp2p-transport-upgrader#45](https://github.com/libp2p/go-libp2p-transport-upgrader/pull/45))
  - readme: add context & fix example code ([libp2p/go-libp2p-transport-upgrader#26](https://github.com/libp2p/go-libp2p-transport-upgrader/pull/26))
  - fix an incorrect error message ([libp2p/go-libp2p-transport-upgrader#27](https://github.com/libp2p/go-libp2p-transport-upgrader/pull/27))
  - Consolidate abstractions and core types into go-libp2p-core (#28) ([libp2p/go-libp2p-transport-upgrader#22](https://github.com/libp2p/go-libp2p-transport-upgrader/pull/22))
- github.com/libp2p/go-libp2p-yamux (v0.1.3 -> v0.2.7):
  - Respect mux.ErrReset ([libp2p/go-libp2p-yamux#10](https://github.com/libp2p/go-libp2p-yamux/pull/10))
- github.com/libp2p/go-maddr-filter (v0.0.4 -> v0.0.5):
  - fix: check for blocked addrs without allocating ([libp2p/go-maddr-filter#14](https://github.com/libp2p/go-maddr-filter/pull/14))
- github.com/libp2p/go-mplex (v0.0.4 -> v0.1.2):
  - remove deprecated log.Warning(f) ([libp2p/go-mplex#65](https://github.com/libp2p/go-mplex/pull/65))
  - Remove dependency on go-libp2p-core and introduce new errors. ([libp2p/go-mplex#72](https://github.com/libp2p/go-mplex/pull/72))
  - Bump lodash from 4.17.5 to 4.17.15 in /interop/js ([libp2p/go-mplex#66](https://github.com/libp2p/go-mplex/pull/66))
  - add test for deadlines ([libp2p/go-mplex#60](https://github.com/libp2p/go-mplex/pull/60))
- github.com/libp2p/go-msgio (v0.0.2 -> v0.0.4):
  - make the maximum message size configurable ([libp2p/go-msgio#15](https://github.com/libp2p/go-msgio/pull/15))
  - combine writes and avoid a few more allocations ([libp2p/go-msgio#14](https://github.com/libp2p/go-msgio/pull/14))
  - avoid allocating unless we need to ([libp2p/go-msgio#13](https://github.com/libp2p/go-msgio/pull/13))
- github.com/libp2p/go-nat (v0.0.3 -> v0.0.5):
  - feat: switch to go-netroute ([libp2p/go-nat#19](https://github.com/libp2p/go-nat/pull/19))
  - fix: really obey the context ([libp2p/go-nat#13](https://github.com/libp2p/go-nat/pull/13))
  - don't mask context ([libp2p/go-nat#10](https://github.com/libp2p/go-nat/pull/10))
- github.com/libp2p/go-reuseport-transport (v0.0.2 -> v0.0.3):
  - fix: less confusing log message ([libp2p/go-reuseport-transport#22](https://github.com/libp2p/go-reuseport-transport/pull/22))
  - readme: replace IPFS contrib links with libp2p ([libp2p/go-reuseport-transport#16](https://github.com/libp2p/go-reuseport-transport/pull/16))
  - replace gx instructions with note about gomod ([libp2p/go-reuseport-transport#15](https://github.com/libp2p/go-reuseport-transport/pull/15))
- github.com/libp2p/go-tcp-transport (v0.0.4 -> v0.2.0):
  - fix: don't allow dialing DNS addresses ([libp2p/go-tcp-transport#61](https://github.com/libp2p/go-tcp-transport/pull/61))
  - Use new constructor for insecure transport in tests ([libp2p/go-tcp-transport#42](https://github.com/libp2p/go-tcp-transport/pull/42))
  - readme: add install, usage & addressing info ([libp2p/go-tcp-transport#41](https://github.com/libp2p/go-tcp-transport/pull/41))
- github.com/libp2p/go-ws-transport (v0.0.6 -> v0.3.1):
  - fix: add read/write locks ([libp2p/go-ws-transport#85](https://github.com/libp2p/go-ws-transport/pull/85))
  - fix: restrict dials to IP + TCP ([libp2p/go-ws-transport#84](https://github.com/libp2p/go-ws-transport/pull/84))
  - Revert "add mutex for write/close" ([libp2p/go-ws-transport#73](https://github.com/libp2p/go-ws-transport/pull/73))
  - feat: faster copy in wasm ([libp2p/go-ws-transport#68](https://github.com/libp2p/go-ws-transport/pull/68))
  - Add WebAssembly support and the ability to Dial from browsers ([libp2p/go-ws-transport#55](https://github.com/libp2p/go-ws-transport/pull/55))
  - fix: close gracefully ([libp2p/go-ws-transport#54](https://github.com/libp2p/go-ws-transport/pull/54))
  - move multiaddr protocol definitions to go-multiaddr ([libp2p/go-ws-transport#52](https://github.com/libp2p/go-ws-transport/pull/52))
  - Add install, usage & addressing info to README ([libp2p/go-ws-transport#49](https://github.com/libp2p/go-ws-transport/pull/49))
- github.com/libp2p/go-yamux (v1.2.3 -> v1.3.5):
  - fix: synchronize when resetting the keepalive timer ([libp2p/go-yamux#21](https://github.com/libp2p/go-yamux/pull/21))
  - fix: don't keepalive when the connection is busy ([libp2p/go-yamux#16](https://github.com/libp2p/go-yamux/pull/16))
  - Rename errors ([libp2p/go-yamux#14](https://github.com/libp2p/go-yamux/pull/14))
  - fix(stream): set writeDeadline when cleanup and forceClose ([libp2p/go-yamux#12](https://github.com/libp2p/go-yamux/pull/12))
  - fixes a stream deadlock multiple ways ([libp2p/go-yamux#8](https://github.com/libp2p/go-yamux/pull/8))

### Contributors

| Contributor                | Commits | Lines ±       | Files Changed |
|----------------------------|---------|---------------|---------------|
| Steven Allen               | 858     | +27833/-15919 | 1906          |
| Dirk McCormick             | 134     | +18058/-8347  | 282           |
| Aarsh Shah                 | 83      | +13458/-11883 | 241           |
| Adin Schmahmann            | 144     | +11878/-6236  | 397           |
| Raúl Kripalani             | 94      | +6894/-10214  | 598           |
| vyzo                       | 60      | +8923/-1160   | 102           |
| Will Scott                 | 79      | +3776/-1467   | 175           |
| Michael Muré               | 29      | +1734/-3290   | 104           |
| dependabot[bot]            | 365     | +3419/-361    | 728           |
| Hector Sanjuan             | 64      | +2053/-1321   | 132           |
| Marten Seemann             | 52      | +1922/-1268   | 147           |
| Michael Avila              | 29      | +828/-1733    | 70            |
| Peter Rabbitson            | 53      | +1073/-1197   | 100           |
| Yusef Napora               | 36      | +1610/-378    | 57            |
| hannahhoward               | 16      | +1342/-559    | 61            |
| Łukasz Magiera             | 9       | +277/-1623    | 41            |
| Marcin Rataj               | 9       | +1686/-99     | 32            |
| Will                       | 7       | +936/-709     | 34            |
| Alex Browne                | 27      | +1019/-503    | 46            |
| David Dias                 | 30      | +987/-431     | 43            |
| Jakub Sztandera            | 43      | +912/-436     | 77            |
| Cole Brown                 | 21      | +646/-398     | 57            |
| Oli Evans                  | 29      | +488/-466     | 43            |
| Cornelius Toole            | 3       | +827/-60      | 20            |
| Hlib                       | 15      | +331/-185     | 28            |
| Adrian Lanzafame           | 9       | +123/-334     | 18            |
| Petar Maymounkov           | 1       | +385/-48      | 5             |
| Alan Shaw                  | 18      | +262/-146     | 35            |
| lnykww                     | 1       | +303/-52      | 6             |
| Hannah Howard              | 1       | +198/-27      | 3             |
| Dominic Della Valle        | 9       | +163/-52      | 14            |
| Adam Uhlir                 | 1       | +211/-2       | 3             |
| Dimitris Apostolou         | 1       | +105/-105     | 64            |
| Frrist                     | 1       | +186/-18      | 5             |
| Henrique Dias              | 22      | +119/-28      | 22            |
| Gergely Tabiczky           | 5       | +74/-60       | 7             |
| Matt Joiner                | 2       | +63/-62       | 4             |
| @RubenKelevra              | 12      | +46/-55       | 12            |
| whyrusleeping              | 6       | +87/-11       | 7             |
| deepakgarg                 | 4       | +42/-43       | 4             |
| protolambda                | 2       | +49/-17       | 9             |
| hucg                       | 2       | +47/-11       | 3             |
| Arber Avdullahu            | 3       | +31/-27       | 3             |
| Sameer Puri                | 1       | +46/-4        | 2             |
| Hucg                       | 3       | +17/-33       | 3             |
| Guilhem Fanton             | 2       | +29/-10       | 7             |
| Christian Muehlhaeuser     | 6       | +20/-19       | 14            |
| Djalil Dreamski            | 3       | +27/-9        | 3             |
| Caian                      | 2       | +36/-0        | 2             |
| Topper Bowers              | 2       | +31/-4        | 4             |
| flowed                     | 1       | +16/-16       | 11            |
| Vibhav Pant                | 4       | +21/-10       | 5             |
| frrist                     | 1       | +26/-4        | 1             |
| Hlib Kanunnikov            | 1       | +25/-3        | 1             |
| george xie                 | 3       | +12/-15       | 11            |
| optman                     | 1       | +13/-9        | 1             |
| Roman Proskuryakov         | 1       | +11/-11       | 2             |
| Vasco Santos               | 1       | +10/-10       | 5             |
| Pretty Please Mark Darkly  | 2       | +16/-2        | 2             |
| Piotr Dyraga               | 2       | +15/-2        | 2             |
| Andrew Nesbitt             | 1       | +5/-11        | 5             |
| postables                  | 4       | +19/-8        | 4             |
| Jim McDonald               | 2       | +13/-1        | 2             |
| PoorPockets McNewHold      | 1       | +12/-0        | 1             |
| Henri S                    | 1       | +6/-6         | 1             |
| Igor Velkov                | 1       | +8/-3         | 1             |
| swedneck                   | 4       | +7/-3         | 4             |
| Devin                      | 2       | +5/-5         | 4             |
| iulianpascalau             | 1       | +5/-3         | 2             |
| MollyM                     | 3       | +7/-1         | 3             |
| Jorropo                    | 2       | +5/-3         | 3             |
| lukesolo                   | 1       | +6/-1         | 2             |
| Wes Morgan                 | 1       | +3/-3         | 1             |
| Kishan Mohanbhai Sagathiya | 1       | +3/-3         | 2             |
| songjiayang                | 1       | +4/-0         | 1             |
| Terry Ding                 | 1       | +2/-2         | 1             |
| Preston Van Loon           | 2       | +3/-1         | 2             |
| Jim Pick                   | 2       | +2/-2         | 2             |
| Jakub Kaczmarzyk           | 1       | +2/-2         | 1             |
| Simon Menke                | 2       | +2/-1         | 2             |
| Jessica Schilling          | 2       | +1/-2         | 2             |
| Edgar Aroutiounian         | 1       | +2/-1         | 1             |
| hikerpig                   | 1       | +1/-1         | 1             |
| ZenGround0                 | 1       | +1/-1         | 1             |
| Thomas Preindl             | 1       | +1/-1         | 1             |
| Sander Pick                | 1       | +1/-1         | 1             |
| Ronsor                     | 1       | +1/-1         | 1             |
| Roman Khafizianov          | 1       | +1/-1         | 1             |
| Rod Vagg                   | 1       | +1/-1         | 1             |
| Max Inden                  | 1       | +1/-1         | 1             |
| Leo Arias                  | 1       | +1/-1         | 1             |
| Kuro1                      | 1       | +1/-1         | 1             |
| Kirill Goncharov           | 1       | +1/-1         | 1             |
| John B Nelson              | 1       | +1/-1         | 1             |
| George Masgras             | 1       | +1/-1         | 1             |
| Aliabbas Merchant          | 1       | +1/-1         | 1             |
| Lorenzo Setale             | 1       | +1/-0         | 1             |
| Boris Mann                 | 1       | +1/-0         | 1             |

## 0.4.23 2020-01-29

Given the large number of fixes merged since 0.4.22, we've decided to cut another patch release.

This release contains critical fixes. Please upgrade ASAP. Importantly, we're strongly considering switching to TLS by default in go-ipfs 0.5.0 and dropping SECIO support. However, the current TLS transport in go-ipfs 0.4.22 has a bug that can cause connections to spontaneously disconnect during the handshake.

This release fixes that bug, among many other issues. Users that _don't_ upgrade may experience connectivity issues when the network upgrades to go-ipfs 0.5.0.

### Highlights

* Fixes build on go 1.13
* Fixes an issue where we may not connect to providers in bitswap.
* Fixes an issue on the TLS transport where we may abort a handshake unintentionally.
* Fixes a common panic in the websocket transport.
* Adds support for recursively resolving dnsaddrs (makes go-ipfs compatible with the new bootstrappers).
* Fixes several potential panics/crashes.
* Switches to using pre-defined autorelays instead of trying to find them in the DHT:
  * Avoids selecting random, potentially poor, relays.
  * Avoids spamming the DHT with requests trying to find relays.
  * Reduces the impact of accidentally enabling AutoRelay + RelayHop. I.e., the network won't try to DoS you.
* Modifies the connection manager to not count connections in the grace period towards the connection limit.
  * Pro: New connections don't cause us to close useful, existing connections.
  * Con: Libp2p will keep more connections. Consider reducing your HighWater after applying this patch.
* Improved peer usefulness tracking in bitswap. Frequently used peers will be marked as "important" and the connection manager will avoid closing connections to these peers.
* Includes a new version of the WebUI to fix some issues with the peers map.

### Changelog

- github.com/ipfs/go-ipfs:
  - feat: update the webui to fix some performance issues ([ipfs/go-ipfs#6844](https://github.com/ipfs/go-ipfs/pull/6844))
  - fix: limit SW registration to content root ([ipfs/go-ipfs#6801](https://github.com/ipfs/go-ipfs/pull/6801))
  - fix issue 6760, adding with hash-only, high CPU usage. ([ipfs/go-ipfs#6764](https://github.com/ipfs/go-ipfs/pull/6764))
  - fix(coreapi/add): close the fake repo used when adding with hash-only ([ipfs/go-ipfs#6747](https://github.com/ipfs/go-ipfs/pull/6747))
  - fix bug 6748 ([ipfs/go-ipfs#6754](https://github.com/ipfs/go-ipfs/pull/6754))
  - fix(pin): wait till after fetching to remove direct pin ([ipfs/go-ipfs#6708](https://github.com/ipfs/go-ipfs/pull/6708))
  - pin: fix pin update X Y where X==Y ([ipfs/go-ipfs#6669](https://github.com/ipfs/go-ipfs/pull/6669))
  - namesys: set the correct cache TTL on publish ([ipfs/go-ipfs#6667](https://github.com/ipfs/go-ipfs/pull/6667))
  - build: fix golangci again ([ipfs/go-ipfs#6641](https://github.com/ipfs/go-ipfs/pull/6641))
  - make: move all test deps to a separate module ([ipfs/go-ipfs#6637](https://github.com/ipfs/go-ipfs/pull/6637))
  - fix: close peerstore on stop ([ipfs/go-ipfs#6629](https://github.com/ipfs/go-ipfs/pull/6629))
  - build: fix build when we don't have a full git tree ([ipfs/go-ipfs#6626](https://github.com/ipfs/go-ipfs/pull/6626))
- github.com/ipfs/go-bitswap (v0.0.8-cbb485998356 -> v0.0.8-e37498cf10d6):
  - fix: wait until we finish connecting before we cancel the context ([ipfs/go-bitswap#226](https://github.com/ipfs/go-bitswap/pull/226))
  - engine: tag peers based on usefulness ([ipfs/go-bitswap#191](https://github.com/ipfs/go-bitswap/pull/191))
- github.com/ipfs/go-cid (v0.0.2 -> v0.0.4):
  - fix parsing issues and nits ([ipfs/go-cid#97](https://github.com/ipfs/go-cid/pull/97))
  - Verify that prefix is correct v0 prefix ([ipfs/go-cid#96](https://github.com/ipfs/go-cid/pull/96))
- github.com/multiformats/go-multihash (v0.0.5 -> v0.0.10):
  - Ensure that length of multihash is properly handled ([multiformats/go-multihash#119](https://github.com/multiformats/go-multihash/pull/119))
  - fix murmur3 name  ([multiformats/go-multihash#115](https://github.com/multiformats/go-multihash/pull/115))
  - rename ID to IDENTITY ([multiformats/go-multihash#113](https://github.com/multiformats/go-multihash/pull/113))
 ([multiformats/go-multihash#119](https://github.com/multiformats/go-multihash/pull/119))
- github.com/libp2p/go-flow-metrics (v0.0.1 -> v0.0.3):
  - fix bug in meter traversal logic ([libp2p/go-flow-metrics#11](https://github.com/libp2p/go-flow-metrics/pull/11))
- github.com/libp2p/go-libp2p (v0.0.28 -> v0.0.32):
  - options to configure known relays for autorelay ([libp2p/go-libp2p#705](https://github.com/libp2p/go-libp2p/pull/705))
  - feat(host): recursively resolve addresses ([libp2p/go-libp2p#764](https://github.com/libp2p/go-libp2p/pull/764))
  - mdns: always use interface addresses ([libp2p/go-libp2p#667](https://github.com/libp2p/go-libp2p/pull/667))
- github.com/libp2p/go-libp2p-connmgr (v0.0.6 -> v0.2.1):
  - don't count connections in the grace period against the limit ([libp2p/go-libp2p-connmgr#50](https://github.com/libp2p/go-libp2p-connmgr/pull/50))
- github.com/libp2p/go-libp2p-kad-dht (v0.0.13 -> v0.0.15):
  - metrics: fix memory leak ([libp2p/go-libp2p-kad-dht#390](https://github.com/libp2p/go-libp2p-kad-dht/pull/390))
- github.com/libp2p/go-libp2p-tls (v0.0.1 -> v0.0.2):
  - close the underlying connection when the handshake fails ([libp2p/go-libp2p-tls#39](https://github.com/libp2p/go-libp2p-tls/pull/39))
  - make the error check for not receiving a public key more explicit ([libp2p/go-libp2p-tls#34](https://github.com/libp2p/go-libp2p-tls/pull/34))
  - Fix: Connection Closed after handshake ([libp2p/go-libp2p-tls#37](https://github.com/libp2p/go-libp2p-tls/pull/37))
- github.com/libp2p/go-libp2p-swarm (v0.0.6 -> v0.0.7):
  - fix: don't assume that transports implement stringer ([libp2p/go-libp2p-swarm#134](https://github.com/libp2p/go-libp2p-swarm/pull/134))
- github.com/libp2p/go-ws-transport (v0.0.4 -> v0.0.6):
  - Add mutex for write/close ([libp2p/go-ws-transport#65](https://github.com/libp2p/go-ws-transport/pull/65))

Other:

Update bloom filter libraries to remove unsound usage of the `unsafe` package.

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Steven Allen | 52 | +1866/-578 | 102 |
| vyzo | 12 | +167/-90 | 22 |
| whyrusleeping | 5 | +136/-52 | 7 |
| Roman Proskuryakov | 7 | +94/-7 | 10 |
| Jakub Sztandera | 3 | +58/-13 | 7 |
| hcg1314 | 2 | +31/-11 | 2 |
| Raúl Kripalani | 2 | +7/-33 | 6 |
| Marten Seemann | 3 | +27/-10 | 5 |
| Marcin Rataj | 2 | +26/-0 | 5 |
| b5 | 1 | +2/-22 | 1 |
| Hector Sanjuan | 1 | +11/-0 | 1 |
| Yusef Napora | 1 | +4/-0 | 1 |

## 0.4.22 2019-08-06

We're releasing a PATCH release of go-ipfs based on 0.4.21 containing some critical fixes.

The IPFS network has scaled to the point where small changes can have a
wide-reaching impact on the entire network. To keep this situation from
escalating, we've put a hold on releasing new features until we can improve our
[release process](https://github.com/ipfs/go-ipfs/blob/master/docs/releases.md)
(which we've trialed in this release) and [testing
procedures](https://github.com/ipfs/go-ipfs/issues/6483).

This release includes fixes for the following regressions:

1. A major bitswap throughput regression introduced in 0.4.21
   ([ipfs/go-ipfs#6442](https://github.com/ipfs/go-ipfs/issues/6442)).
2. High bitswap CPU usage when connected to many (e.g. 10,000) peers. See
   [ipfs/go-bitswap#154](https://github.com/ipfs/go-bitswap/issues/154).
2. The local network discovery service sometimes initializes before the
   networking module, causing it to announce the wrong addresses and sometimes
   complain about not being able to determine the IP address
   ([ipfs/go-ipfs#6415](https://github.com/ipfs/go-ipfs/pull/6415)).
   
It also includes fixes for:

1. Pins not being persisted after `ipfs block add --pin`
   ([ipfs/go-ipfs#6441](https://github.com/ipfs/go-ipfs/pull/6441)).
2. Panic due to concurrent map access when adding and listing pins at the same
   time ([ipfs/go-ipfs#6419](https://github.com/ipfs/go-ipfs/pull/6419)).
3. Potential pin-set corruption given a concurrent `ipfs repo gc` and `ipfs pin
   rm` ([ipfs/go-ipfs#6444](https://github.com/ipfs/go-ipfs/pull/6444)).
4. Build failure due to a deleted git tag in one of our dependencies
   ([ipfs/go-ds-badger#64](https://github.com/ipfs/go-ds-badger/pull/65)).

Thanks to:

* [@hannahhoward](https://github.com/hannahhoward) for fixing both bitswap issues.
* [@sanderpick](https://github.com/sanderpick) for catching and fixing the local
  discovery bug.
* [@campoy](https://github.com/campoy) for fixing the build issue.

## 0.4.21 2019-05-30

We're happy to announce go-ipfs 0.4.21. This release has some critical bug fixes
and a handful of new features so every user should upgrade.

Key bug fixes:

* Too many open file descriptors/too many peers
  ([#6237](https://github.com/ipfs/go-ipfs/issues/6237)).
* Adding multiple files at the same time doesn't work
  ([#6254](https://github.com/ipfs/go-ipfs/pull/6255)).
* CPU utilization spikes and then holds at 100%
  ([#5613](https://github.com/ipfs/go-ipfs/issues/5613)).

Key features:

* Experimental TLS1.3 support (to eventually replace secio).
* OpenSSL support for SECIO handshakes (performance improvement).

**IMPORTANT:** This release fixes a bug in our security transport that could
potentially drop data from the channel. Note: This issue affects neither the
privacy nor the integrity of the data with respect to a third-party attacker.
Only the peer sending us data could trigger this bug.

**ALL USERS MUST UPGRADE.** We intended to introduce a feature this release that,
unfortunately, [reliably triggered this bug][secio-bug]. To avoid partitioning
the network, we've decided to postpone this feature for a release or two.

Specifically, we're going to provide a minimum _one month_ upgrade period. After
that, we'll start testing the impact of deploying the proposed changes.

If you're running the mainline go-ipfs, please upgrade ASAP. If you're building
a separate app or working on a forked go-ipfs, make sure to upgrade
github.com/libp2p/go-libp2p-secio to _at least_ v0.0.3.

[secio-bug]: https://github.com/libp2p/go-libp2p/issues/644

### Contributors

First off, we'd like to give a shout-out to all contributors that participated
in this release (including contributions to ipld, libp2p, and multiformats):

| Contributor                | Commits | Lines ±     | Files Changed |
|----------------------------|---------|-------------|---------------|
| Steven Allen               | 220     | +6078/-4211 | 520           |
| Łukasz Magiera             | 53      | +5039/-4557 | 274           |
| vyzo                       | 179     | +2929/-1704 | 238           |
| Raúl Kripalani             | 44      | +757/-1895  | 134           |
| hannahhoward               | 11      | +755/-1005  | 49            |
| Marten Seemann             | 16      | +862/-203   | 44            |
| keks                       | 10      | +359/-110   | 12            |
| Jan Winkelmann             | 8       | +368/-26    | 16            |
| Jakub Sztandera            | 4       | +361/-8     | 7             |
| Adrian Lanzafame           | 1       | +287/-18    | 5             |
| Erik Ingenito              | 4       | +247/-28    | 8             |
| Reid 'arrdem' McKenzie     | 1       | +220/-20    | 3             |
| Yusef Napora               | 26      | +98/-130    | 26            |
| Michael Avila              | 3       | +116/-59    | 8             |
| Raghav Gulati              | 13      | +145/-26    | 13            |
| tg                         | 1       | +41/-33     | 1             |
| Matt Joiner                | 6       | +41/-30     | 7             |
| Cole Brown                 | 1       | +37/-25     | 1             |
| Dominic Della Valle        | 2       | +12/-40     | 4             |
| Overbool                   | 1       | +50/-0      | 2             |
| Christopher Buesser        | 3       | +29/-16     | 10            |
| myself659                  | 1       | +38/-5      | 2             |
| Alex Browne                | 3       | +30/-8      | 3             |
| jmank88                    | 1       | +27/-4      | 2             |
| Vikram                     | 1       | +25/-1      | 2             |
| MollyM                     | 7       | +17/-9      | 7             |
| Marcin Rataj               | 1       | +17/-1      | 1             |
| requilence                 | 1       | +11/-4      | 1             |
| Teran McKinney             | 1       | +8/-2       | 1             |
| Oli Evans                  | 1       | +5/-5       | 1             |
| Masashi Salvador Mitsuzawa | 1       | +5/-1       | 1             |
| chenminjian                | 1       | +4/-0       | 1             |
| Edgar Lee                  | 1       | +3/-1       | 1             |
| Dirk McCormick             | 1       | +2/-2       | 2             |
| ia                         | 1       | +1/-1       | 1             |
| Alan Shaw                  | 1       | +1/-1       | 1             |

### Bug Fixes And Enhancements

This release includes quite a number of critical bug fixes and
performance/reliability enhancements.

#### Error when adding multiple files

The last release broke the simple command `ipfs add file1 file2`. It turns out
we simply lacked a test case for this. Both of these issues (the bug and the
lack of a test case) have now been fixed.

#### SECIO

As noted above, we've fixed a bug that could cause data to be dropped from a
SECIO connection on read. Specifically, this happens when:

1. The capacity of the read buffer is greater than the length.
2. The remote peer sent more than the length but less than the capacity in a
   single secio "frame".

In this case, we'd fill the read buffer to it's capacity instead of its length.

#### Too many open files, too many peers, etc.

Go-ipfs automatically closes the least useful connections when it accumulates
too many connections. Unfortunately, some relayed connections were blocking in
`Close()`, halting the entire process.

#### Out of control CPU usage

Many users noted out of control CPU usage this release. This turned out to be a
long-standing issue with how the DHT handled provider records (records recording
which peers have what content):

1. It wasn't removing provider records for content until the set of providers
   completely emptied.
2. It was loading every provider record into memory whenever we updated the set
   of providers.

Combined, these two issues were trashing the provider record cache, forcing the
DHT to repeatedly load and discard provider records.

#### More Reliable Connection Management

Go-ipfs has a subsystem called the "connection manager" to close the
least-useful connections when go-ipfs runs low on resources.

Unfortunately, other IPFS subsystems may learn about connections _before_ the
connection manager. Previously, if some IPFS subsystem tried to mark a
connection as useful before the connection manager learned about it, the
connection manager would discard this information. We believe this was causing
[#6271](https://github.com/ipfs/go-ipfs/issues/6271). [It no longer does
that](https://github.com/libp2p/go-libp2p-connmgr/pull/39).

#### Improved Bitswap Connection Management

Bitswap now uses the connection manager to mark all peers downloading blocks as
important (while downloading). Previously, it only marked peers from which _it_
was downloading blocks.

#### Reduced Memory Usage

The most noticeable memory reduction in this release comes from fixing connection
closing. However, we've made a few additional improvements:

* Bitswap's "work queue" no longer remembers every peer it has seen
  indefinitely.
* The peerstore now interns protocol names.
* The per-peer goroutine count has been reduced.
* The DHT now wastes less memory on idle peers by pooling buffered writers and
  returning them to the pool when not actively using them.

#### Increased File Descriptor Limit

The default file descriptor limit has been raised to 8192 (from 2048).
Unfortunately, go-ipfs behaves poorly when it runs out of file descriptors and
it uses a _lot_ of file descriptors.

Luckily, most modern kernels can handle thousands of file descriptors without
any difficulty.

#### Decreased Connection Handshake Latency

Libp2p now shaves off a couple of round trips when initiating connections by
beginning the protocol negotiation before the remote peer responds to the
initial handshake message.

In the optimal case (when the target peer speaks our preferred protocol), this
reduces the number of handshake round-trips from 6 to 4 (including the TCP
handshake).

### Commands

This release brings no new commands but does introduce a few changes, bugfixes,
and enhancements. This section is hardly complete but it lists the most
noticeable changes.

Take note: this release also introduces a few breaking changes.

#### [DEPRECATION] The URLStore Command Deprecated

The experimental `ipfs urlstore` command is now deprecated. Please use `ipfs add
--nocopy URL` instead.

#### [BREAKING] The DHT Command Base64 Encodes Values

When responding to an `ipfs dht get` command, the daemon now encodes the
returned value using base64. The `ipfs` command will automatically decode this
value before returning it to the user so this change should only affect those
using the HTTP API directly.

Unfortunately, this change was necessary as DHT records are arbitrary binary
blobs which can't be directly stored in JSON strings.

#### [BREAKING] Base32 Encoded v1 CIDs By Default

Both js-ipfs and go-ipfs now encode CIDv1 CIDs using base32 by default, instead
of base58. Unfortunately, base58 is case-sensitive and doesn't play well with
browsers (see [#4143](https://github.com/ipfs/go-ipfs/issues/4143).

#### Human Readable Numbers

The `ipfs bitswap stat` and and `ipfs object stat` commands now support a
`--humanize` flag that formats numbers with human-readable units (GiB, MiB,
etc.).

#### Improved Errors

This release improves two types of errors:

1. Commands that take paths/multiaddrs now include the path/multiaddr in the
   error message when it fails to parse.
2. `ipfs swarm connect` now returns a detailed error describing which addresses
   were tried and why the dial failed.

#### Ping Improvements

The ping command has received some small improvements and fixes:

1. It now exits with a non-zero exit status on failure.
2. It no longer succeeds with zero successful pings if we have a zombie but
   non-functional connection to the peer being pinged
   ([#6298](https://github.com/ipfs/go-ipfs/issues/6298)).
3. It now prints out the average latency when canceled with `^C` (like the unix
   `ping` command).

#### Improved Help Text

Go-ipfs now intelligently wraps help text for easier reading. On an 80 character
wide terminal,

**Before**

```
USAGE
  ipfs add <path>... - Add a file or directory to ipfs.

SYNOPSIS
  ipfs add [--recursive | -r] [--dereference-args] [--stdin-name=<stdin-name>] [
--hidden | -H] [--quiet | -q] [--quieter | -Q] [--silent] [--progress | -p] [--t
rickle | -t] [--only-hash | -n] [--wrap-with-directory | -w] [--chunker=<chunker
> | -s] [--pin=false] [--raw-leaves] [--nocopy] [--fscache] [--cid-version=<cid-
version>] [--hash=<hash>] [--inline] [--inline-limit=<inline-limit>] [--] <path>
...

ARGUMENTS

  <path>... - The path to a file to be added to ipfs.

OPTIONS

  -r,               --recursive           bool   - Add directory paths recursive
ly.
  --dereference-args                      bool   - Symlinks supplied in argument
s are dereferenced.
  --stdin-name                            string - Assign a name if the file sou
rce is stdin.
  -H,               --hidden              bool   - Include files that are hidden
. Only takes effect on recursive add.
  -q,               --quiet               bool   - Write minimal output.
  -Q,               --quieter             bool   - Write only final hash.
  --silent                                bool   - Write no output.
  -p,               --progress            bool   - Stream progress data.
  -t,               --trickle             bool   - Use trickle-dag format for da
g generation.
  -n,               --only-hash           bool   - Only chunk and hash - do not 
write to disk.
  -w,               --wrap-with-directory bool   - Wrap files with a directory o
bject.
  -s,               --chunker             string - Chunking algorithm, size-[byt
es] or rabin-[min]-[avg]-[max]. Default: size-262144.
  --pin                                   bool   - Pin this object when adding. 
Default: true.
  --raw-leaves                            bool   - Use raw blocks for leaf nodes
. (experimental).
  --nocopy                                bool   - Add the file using filestore.
 Implies raw-leaves. (experimental).
  --fscache                               bool   - Check the filestore for pre-e
xisting blocks. (experimental).
  --cid-version                           int    - CID version. Defaults to 0 un
less an option that depends on CIDv1 is passed. (experimental).
  --hash                                  string - Hash function to use. Implies
 CIDv1 if not sha2-256. (experimental). Default: sha2-256.
  --inline                                bool   - Inline small blocks into CIDs
. (experimental).
  --inline-limit                          int    - Maximum block size to inline.
 (experimental). Default: 32.

```


**After**

```
USAGE
  ipfs add <path>... - Add a file or directory to ipfs.

SYNOPSIS
  ipfs add [--recursive | -r] [--dereference-args] [--stdin-name=<stdin-name>]
           [--hidden | -H] [--quiet | -q] [--quieter | -Q] [--silent]
           [--progress | -p] [--trickle | -t] [--only-hash | -n]
           [--wrap-with-directory | -w] [--chunker=<chunker> | -s] [--pin=false]
           [--raw-leaves] [--nocopy] [--fscache] [--cid-version=<cid-version>]
           [--hash=<hash>] [--inline] [--inline-limit=<inline-limit>] [--]
           <path>...

ARGUMENTS

  <path>... - The path to a file to be added to ipfs.

OPTIONS

  -r, --recursive            bool   - Add directory paths recursively.
  --dereference-args         bool   - Symlinks supplied in arguments are
                                      dereferenced.
  --stdin-name               string - Assign a name if the file source is stdin.
  -H, --hidden               bool   - Include files that are hidden. Only takes
                                      effect on recursive add.
  -q, --quiet                bool   - Write minimal output.
  -Q, --quieter              bool   - Write only final hash.
  --silent                   bool   - Write no output.
  -p, --progress             bool   - Stream progress data.
  -t, --trickle              bool   - Use trickle-dag format for dag generation.
  -n, --only-hash            bool   - Only chunk and hash - do not write to
                                      disk.
  -w, --wrap-with-directory  bool   - Wrap files with a directory object.
  -s, --chunker              string - Chunking algorithm, size-[bytes] or
                                      rabin-[min]-[avg]-[max]. Default:
                                      size-262144.
  --pin                      bool   - Pin this object when adding. Default:
                                      true.
  --raw-leaves               bool   - Use raw blocks for leaf nodes.
                                      (experimental).
  --nocopy                   bool   - Add the file using filestore. Implies
                                      raw-leaves. (experimental).
  --fscache                  bool   - Check the filestore for pre-existing
                                      blocks. (experimental).
  --cid-version              int    - CID version. Defaults to 0 unless an
                                      option that depends on CIDv1 is passed.
                                      (experimental).
  --hash                     string - Hash function to use. Implies CIDv1 if
                                      not sha2-256. (experimental). Default:
                                      sha2-256.
  --inline                   bool   - Inline small blocks into CIDs.
                                      (experimental).
  --inline-limit             int    - Maximum block size to inline.
                                      (experimental). Default: 32.
```

### Features

This release is primarily a bug fix release but it still includes two nice
features from libp2p.

#### Experimental TLS1.3 support

Go-ipfs now has experimental TLS1.3 support. Currently, libp2p (IPFS's
networking library) uses a custom TLS-like protocol we call SECIO. However, the
conventional wisdom concerning custom security transports is "just don't" so we
are working on replacing it with TLS1.3

To choose this protocol by default, set the `Experimental.PreferTLS` config
variable:

```bash
> ipfs config --bool Experimental.PreferTLS true
```

Why TLS1.3 and not X (noise, etc.)?

1. Libp2p allows negotiating transports so there's no reason not to add noise
   support to libp2p as well.
2. TLS has wide language support which should make implementing libp2p for new
   languages significantly simpler.

#### OpenSSL Support

Go-ipfs can now (optionally) be built with OpenSSL support for improved
performance when establishing connections. This is primarily useful for nodes
receiving multiple inbound connections per second.

To enable openssl support, rebuild go-ipfs with:

```bash
> make build GOTAGS=openssl
```

### CoreAPI

The CoreAPI refactor is still underway and we've made significant progress
towards a usable ipfs-as-a-library constructor. Specifically, we've integrated
the [fx](https://go.uber.org/fx) dependency injection system and are
now working on cleaning up our initialization logic. This should make it easier
to inject new services into a go-ipfs process without messing with the core
internals.

### Build: `GOCC` Environment Variable

Build system now uses `GOCC` environment variable allowing for use of specific
go versions during builds.

### Changelog

- github.com/ipfs/go-ipfs:
  - fix: use http.Error for sending errors ([ipfs/go-ipfs#6379](https://github.com/ipfs/go-ipfs/pull/6379))
  - core: call app.Stop once ([ipfs/go-ipfs#6380](https://github.com/ipfs/go-ipfs/pull/6380))
  - explain what dhtclient does ([ipfs/go-ipfs#6375](https://github.com/ipfs/go-ipfs/pull/6375))
  - ci: actually enable golangci-lint ([ipfs/go-ipfs#6362](https://github.com/ipfs/go-ipfs/pull/6362))
  - commands/swarm(fix): handle empty multiaddrs ([ipfs/go-ipfs#6355](https://github.com/ipfs/go-ipfs/pull/6355))
  - feat: improve errors when a path fails to parse ([ipfs/go-ipfs#6346](https://github.com/ipfs/go-ipfs/pull/6346))
  - fix vendoring dependencies when building the source tarball ([ipfs/go-ipfs#6349](https://github.com/ipfs/go-ipfs/pull/6349))
  - core: Use correct default for connmgr lowWater ([ipfs/go-ipfs#6352](https://github.com/ipfs/go-ipfs/pull/6352))
  - doc: remove out of date documentation ([ipfs/go-ipfs#6345](https://github.com/ipfs/go-ipfs/pull/6345))
  - Add generation of dependency changes to mkreleaselog ([ipfs/go-ipfs#6348](https://github.com/ipfs/go-ipfs/pull/6348))
  - readme: remove mention of DCO ([ipfs/go-ipfs#6344](https://github.com/ipfs/go-ipfs/pull/6344))
  - Add golangci-lint ([ipfs/go-ipfs#6321](https://github.com/ipfs/go-ipfs/pull/6321))
  - docs+mk: update guidance for unsupported platforms ([ipfs/go-ipfs#6338](https://github.com/ipfs/go-ipfs/pull/6338))
  - fix formatting in object get ([ipfs/go-ipfs#6340](https://github.com/ipfs/go-ipfs/pull/6340))
  - fail start when loading a plugin fails ([ipfs/go-ipfs#6339](https://github.com/ipfs/go-ipfs/pull/6339))
  - fix a typo in the issue template ([ipfs/go-ipfs#6335](https://github.com/ipfs/go-ipfs/pull/6335))
  - github: turn issue template into a multiple-choice question ([ipfs/go-ipfs#6333](https://github.com/ipfs/go-ipfs/pull/6333))
  - object put: Allow empty objects ([ipfs/go-ipfs#6330](https://github.com/ipfs/go-ipfs/pull/6330))
  - Update fuse.md ([ipfs/go-ipfs#6332](https://github.com/ipfs/go-ipfs/pull/6332))
  - work towards fixing dht commands ([ipfs/go-ipfs#6277](https://github.com/ipfs/go-ipfs/pull/6277))
  - fix setting ulimit ([ipfs/go-ipfs#6319](https://github.com/ipfs/go-ipfs/pull/6319))
  - switch to base32 by default for CIDv1 ([ipfs/go-ipfs#6300](https://github.com/ipfs/go-ipfs/pull/6300))
  - cmdkit -> cmds ([ipfs/go-ipfs#6318](https://github.com/ipfs/go-ipfs/pull/6318))
  - raise default fd limit to 8192 ([ipfs/go-ipfs#6266](https://github.com/ipfs/go-ipfs/pull/6266))
  - pin: don't walk all pinned blocks when removing a non-existent pin ([ipfs/go-ipfs#6311](https://github.com/ipfs/go-ipfs/pull/6311))
  - ping: fix a bunch of issues ([ipfs/go-ipfs#6312](https://github.com/ipfs/go-ipfs/pull/6312))
  - test(coreapi): use a thread-safe datastore everywhere ([ipfs/go-ipfs#6222](https://github.com/ipfs/go-ipfs/pull/6222))
  - fix(Dockerfile): Allow ipfs mount in Docker container ([ipfs/go-ipfs#5560](https://github.com/ipfs/go-ipfs/pull/5560))
  - docs: fix Routing section ([ipfs/go-ipfs#6309](https://github.com/ipfs/go-ipfs/pull/6309))
  - License update to dual MIT and Apache 2 ([ipfs/go-ipfs#6301](https://github.com/ipfs/go-ipfs/pull/6301))
  - Go test fix ([ipfs/go-ipfs#6293](https://github.com/ipfs/go-ipfs/pull/6293))
  - commands(pin update): return resolved CIDs instead of paths ([ipfs/go-ipfs#6275](https://github.com/ipfs/go-ipfs/pull/6275))
  - core: fix autonat construction ([ipfs/go-ipfs#6289](https://github.com/ipfs/go-ipfs/pull/6289))
  - Test and fix GC/pin bug ([ipfs/go-ipfs#6288](https://github.com/ipfs/go-ipfs/pull/6288))
  - GOCC implementation & fix in make & build scripts ([ipfs/go-ipfs#6282](https://github.com/ipfs/go-ipfs/pull/6282))
  - gc: cancel context ([ipfs/go-ipfs#6281](https://github.com/ipfs/go-ipfs/pull/6281))
  - fix: windows friendly daemon help ([ipfs/go-ipfs#6278](https://github.com/ipfs/go-ipfs/pull/6278))
  - Invert constructor config handling  ([ipfs/go-ipfs#6276](https://github.com/ipfs/go-ipfs/pull/6276))
  - docs: document environment variables ([ipfs/go-ipfs#6268](https://github.com/ipfs/go-ipfs/pull/6268))
  - add: Return error from iterator ([ipfs/go-ipfs#6272](https://github.com/ipfs/go-ipfs/pull/6272))
  - commands(feat): use the coreapi in the urlstore command ([ipfs/go-ipfs#6259](https://github.com/ipfs/go-ipfs/pull/6259))
  - humanize for ipfs bitswap stat ([ipfs/go-ipfs#6258](https://github.com/ipfs/go-ipfs/pull/6258))
  - Revert "raise default fd limit to 8192" ([ipfs/go-ipfs#6265](https://github.com/ipfs/go-ipfs/pull/6265))
  - raise default fd limit to 8192 ([ipfs/go-ipfs#6261](https://github.com/ipfs/go-ipfs/pull/6261))
  - Fix AutoNAT service for private network ([ipfs/go-ipfs#6251](https://github.com/ipfs/go-ipfs/pull/6251))
  - add: Fix adding multiple files ([ipfs/go-ipfs#6255](https://github.com/ipfs/go-ipfs/pull/6255))
  - reprovider: Use goprocess ([ipfs/go-ipfs#6248](https://github.com/ipfs/go-ipfs/pull/6248))
  - core/corehttp/gateway_handler: pass a request ctx instead of the node ([ipfs/go-ipfs#6244](https://github.com/ipfs/go-ipfs/pull/6244))
  - constructor: cleanup some things ([ipfs/go-ipfs#6246](https://github.com/ipfs/go-ipfs/pull/6246))
  - Support --human flag in cmd/object-stat ([ipfs/go-ipfs#6241](https://github.com/ipfs/go-ipfs/pull/6241))
  - build: fix macos build with fuse ([ipfs/go-ipfs#6235](https://github.com/ipfs/go-ipfs/pull/6235))
  - add an experiment to prefer TLS 1.3 over secio ([ipfs/go-ipfs#6229](https://github.com/ipfs/go-ipfs/pull/6229))
  - fix two small nits in the go-ipfs constructor ([ipfs/go-ipfs#6234](https://github.com/ipfs/go-ipfs/pull/6234))
  - DI-based core.NewNode ([ipfs/go-ipfs#6162](https://github.com/ipfs/go-ipfs/pull/6162))
  - coreapi: Drop error from ParsePath ([ipfs/go-ipfs#6122](https://github.com/ipfs/go-ipfs/pull/6122))
  - fix the wrong path configuration in root redirection ([ipfs/go-ipfs#6215](https://github.com/ipfs/go-ipfs/pull/6215))
- github.com/ipfs/go-bitswap (v0.0.4 -> v0.0.7):
  - feat(engine): tag peers with requests ([ipfs/go-bitswap#128](https://github.com/ipfs/go-bitswap/pull/128))
  - fix(network): add mutex to avoid data race ([ipfs/go-bitswap#127](https://github.com/ipfs/go-bitswap/pull/127))
  - Change bitswap provide toggle to not be static ([ipfs/go-bitswap#124](https://github.com/ipfs/go-bitswap/pull/124))
  - Use shared peer task queue with Graphsync ([ipfs/go-bitswap#119](https://github.com/ipfs/go-bitswap/pull/119))
  - Add missing godoc comments, refactor to avoid confusion ([ipfs/go-bitswap#117](https://github.com/ipfs/go-bitswap/pull/117))
  - fix(decision): cleanup request queues ([ipfs/go-bitswap#116](https://github.com/ipfs/go-bitswap/pull/116))
  - Control provider workers with experiment flag ([ipfs/go-bitswap#110](https://github.com/ipfs/go-bitswap/pull/110))
  - connmgr: give peers more weight when actively participating in a session ([ipfs/go-bitswap#111](https://github.com/ipfs/go-bitswap/pull/111))
  - make the WantlistManager own the PeerHandler ([ipfs/go-bitswap#78](https://github.com/ipfs/go-bitswap/pull/78))
  - remove IPFS_LOW_MEM flag support ([ipfs/go-bitswap#115](https://github.com/ipfs/go-bitswap/pull/115))
- github.com/ipfs/go-cid (v0.0.1 -> v0.0.2):
  - default cidv1 to base32 ([ipfs/go-cid#85](https://github.com/ipfs/go-cid/pull/85))
- github.com/ipfs/go-cidutil (v0.0.1 -> v0.0.2):
  - default cidv1 to base32 ([ipfs/go-cidutil#13](https://github.com/ipfs/go-cidutil/pull/13))
- github.com/ipfs/go-datastore (v0.0.3 -> v0.0.5):
  - MapDatastore: obey KeysOnly ([ipfs/go-datastore#130](https://github.com/ipfs/go-datastore/pull/130))
  - fix the keytransform datastore's query implementation ([ipfs/go-datastore#127](https://github.com/ipfs/go-datastore/pull/127))
  - sync: apply entire query while locked ([ipfs/go-datastore#129](https://github.com/ipfs/go-datastore/pull/129))
  - filter: values are now always bytes ([ipfs/go-datastore#126](https://github.com/ipfs/go-datastore/pull/126))
  - autobatch: batch deletes ([ipfs/go-datastore#128](https://github.com/ipfs/go-datastore/pull/128))
- github.com/ipfs/go-ipfs-cmds (v0.0.5 -> v0.0.8):
  - fix: use golang's http.Error to send errors ([ipfs/go-ipfs-cmds#167](https://github.com/ipfs/go-ipfs-cmds/pull/167))
  - improve help text on narrow terminals ([ipfs/go-ipfs-cmds#140](https://github.com/ipfs/go-ipfs-cmds/pull/140))
  - chore: remove an old hack ([ipfs/go-ipfs-cmds#165](https://github.com/ipfs/go-ipfs-cmds/pull/165))
  - http: use the request context ([ipfs/go-ipfs-cmds#163](https://github.com/ipfs/go-ipfs-cmds/pull/163))
  - merge in go-ipfs-cmdkit ([ipfs/go-ipfs-cmds#164](https://github.com/ipfs/go-ipfs-cmds/pull/164))
  - fix: return the correct error ([ipfs/go-ipfs-cmds#162](https://github.com/ipfs/go-ipfs-cmds/pull/162))
- github.com/ipfs/go-ipfs-config (v0.0.1 -> v0.0.3):
  - Closes: #6284 Add appropriate IPv6 ranges to defaultServerFilters ([ipfs/go-ipfs-config#34](https://github.com/ipfs/go-ipfs-config/pull/34))
  - add an experiment to prefer TLS 1.3 over secio ([ipfs/go-ipfs-config#32](https://github.com/ipfs/go-ipfs-config/pull/32))
- github.com/ipfs/go-ipfs-files (v0.0.2 -> v0.0.3):
  - webfile: make Size() work before Read ([ipfs/go-ipfs-files#18](https://github.com/ipfs/go-ipfs-files/pull/18))
  - check http status code during WebFile reads and return error for non-2XX ([ipfs/go-ipfs-files#17](https://github.com/ipfs/go-ipfs-files/pull/17))
- github.com/ipfs/go-ipld-cbor (v0.0.1 -> v0.0.2):
  - switch to base32 by default ([ipfs/go-ipld-cbor#62](https://github.com/ipfs/go-ipld-cbor/pull/62))
- github.com/ipfs/go-ipld-git (v0.0.1 -> v0.0.2):
  - switch to base32 by default ([ipfs/go-ipld-git#40](https://github.com/ipfs/go-ipld-git/pull/40))
- github.com/ipfs/go-mfs (v0.0.4 -> v0.0.7):
  - Fix directory mv and add tests ([ipfs/go-mfs#76](https://github.com/ipfs/go-mfs/pull/76))
  - fix: not remove file by mistakes ([ipfs/go-mfs#73](https://github.com/ipfs/go-mfs/pull/73))
- github.com/ipfs/go-path (v0.0.3 -> v0.0.4):
  - include the path in path errors ([ipfs/go-path#28](https://github.com/ipfs/go-path/pull/28))
- github.com/ipfs/go-unixfs (v0.0.4 -> v0.0.6):
  - chore: remove URL field ([ipfs/go-unixfs#72](https://github.com/ipfs/go-unixfs/pull/72))
- github.com/ipfs/interface-go-ipfs-core (v0.0.6 -> v0.0.8):
  - switch to base32 cidv1 by default ([ipfs/interface-go-ipfs-core#29](https://github.com/ipfs/interface-go-ipfs-core/pull/29))
  - path: drop error from ParsePath ([ipfs/interface-go-ipfs-core#22](https://github.com/ipfs/interface-go-ipfs-core/pull/22))
  - tests: fix a bunch of small test lints/issues ([ipfs/interface-go-ipfs-core#28](https://github.com/ipfs/interface-go-ipfs-core/pull/28))
  - Update Pin.RmRecursive docs to clarify shared indirect pins are not removed ([ipfs/interface-go-ipfs-core#26](https://github.com/ipfs/interface-go-ipfs-core/pull/26))
- github.com/libp2p/go-buffer-pool (v0.0.1 -> v0.0.2):
  - feat: add buffered writer ([libp2p/go-buffer-pool#9](https://github.com/libp2p/go-buffer-pool/pull/9))
- github.com/libp2p/go-conn-security-multistream (v0.0.1 -> v0.0.2):
  - block while writing ([libp2p/go-conn-security-multistream#10](https://github.com/libp2p/go-conn-security-multistream/pull/10))
- github.com/libp2p/go-libp2p (v0.0.12 -> v0.0.28):
  - Close the connection manager ([libp2p/go-libp2p#639](https://github.com/libp2p/go-libp2p/pull/639))
  - Frequent Relay Advertisements ([libp2p/go-libp2p#637](https://github.com/libp2p/go-libp2p/pull/637))
  - ping: return a stream of results ([libp2p/go-libp2p#626](https://github.com/libp2p/go-libp2p/pull/626))
  - Use cancelable background context in identify ([libp2p/go-libp2p#624](https://github.com/libp2p/go-libp2p/pull/624))
  - avoid intermediate allocation in relayAddrs ([libp2p/go-libp2p#609](https://github.com/libp2p/go-libp2p/pull/609))
  - cache relayAddrs for a short period of time ([libp2p/go-libp2p#608](https://github.com/libp2p/go-libp2p/pull/608))
  - autorelay: break findRelays into multiple functions and avoid the goto ([libp2p/go-libp2p#606](https://github.com/libp2p/go-libp2p/pull/606))
  - autorelay: curtail addrsplosion ([libp2p/go-libp2p#598](https://github.com/libp2p/go-libp2p/pull/598))
  - Periodically schedule identify push if the address set has changed ([libp2p/go-libp2p#597](https://github.com/libp2p/go-libp2p/pull/597))
  - Replace peer addresses in identify ([libp2p/go-libp2p#599](https://github.com/libp2p/go-libp2p/pull/599))
- github.com/libp2p/go-libp2p-circuit (v0.0.4 -> v0.0.8):
  - call Stream.Reset instead of Stream.Close ([libp2p/go-libp2p-circuit#76](https://github.com/libp2p/go-libp2p-circuit/pull/76))
  - Tag the hop relay when creating stop streams ([libp2p/go-libp2p-circuit#77](https://github.com/libp2p/go-libp2p-circuit/pull/77))
  - Tag peers with live hop streams ([libp2p/go-libp2p-circuit#75](https://github.com/libp2p/go-libp2p-circuit/pull/75))
  - Hard Limit the number of hop stream goroutines ([libp2p/go-libp2p-circuit#74](https://github.com/libp2p/go-libp2p-circuit/pull/74))
  - set deadline for stop handshake ([libp2p/go-libp2p-circuit#73](https://github.com/libp2p/go-libp2p-circuit/pull/73))
- github.com/libp2p/go-libp2p-connmgr (v0.0.1 -> v0.0.6):
  - Background trimming ([libp2p/go-libp2p-connmgr#43](https://github.com/libp2p/go-libp2p-connmgr/pull/43))
  - Implement UpsertTag ([libp2p/go-libp2p-connmgr#38](https://github.com/libp2p/go-libp2p-connmgr/pull/38))
  - Add peer protection capability (implementation) ([libp2p/go-libp2p-connmgr#36](https://github.com/libp2p/go-libp2p-connmgr/pull/36))
- github.com/libp2p/go-libp2p-crypto (v0.0.1 -> v0.0.2):
  - add openssl support ([libp2p/go-libp2p-crypto#61](https://github.com/libp2p/go-libp2p-crypto/pull/61))
- github.com/libp2p/go-libp2p-discovery (v0.0.1 -> v0.0.4):
  - More consistent use of options ([libp2p/go-libp2p-discovery#25](https://github.com/libp2p/go-libp2p-discovery/pull/25))
  - Use 3hrs as routing advertisement ttl ([libp2p/go-libp2p-discovery#23](https://github.com/libp2p/go-libp2p-discovery/pull/23))
- github.com/libp2p/go-libp2p-interface-connmgr (v0.0.1 -> v0.0.5):
  - Add Close method to the ConnManager interface ([libp2p/go-libp2p-interface-connmgr#18](https://github.com/libp2p/go-libp2p-interface-connmgr/pull/18))
  - Add UpsertTag to the interface ([libp2p/go-libp2p-interface-connmgr#17](https://github.com/libp2p/go-libp2p-interface-connmgr/pull/17))
  - Fix NullConnMgr to respect ConnManager interface ([libp2p/go-libp2p-interface-connmgr#15](https://github.com/libp2p/go-libp2p-interface-connmgr/pull/15))
  - Add peer protection capability ([libp2p/go-libp2p-interface-connmgr#14](https://github.com/libp2p/go-libp2p-interface-connmgr/pull/14))
- github.com/libp2p/go-libp2p-kad-dht (v0.0.7 -> v0.0.13):
  - fix: reduce memory used by buffered writers ([libp2p/go-libp2p-kad-dht#332](https://github.com/libp2p/go-libp2p-kad-dht/pull/332))
  - query: fix a goroutine leak when the routing table is empty ([libp2p/go-libp2p-kad-dht#329](https://github.com/libp2p/go-libp2p-kad-dht/pull/329))
  - query: fix error "leak" ([libp2p/go-libp2p-kad-dht#328](https://github.com/libp2p/go-libp2p-kad-dht/pull/328))
  - providers: run datastore GC concurrently ([libp2p/go-libp2p-kad-dht#326](https://github.com/libp2p/go-libp2p-kad-dht/pull/326))
  - fix(providers): gc ([libp2p/go-libp2p-kad-dht#325](https://github.com/libp2p/go-libp2p-kad-dht/pull/325))
  - Remove the old protocol from the defaults ([libp2p/go-libp2p-kad-dht#320](https://github.com/libp2p/go-libp2p-kad-dht/pull/320))
  - Fix some provider subsystem performance issues ([libp2p/go-libp2p-kad-dht#319](https://github.com/libp2p/go-libp2p-kad-dht/pull/319))
- github.com/libp2p/go-libp2p-peerstore (v0.0.2 -> v0.0.6):
  - segment the memory peerstore + granular locks ([libp2p/go-libp2p-peerstore#78](https://github.com/libp2p/go-libp2p-peerstore/pull/78))
  - don't delete under the read lock ([libp2p/go-libp2p-peerstore#76](https://github.com/libp2p/go-libp2p-peerstore/pull/76))
  - Read/Write locking ([libp2p/go-libp2p-peerstore#74](https://github.com/libp2p/go-libp2p-peerstore/pull/74))
  - optimize peerstore memory ([libp2p/go-libp2p-peerstore#71](https://github.com/libp2p/go-libp2p-peerstore/pull/71))
  - fix unmarshalling of peer IDs ([libp2p/go-libp2p-peerstore#72](https://github.com/libp2p/go-libp2p-peerstore/pull/72))
  - fix error handling in UpdateAddrs: return on error ([libp2p/go-libp2p-peerstore#70](https://github.com/libp2p/go-libp2p-peerstore/pull/70))
- github.com/libp2p/go-libp2p-pubsub (v0.0.1 -> v0.0.3):
  - rework validator pipeline ([libp2p/go-libp2p-pubsub#176](https://github.com/libp2p/go-libp2p-pubsub/pull/176))
  - Test adversarial signing ([libp2p/go-libp2p-pubsub#181](https://github.com/libp2p/go-libp2p-pubsub/pull/181))
  - Strict message signing by default ([libp2p/go-libp2p-pubsub#180](https://github.com/libp2p/go-libp2p-pubsub/pull/180))
- github.com/libp2p/go-libp2p-secio (v0.0.1 -> v0.0.3):
  - fix buffer size check ([libp2p/go-libp2p-secio#44](https://github.com/libp2p/go-libp2p-secio/pull/44))
- github.com/libp2p/go-libp2p-swarm (v0.0.2 -> v0.0.6):
  - dial: return a nice custom dial error ([libp2p/go-libp2p-swarm#121](https://github.com/libp2p/go-libp2p-swarm/pull/121))
- github.com/libp2p/go-libp2p-tls (null -> v0.0.1):
  - implement the new handshake ([libp2p/go-libp2p-tls#20](https://github.com/libp2p/go-libp2p-tls/pull/20))
  - use a prefix when signing the public key ([libp2p/go-libp2p-tls#26](https://github.com/libp2p/go-libp2p-tls/pull/26))
  - use ChaCha if one of the peers doesn't have AES hardware support ([libp2p/go-libp2p-tls#23](https://github.com/libp2p/go-libp2p-tls/pull/23))
  - improve peer verification ([libp2p/go-libp2p-tls#17](https://github.com/libp2p/go-libp2p-tls/pull/17))
  - add an example (mainly for development) ([libp2p/go-libp2p-tls#14](https://github.com/libp2p/go-libp2p-tls/pull/14))
- github.com/libp2p/go-libp2p-transport-upgrader (v0.0.1 -> v0.0.4):
  - improve correctness of closing connections on failure ([libp2p/go-libp2p-transport-upgrader#19](https://github.com/libp2p/go-libp2p-transport-upgrader/pull/19))
- github.com/libp2p/go-maddr-filter (v0.0.1 -> v0.0.4):
  - fix filter listing ([libp2p/go-maddr-filter#13](https://github.com/libp2p/go-maddr-filter/pull/13))
  - Reinstate deprecated Remove() method to reverse breakage ([libp2p/go-maddr-filter#12](https://github.com/libp2p/go-maddr-filter/pull/12))
  - Implement support for whitelists, default-deny/allow ([libp2p/go-maddr-filter#8](https://github.com/libp2p/go-maddr-filter/pull/8))
- github.com/libp2p/go-mplex (v0.0.1 -> v0.0.4):
  - disable write coalescing ([libp2p/go-mplex#61](https://github.com/libp2p/go-mplex/pull/61))
  - fix SetDeadline error conditions ([libp2p/go-mplex#59](https://github.com/libp2p/go-mplex/pull/59))
  - don't use contexts for deadlines ([libp2p/go-mplex#58](https://github.com/libp2p/go-mplex/pull/58))
  - don't reset on pathologies, just ignore the data ([libp2p/go-mplex#57](https://github.com/libp2p/go-mplex/pull/57))
  - coalesce writes ([libp2p/go-mplex#54](https://github.com/libp2p/go-mplex/pull/54))
  - read as much as we can in one go ([libp2p/go-mplex#53](https://github.com/libp2p/go-mplex/pull/53))
  - use timeouts when sending messages for stream open, close, and reset. ([libp2p/go-mplex#52](https://github.com/libp2p/go-mplex/pull/52))
  - fix: reset a stream even if closed remotely ([libp2p/go-mplex#50](https://github.com/libp2p/go-mplex/pull/50))
  - downgrade Error log to Warning ([libp2p/go-mplex#46](https://github.com/libp2p/go-mplex/pull/46))
  - Fix race condition by adding a mutex for deadline access ([libp2p/go-mplex#41](https://github.com/libp2p/go-mplex/pull/41))
- github.com/libp2p/go-msgio (v0.0.1 -> v0.0.2):
  - fix: never claim to read more than read ([libp2p/go-msgio#12](https://github.com/libp2p/go-msgio/pull/12))
- github.com/libp2p/go-ws-transport (v0.0.2 -> v0.0.4):
  - dep: import go-smux-* into the libp2p org ([libp2p/go-ws-transport#43](https://github.com/libp2p/go-ws-transport/pull/43))
  - replace gx instructions with note about gomod ([libp2p/go-ws-transport#42](https://github.com/libp2p/go-ws-transport/pull/42))


## 0.4.20 2019-04-16

We're happy to release go-ipfs 0.4.20. This release includes some critical
performance and stability fixes so all users should upgrade ASAP.

This is also the first release to use go modules instead of GX. While GX has
been a great way to dogfood an IPFS-based package manager, building and
maintaining a custom package manager is a _lot_ of work and we haven't been able
to dedicate enough time to bring the user experience of gx to an acceptable
level. You can read [#5850](https://github.com/ipfs/go-ipfs/issues/5850) for
some discussion on this matter.

### Docker

As of this release, it's now much easier to run arbitrary IPFS commands within
the docker container:

```bash
> docker run --name my-ipfs ipfs/go-ipfs:v0.4.20 config profile apply server # apply the server profile
> docker start my-ipfs # start the daemon
```

This release also [reverts](https://github.com/ipfs/go-ipfs/pull/6040) a change that
caused some significant trouble in 0.4.19. If you've been running into Docker
permission errors in 0.4.19, please upgrade.

### WebUI

This release contains a major
[WebUI](https://github.com/ipfs-shipyard/ipfs-webui) release with some
significant improvements to the file browser and new opt-in, privately hosted,
anonymous usage analytics.

### Commands

As usual, we've made several changes and improvements to our commands. The most
notable changes are listed in this section.

#### New: `ipfs version deps`

This release includes a new command, `ipfs version deps`, to list all
dependencies (with versions) of the current go-ipfs build. This should make it
easy to tell exactly how go-ipfs was built when tracking down issues.

#### New: `ipfs add URL`

The `ipfs add` command has gained support for URLs. This means you can:

1. Add files with `ipfs add URL` instead of downloading the file first.
2. Replace all uses of the `ipfs urlstore` command with a call to `ipfs add
   --nocopy`. The `ipfs urlstore` command will be deprecated in a future
   release.


#### Changed: `ipfs swarm connect`

The `ipfs swarm connect` command has a few new features:

It now marks the newly created connection as "important". This should ensure
that the connection manager won't come along later and close the connection if
it doesn't think it's being used.

It can now resolve `/dnsaddr` addresses that _don't_ end in a peer ID. For
example, you can now run `ipfs swarm connect /dnsaddr/bootstrap.libp2p.io` to
connect to one of the bootstrap peers at random. NOTE: This could connect you to
an _arbitrary_ peer as DNS is not secure (by default). Please do not rely on
this except for testing or unless you know what you're doing.

Finally, `ipfs swarm connect` now returns _all_ errors on failure. This should
make it much easier to debug connectivity issues. For example, one might see an
error like:

```
Error: connect QmYou failure: dial attempt failed: 6 errors occurred:
	* <peer.ID Qm*Me> --> <peer.ID Qm*You> (/ip4/127.0.0.1/tcp/4001) dial attempt failed: dial tcp4 127.0.0.1:4001: connect: connection refused
	* <peer.ID Qm*Me> --> <peer.ID Qm*You> (/ip6/::1/tcp/4001) dial attempt failed: dial tcp6 [::1]:4001: connect: connection refused
	* <peer.ID Qm*Me> --> <peer.ID Qm*You> (/ip6/2604::1/tcp/4001) dial attempt failed: dial tcp6 [2604::1]:4001: connect: network is unreachable
	* <peer.ID Qm*Me> --> <peer.ID Qm*You> (/ip6/2602::1/tcp/4001) dial attempt failed: dial tcp6 [2602::1]:4001: connect: network is unreachable
	* <peer.ID Qm*Me> --> <peer.ID Qm*You> (/ip4/150.0.1.2/tcp/4001) dial attempt failed: dial tcp4 0.0.0.0:4001->150.0.1.2:4001: i/o timeout
	* <peer.ID Qm*Me> --> <peer.ID Qm*You> (/ip4/200.0.1.2/tcp/4001) dial attempt failed: dial tcp4 0.0.0.0:4001->200.0.1.2:4001: i/o timeout
```

#### Changed: `ipfs bitswap stat`

`ipfs bitswap stat` no longer lists bitswap partners unless the `-v` flag is
passed. That is, it will now return:

```
> ipfs bitswap stat
bitswap status
	provides buffer: 0 / 256
	blocks received: 0
	blocks sent: 79
	data received: 0
	data sent: 672706
	dup blocks received: 0
	dup data received: 0 B
	wantlist [0 keys]
	partners [197]
```

Instead of:

```
> ipfs bitswap stat -v
bitswap status
	provides buffer: 0 / 256
	blocks received: 0
	blocks sent: 79
	data received: 0
	data sent: 672706
	dup blocks received: 0
	dup data received: 0 B
	wantlist [0 keys]
	partners [203]
		QmNQTTTRCDpCYCiiu6TYWCqEa7ShAUo9jrZJvWngfSu1mL
		QmNWaxbqERvdcgoWpqAhDMrbK2gKi3SMGk3LUEvfcqZcf4
		QmNgSVpgZVEd41pBX6DyCaHRof8UmUJLqQ3XH2qNL9xLvN
        ... omitting 200 lines ...
```

#### Changed: `ipfs repo stat --human`

The `--human` flag in the `ipfs repo stat` command now intelligently picks a
size unit instead of always using MiB.

#### Changed: `ipfs resolve` (`ipfs dns`, `ipfs name resolve`)

All of the resolve commands now:

1. Resolve _recursively_ (up to 32 steps) by default to better match user
   expectations (these commands used to be non-recursive by default). To turn
   recursion off, pass `-r false`.
2. When resolving non-recursively, these commands no longer fail when partially
   resolving a name. Instead, they simply return the intermediate result.

#### Changed: `ipfs files flush`

The `ipfs files flush` command now returns the CID of the flushed file.

### Performance And Reliability

This release has the usual collection of performance and reliability
improvements.

#### Badger Memory Usage

Those of you using the badger datastore should notice reduced memory usage in
this release due to some upstream changes. Badger still uses significantly more
memory than the default datastore configuration but this will hopefully continue
to improve.

#### Bitswap

We fixed some critical CPU utilization regressions in bitswap for this release.
If you've been noticing CPU _regressions_ in go-ipfs 0.4.19, especially when
running a public gateway, upgrading to 0.4.20 will likely fix them.

#### Relays

After AutoRelay was introduced in go-ipfs 0.4.19, the number of peers connecting
through relays skyrocketed to over 120K concurrent peers. This highlighted some
performance issues that we've now fixed in this release. Specifically:

* We've significantly reduced the amount of memory allocated per-peer.
* We've fixed a bug where relays might, in rare cases, try to actively dial a
  peer to relay traffic. By default, relays only forward traffic between peers
  already connected to the relay.
* We've fixed quite a number of performance issues that only show up when
  rapidly forming new connections. This will actually help _all_ nodes but will
  especially help relays.
  
If you've enabled relay _hop_ (`Swarm.EnableRelayHop`) in go-ipfs 0.4.19 and it
hasn't burned down your machine yet, this release should improve things
significantly. However, relays are still under heavy load so running an open
relay will continue to be resource intensive.

We're continuing to investigate this issue and have a few more patches on the
way that, unfortunately, won't make it into this release.

#### Panics

We've fixed two notable panics in this release:

* We've fixed a frequent panic in the DHT.
* We've fixed an occasional panic in the experimental QUIC transport.

### Content Routing

IPFS announces and finds content by sending and retrieving content routing
("provider") records to and from the DHT. Unfortunately, sending out these
records can be quite resource intensive.

This release has two changes to alleviate this: a reduced number of initial
provide workers and a persistent provider queue.

We've reduced the number of parallel initial provide workers (workers that send
out provider records when content is initially added to go-ipfs) from 512 to 6.
Each provide request (currently, due to some issues in our DHT) tries to
establish hundreds of connections, significantly impacting the performance of
go-ipfs and [crashing some
routers](https://github.com/ipfs/go-ipfs/issues/3320).

We've introduced a new persistent provider queue for files added via `ipfs add`
and `ipfs pin add`. When new directory trees are added to go-ipfs, go-ipfs will
add the root/final CID to this queue. Then, in the background, go-ipfs will walk
the queue, sequentially sending out provider records for each CID.

This ensures that root CIDs are sent out as soon as possible and are sent even
when files are added when the go-ipfs daemon isn't running.

By example, let's add a directory tree to go-ipfs:

```bash
> # We're going to do this in "online" mode first so let's start the daemon.
> ipfs daemon &
...
Daemon is ready
> # Now, we're going to create a directory to add.
> mkdir foo
> for i in {0..1000}; do echo do echo $i > foo/$i; done
> # finally, we're going to add it.
> ipfs add -r foo
added QmUQcSjQx2bg4cSe2rUZyQi6F8QtJFJb74fWL7D784UWf9 foo/0
...
added QmQac2chFyJ24yfG2Dfuqg1P5gipLcgUDuiuYkQ5ExwGap foo/990
added QmQWwz9haeQ5T2QmQeXzqspKdowzYELShBCLzLJjVa2DuV foo/991
added QmQ5D4MtHUN4LTS4n7mgyHyaUukieMMyCfvnzXQAAbgTJm foo/992
added QmZq4n4KRNq3k1ovzxJ4qdQXZSrarfJjnoLYPR3ztHd7EY foo/993
added QmdtrsuVf8Nf1s1MaSjLAd54iNqrn1KN9VoFNgKGnLgjbt foo/994
added QmbstvU9mnW2hsE94WFmw5WbrXdLTu2Sf9kWWSozrSDscL foo/995
added QmXFd7f35gAnmisjfFmfYKkjA3F3TSpvUYB9SXr6tLsdg8 foo/996
added QmV5BxS1YQ9V227Np2Cq124cRrFDAyBXNMqHHa6kpJ9cr6 foo/997
added QmcXsccUtwKeQ1SuYC3YgyFUeYmAR9CXwGGnT3LPeCg5Tx foo/998
added Qmc4mcQcpaNzyDQxQj5SyxwFg9ZYz5XBEeEZAuH4cQirj9 foo/999
added QmXpXzUhcS9edmFBuVafV5wFXKjfXkCQcjAUZsTs7qFf3G foo
```

In 0.4.19, we would have sent out provider records for files `foo/{0..1000}`
_before_ sending out a provider record for `foo`. If you were ask a friend to
download /ipfs/QmUQcSjQx2bg4cSe2rUZyQi6F8QtJFJb74fWL7D784UWf9, they would
(baring other issues) be able to find it pretty quickly as this is the first CID
you'll have announced to the network. However, if you ask your friend to
download /ipfs/QmXpXzUhcS9edmFBuVafV5wFXKjfXkCQcjAUZsTs7qFf3G/0, they'll have to
wait for you to finish telling the network about every file in `foo` first.

In 0.4.20, we _immediately_ tell the network about
`QmXpXzUhcS9edmFBuVafV5wFXKjfXkCQcjAUZsTs7qFf3G` (the `foo` directory) as soon
as we finish adding the directory to go-ipfs _without_ waiting to finish
announcing `foo/{0..1000}`. This is especially important in this release
because we've drastically reduced the number of provide workers.

The second benefit is that this queue is persistent. That means go-ipfs won't
forget to send out this record, even if it was offline when the content was
initially added. NOTE: go-ipfs _does_ continuously _re_-send provider records in
the background twice a day, it just might be a while before it gets around to
sending one out any specific one.

### Bitswap

Bitswap now periodically re-sends its wantlist to connected peers. This should
help work around some race conditions we've seen in bitswap where one node wants
a block but the other doesn't know for some reason.

You can track this issue here: https://github.com/ipfs/go-ipfs/issues/5183.

### Improved NAT Traversal

While NATs are still p2p enemy #1, this release includes slightly improved
support for traversing them.

Specifically, this release now:

1. Better detects the "gateway" NAT, even when multiple devices on the network
   _claim_ to be NATs.
2. Better guesses the external IP address when port mapping, even when the
   gateway lies.

### Reduced AutoRelay Boot Time

The experimental AutoRelay feature can now detect NATs _much_ faster as we've
reduced initial NAT detection delay to 15 seconds. There's still room for
improvement but this should make nodes that have enabled this feature dialable
earlier on start.

### Changelogs

- github.com/ipfs/go-ipfs:
  - gitattributes: avoid normalizing known binary files ([ipfs/go-ipfs#6209](https://github.com/ipfs/go-ipfs/pull/6209))
  - gitattributes: default to LF ([ipfs/go-ipfs#6198](https://github.com/ipfs/go-ipfs/pull/6198))
  - Fix level db panic ([ipfs/go-ipfs#6186](https://github.com/ipfs/go-ipfs/pull/6186))
  - Dockerfile: Remove 2 year old deprecation warning ([ipfs/go-ipfs#6188](https://github.com/ipfs/go-ipfs/pull/6188))
  - align output for the command ipfs object stat ([ipfs/go-ipfs#6189](https://github.com/ipfs/go-ipfs/pull/6189))
  - provider queue: don't repeatedly retry the same item if we fail ([ipfs/go-ipfs#6187](https://github.com/ipfs/go-ipfs/pull/6187))
  - test: remove version/deps from ro commands test ([ipfs/go-ipfs#6185](https://github.com/ipfs/go-ipfs/pull/6185))
  - feat: add version deps command [modversion] ([ipfs/go-ipfs#6115](https://github.com/ipfs/go-ipfs/pull/6115))
  - readme: update for go modules ([ipfs/go-ipfs#6180](https://github.com/ipfs/go-ipfs/pull/6180))
  - Switch to Go 1.12 ([ipfs/go-ipfs#6144](https://github.com/ipfs/go-ipfs/pull/6144))
  - ci: avoid interleaving output from different sharness tests ([ipfs/go-ipfs#6175](https://github.com/ipfs/go-ipfs/pull/6175))
  - fix two bugs where the repo may not properly be closed ([ipfs/go-ipfs#6176](https://github.com/ipfs/go-ipfs/pull/6176))
  - fix error check in swarm connect ([ipfs/go-ipfs#6174](https://github.com/ipfs/go-ipfs/pull/6174))
  - feat(coreapi): tag all explicit connect requests in the connection manager ([ipfs/go-ipfs#6171](https://github.com/ipfs/go-ipfs/pull/6171))
  - chore: remove CODEOWNERS ([ipfs/go-ipfs#6172](https://github.com/ipfs/go-ipfs/pull/6172))
  - feat: update to IPFS Web UI 2.4.4 ([ipfs/go-ipfs#6169](https://github.com/ipfs/go-ipfs/pull/6169))
  - fix add error handling ([ipfs/go-ipfs#6156](https://github.com/ipfs/go-ipfs/pull/6156))
  - chore: remove waffle ([ipfs/go-ipfs#6157](https://github.com/ipfs/go-ipfs/pull/6157))
  - chore: fix a bunch of issues caught by golangci-lint ([ipfs/go-ipfs#6140](https://github.com/ipfs/go-ipfs/pull/6140))
  - docs/experimental-features.md: link to ipfs-ds-convert ([ipfs/go-ipfs#6154](https://github.com/ipfs/go-ipfs/pull/6154))
  - interrupt: fix send on closed ([ipfs/go-ipfs#6147](https://github.com/ipfs/go-ipfs/pull/6147))
  - docs: document Gateway.Writable not Gateway.Writeable ([ipfs/go-ipfs#6151](https://github.com/ipfs/go-ipfs/pull/6151))
  - Fuse fixes ([ipfs/go-ipfs#6135](https://github.com/ipfs/go-ipfs/pull/6135))
  - Remove duplicate blockstore from the package list ([ipfs/go-ipfs#6138](https://github.com/ipfs/go-ipfs/pull/6138))
  - Query for provider head/tail ([ipfs/go-ipfs#6125](https://github.com/ipfs/go-ipfs/pull/6125))
  - Remove dead link from ISSUE_TEMPLATE.md ([ipfs/go-ipfs#6128](https://github.com/ipfs/go-ipfs/pull/6128))
  - coreapi: remove Unixfs.Wrap ([ipfs/go-ipfs#6123](https://github.com/ipfs/go-ipfs/pull/6123))
  - coreapi unixfs: change Wrap logic to make more sense  ([ipfs/go-ipfs#6019](https://github.com/ipfs/go-ipfs/pull/6019))
  - deps: switch back to jbenet go-is-domain ([ipfs/go-ipfs#6119](https://github.com/ipfs/go-ipfs/pull/6119))
  - command repo stat: add human flag tests to t0080-repo.sh ([ipfs/go-ipfs#6116](https://github.com/ipfs/go-ipfs/pull/6116))
  - gc: fix a potential deadlock ([ipfs/go-ipfs#6112](https://github.com/ipfs/go-ipfs/pull/6112))
  - fix config options in osxfuse error messages ([ipfs/go-ipfs#6105](https://github.com/ipfs/go-ipfs/pull/6105))
  - Command repo stat: improve human flag behavior ([ipfs/go-ipfs#6106](https://github.com/ipfs/go-ipfs/pull/6106))
  - Provide root node immediately on add and pin add ([ipfs/go-ipfs#6068](https://github.com/ipfs/go-ipfs/pull/6068))
  - gomod: Update Dockerfile, remove Dockerfile.fast ([ipfs/go-ipfs#6100](https://github.com/ipfs/go-ipfs/pull/6100))
  - Return CID from 'ipfs files flush'  ([ipfs/go-ipfs#6102](https://github.com/ipfs/go-ipfs/pull/6102))
  - resolve: fix recursion ([ipfs/go-ipfs#6087](https://github.com/ipfs/go-ipfs/pull/6087))
  - fix(swarm): add dnsaddr support in swarm connect ([ipfs/go-ipfs#5535](https://github.com/ipfs/go-ipfs/pull/5535))
  - make in-memory datastore thread-safe ([ipfs/go-ipfs#6085](https://github.com/ipfs/go-ipfs/pull/6085))
  - Update package table to remove broken jenkins links ([ipfs/go-ipfs#6084](https://github.com/ipfs/go-ipfs/pull/6084))
  - mk: fix maketarball to work with gomod ([ipfs/go-ipfs#6078](https://github.com/ipfs/go-ipfs/pull/6078))
  - fix ls command to use the new coreinterface types ([ipfs/go-ipfs#6051](https://github.com/ipfs/go-ipfs/pull/6051))
  - mk: remove install_unsupported, leave a note ([ipfs/go-ipfs#6063](https://github.com/ipfs/go-ipfs/pull/6063))
  - mk: change git-hash command to include information about modifications ([ipfs/go-ipfs#6060](https://github.com/ipfs/go-ipfs/pull/6060))
  - mk: fix make install by not setting GOBIN ([ipfs/go-ipfs#6059](https://github.com/ipfs/go-ipfs/pull/6059))
  - go: require Golang 1.11.4 ([ipfs/go-ipfs#6057](https://github.com/ipfs/go-ipfs/pull/6057))
  - yamux: increase yamux window size to 8MiB. ([ipfs/go-ipfs#6049](https://github.com/ipfs/go-ipfs/pull/6049))
  - Introduce go modules [yey] ([ipfs/go-ipfs#6038](https://github.com/ipfs/go-ipfs/pull/6038))
  - cleanup daemon online logic ([ipfs/go-ipfs#6050](https://github.com/ipfs/go-ipfs/pull/6050))
  - ci: test on 32bit os ([ipfs/go-ipfs#5429](https://github.com/ipfs/go-ipfs/pull/5429))
  - feat/cmds: hide peers info default in bitswap stat ([ipfs/go-ipfs#5820](https://github.com/ipfs/go-ipfs/pull/5820))
  - Improve CLI help pages ([ipfs/go-ipfs#6013](https://github.com/ipfs/go-ipfs/pull/6013))
  - Close #6044 ([ipfs/go-ipfs#6045](https://github.com/ipfs/go-ipfs/pull/6045))
  - commands(dht): return final error ([ipfs/go-ipfs#6034](https://github.com/ipfs/go-ipfs/pull/6034))
  - Revert "Really run as non-root user in docker container" ([ipfs/go-ipfs#6040](https://github.com/ipfs/go-ipfs/pull/6040))
- github.com/ipfs/go-bitswap:
  - feat(messagequeue): rebroadcast wantlist ([ipfs/go-bitswap#106](https://github.com/ipfs/go-bitswap/pull/106))
  - reduce provide workers to 6 ([ipfs/go-bitswap#93](https://github.com/ipfs/go-bitswap/pull/93))
  - Reduce memory allocation ([ipfs/go-bitswap#103](https://github.com/ipfs/go-bitswap/pull/103))
  - refactor(messagequeue): remove dead code ([ipfs/go-bitswap#98](https://github.com/ipfs/go-bitswap/pull/98))
  - fix: limit use of custom context type ([ipfs/go-bitswap#89](https://github.com/ipfs/go-bitswap/pull/89))
  - fix: remove non-error log message ([ipfs/go-bitswap#91](https://github.com/ipfs/go-bitswap/pull/91))
  - fix(messagequeue): Remove second run loop ([ipfs/go-bitswap#94](https://github.com/ipfs/go-bitswap/pull/94))
- github.com/ipfs/go-blockservice:
  - Revert "Remove verifcid as it is handled in go-cid" ([ipfs/go-blockservice#25](https://github.com/ipfs/go-blockservice/pull/25))
  - Remove verifcid as it is handled in go-cid ([ipfs/go-blockservice#23](https://github.com/ipfs/go-blockservice/pull/23))
- github.com/ipfs/go-datastore:
  - cleanup and optimize naive query filters ([ipfs/go-datastore#125](https://github.com/ipfs/go-datastore/pull/125))
  - Fix – sorted limited offset mount queries ([ipfs/go-datastore#124](https://github.com/ipfs/go-datastore/pull/124))
  - Fix function comments based on best practices from Effective Go ([ipfs/go-datastore#122](https://github.com/ipfs/go-datastore/pull/122))
  - remove ThreadSafeDatastore ([ipfs/go-datastore#120](https://github.com/ipfs/go-datastore/pull/120))
  - Splinter TTLDatastore interface into TTL + Datastore ([ipfs/go-datastore#118](https://github.com/ipfs/go-datastore/pull/118))
- github.com/ipfs/go-ds-badger:
  - tweak the default options ([ipfs/go-ds-badger#52](https://github.com/ipfs/go-ds-badger/pull/52))
  - remove thread-safe assertion ([ipfs/go-ds-badger#55](https://github.com/ipfs/go-ds-badger/pull/55))
  - make memory-safe against concurrent closure/operations ([ipfs/go-ds-badger#53](https://github.com/ipfs/go-ds-badger/pull/53))
  - make badger use our logging framework ([ipfs/go-ds-badger#50](https://github.com/ipfs/go-ds-badger/pull/50))
- github.com/ipfs/go-ds-flatfs:
  - remove thread-safe assertion ([ipfs/go-ds-flatfs#53](https://github.com/ipfs/go-ds-flatfs/pull/53))
- github.com/ipfs/go-ds-leveldb:
  - Fast reverse query ([ipfs/go-ds-leveldb#28](https://github.com/ipfs/go-ds-leveldb/pull/28))
  - remove thread-safe assertion ([ipfs/go-ds-leveldb#27](https://github.com/ipfs/go-ds-leveldb/pull/27))
- github.com/ipfs/go-ipfs-cmdkit:
  - Extract files package ([ipfs/go-ipfs-cmdkit#31](https://github.com/ipfs/go-ipfs-cmdkit/pull/31))
- github.com/ipfs/go-ipfs-cmds:
  - sync: add yet another sync error ([ipfs/go-ipfs-cmds#161](https://github.com/ipfs/go-ipfs-cmds/pull/161))
  - Removed broken link from readme ([ipfs/go-ipfs-cmds#159](https://github.com/ipfs/go-ipfs-cmds/pull/159))
  - Fix broken link in readme ([ipfs/go-ipfs-cmds#160](https://github.com/ipfs/go-ipfs-cmds/pull/160))
  - set WebFile fpath to URL base ([ipfs/go-ipfs-cmds#158](https://github.com/ipfs/go-ipfs-cmds/pull/158))
  - Handle stdin name in cli/parse ([ipfs/go-ipfs-cmds#157](https://github.com/ipfs/go-ipfs-cmds/pull/157))
  - support url paths as files.WebFile ([ipfs/go-ipfs-cmds#154](https://github.com/ipfs/go-ipfs-cmds/pull/154))
  - typed encoder: improve pointer reflection ([ipfs/go-ipfs-cmds#155](https://github.com/ipfs/go-ipfs-cmds/pull/155))
  - cli: don't sync output to NUL on Windows ([ipfs/go-ipfs-cmds#153](https://github.com/ipfs/go-ipfs-cmds/pull/153))
- github.com/ipfs/go-ipfs-files:
  - return url as AbsPath from WebFile to implement FileInfo ([ipfs/go-ipfs-files#13](https://github.com/ipfs/go-ipfs-files/pull/13))
  - fix the content disposition header ([ipfs/go-ipfs-files#14](https://github.com/ipfs/go-ipfs-files/pull/14))
  - go format ([ipfs/go-ipfs-files#15](https://github.com/ipfs/go-ipfs-files/pull/15))
  - simplify content type checking ([ipfs/go-ipfs-files#9](https://github.com/ipfs/go-ipfs-files/pull/9))
  - remove extra webfile test code ([ipfs/go-ipfs-files#12](https://github.com/ipfs/go-ipfs-files/pull/12))
- github.com/ipfs/go-merkledag:
  - add function to marshal raw nodes to json ([ipfs/go-merkledag#36](https://github.com/ipfs/go-merkledag/pull/36))
  - fix some performance regressions when reading protobuf nodes ([ipfs/go-merkledag#34](https://github.com/ipfs/go-merkledag/pull/34))
- github.com/ipfs/go-metrics-interface:
  - update the counter interface to match prometheus ([ipfs/go-metrics-interface#2](https://github.com/ipfs/go-metrics-interface/pull/2))
- github.com/ipfs/go-mfs:
  - Return node from FlushPath ([ipfs/go-mfs#72](https://github.com/ipfs/go-mfs/pull/72))
  - Wire up context to FlushPath ([ipfs/go-mfs#70](https://github.com/ipfs/go-mfs/pull/70))
- github.com/ipfs/interface-go-ipfs-core:
  - don't close the top-level addr ([ipfs/interface-go-ipfs-core#25](https://github.com/ipfs/interface-go-ipfs-core/pull/25))
  - fix a bunch of small test "bugs" ([ipfs/interface-go-ipfs-core#24](https://github.com/ipfs/interface-go-ipfs-core/pull/24))
  - remove Wrap ([ipfs/interface-go-ipfs-core#21](https://github.com/ipfs/interface-go-ipfs-core/pull/21))
  - Unixfs.Wrap Fixes ([ipfs/interface-go-ipfs-core#10](https://github.com/ipfs/interface-go-ipfs-core/pull/10))
  - tweak the Ls interface ([ipfs/interface-go-ipfs-core#14](https://github.com/ipfs/interface-go-ipfs-core/pull/14))
- github.com/libp2p/go-buffer-pool:
  - Enable tests ([libp2p/go-buffer-pool#6](https://github.com/libp2p/go-buffer-pool/pull/6))
- github.com/libp2p/go-flow-metrics:
  - Just repair spelling mistake ([libp2p/go-flow-metrics#3](https://github.com/libp2p/go-flow-metrics/pull/3))
- github.com/libp2p/go-libp2p:
  - Deprecate gx in readme & link to workspace repo ([libp2p/go-libp2p#591](https://github.com/libp2p/go-libp2p/pull/591))
  - Respect nodial option in routed host ([libp2p/go-libp2p#590](https://github.com/libp2p/go-libp2p/pull/590))
  - fix panic in observed address activation check ([libp2p/go-libp2p#586](https://github.com/libp2p/go-libp2p/pull/586))
  - Improve observed address handling ([libp2p/go-libp2p#585](https://github.com/libp2p/go-libp2p/pull/585))
  - identify: avoid parsing/printing multiaddrs ([libp2p/go-libp2p#583](https://github.com/libp2p/go-libp2p/pull/583))
  - move things outside of the lock in obsaddr ([libp2p/go-libp2p#582](https://github.com/libp2p/go-libp2p/pull/582))
  - identify: be more careful about the addresses we store ([libp2p/go-libp2p#577](https://github.com/libp2p/go-libp2p/pull/577))
  - relay: turn autorelay into a service and always filter out relay addresses ([libp2p/go-libp2p#578](https://github.com/libp2p/go-libp2p/pull/578))
  - chore: fail in the libp2p constructor if we fail to store the key ([libp2p/go-libp2p#576](https://github.com/libp2p/go-libp2p/pull/576))
  - Fix broken link in README.md ([libp2p/go-libp2p#580](https://github.com/libp2p/go-libp2p/pull/580))
  - Link to docs & discuss in readme ([libp2p/go-libp2p#571](https://github.com/libp2p/go-libp2p/pull/571))
  - Reduce autorelay boot delay and correctly handle private->public transition ([libp2p/go-libp2p#570](https://github.com/libp2p/go-libp2p/pull/570))
  - reduce nat error level ([libp2p/go-libp2p#568](https://github.com/libp2p/go-libp2p/pull/568))
  - relay: simplify declaration of multiaddr var ([libp2p/go-libp2p#563](https://github.com/libp2p/go-libp2p/pull/563))
  - Fix UDP listen on a Unspecified Address and Dial from the Unspecified Address ([libp2p/go-libp2p#561](https://github.com/libp2p/go-libp2p/pull/561))
  - Remove jenkins column from package table ([libp2p/go-libp2p#562](https://github.com/libp2p/go-libp2p/pull/562))
  - Fix typos in p2p/net/README.md ([libp2p/go-libp2p#555](https://github.com/libp2p/go-libp2p/pull/555))
  - better nat mapping ([libp2p/go-libp2p#549](https://github.com/libp2p/go-libp2p/pull/549))
- github.com/libp2p/go-libp2p-autonat:
  - fully close the autonat client stream ([libp2p/go-libp2p-autonat#21](https://github.com/libp2p/go-libp2p-autonat/pull/21))
  - parallelize dialbacks ([libp2p/go-libp2p-autonat#20](https://github.com/libp2p/go-libp2p-autonat/pull/20))
  - Pacify the race detector ([libp2p/go-libp2p-autonat#17](https://github.com/libp2p/go-libp2p-autonat/pull/17))
- github.com/libp2p/go-libp2p-autonat-svc:
  - full close the autonat stream ([libp2p/go-libp2p-autonat-svc#20](https://github.com/libp2p/go-libp2p-autonat-svc/pull/20))
  - reduce dialback timeout to 15s ([libp2p/go-libp2p-autonat-svc#17](https://github.com/libp2p/go-libp2p-autonat-svc/pull/17))
- github.com/libp2p/go-libp2p-circuit:
  - use buffer pool in newDelimitedReader ([libp2p/go-libp2p-circuit#71](https://github.com/libp2p/go-libp2p-circuit/pull/71))
  - Use NoDial option when opening hop streams for non-active relays ([libp2p/go-libp2p-circuit#70](https://github.com/libp2p/go-libp2p-circuit/pull/70))
  - use io.CopyBuffer with explicitly allocated buffers ([libp2p/go-libp2p-circuit#69](https://github.com/libp2p/go-libp2p-circuit/pull/69))
  - docs and nits ([libp2p/go-libp2p-circuit#66](https://github.com/libp2p/go-libp2p-circuit/pull/66))
- github.com/libp2p/go-libp2p-kad-dht:
  - dialQueue: start the control loop later ([libp2p/go-libp2p-kad-dht#312](https://github.com/libp2p/go-libp2p-kad-dht/pull/312))
  - make it work in wasm ([libp2p/go-libp2p-kad-dht#310](https://github.com/libp2p/go-libp2p-kad-dht/pull/310))
  - Revert "GoModules: Checksum mismatch:" ([libp2p/go-libp2p-kad-dht#309](https://github.com/libp2p/go-libp2p-kad-dht/pull/309))
  - defer dialqueue action until initial peers have been added ([libp2p/go-libp2p-kad-dht#301](https://github.com/libp2p/go-libp2p-kad-dht/pull/301))
- github.com/libp2p/go-libp2p-nat:
  - switch to libp2p's go-nat fork ([libp2p/go-libp2p-nat#16](https://github.com/libp2p/go-libp2p-nat/pull/16))
  - remove all uses of multiaddrs ([libp2p/go-libp2p-nat#14](https://github.com/libp2p/go-libp2p-nat/pull/14))
- github.com/libp2p/go-libp2p-net:
  - fix WithNoDial to return the context ([libp2p/go-libp2p-net#43](https://github.com/libp2p/go-libp2p-net/pull/43))
  - NoDial context option ([libp2p/go-libp2p-net#42](https://github.com/libp2p/go-libp2p-net/pull/42))
- github.com/libp2p/go-libp2p-peer:
  - Let ID implement encoding.Binary[Un]Marshaler and encoding.Text[Un]Marshaler ([libp2p/go-libp2p-peer#44](https://github.com/libp2p/go-libp2p-peer/pull/44))
- github.com/libp2p/go-libp2p-peerstore:
  - keep temp addresses for 2 minutes ([libp2p/go-libp2p-peerstore#67](https://github.com/libp2p/go-libp2p-peerstore/pull/67))
  - migrate to multiformats/go-base32 ([libp2p/go-libp2p-peerstore#61](https://github.com/libp2p/go-libp2p-peerstore/pull/61))
- github.com/libp2p/go-libp2p-protocol:
  - update readme ([libp2p/go-libp2p-protocol#6](https://github.com/libp2p/go-libp2p-protocol/pull/6))
  - Enable standard Travis CI tests. ([libp2p/go-libp2p-protocol#5](https://github.com/libp2p/go-libp2p-protocol/pull/5))
  - Fix go get address. ([libp2p/go-libp2p-protocol#4](https://github.com/libp2p/go-libp2p-protocol/pull/4))
  - Add MIT license ([libp2p/go-libp2p-protocol#3](https://github.com/libp2p/go-libp2p-protocol/pull/3))
  - Standardized Readme ([libp2p/go-libp2p-protocol#2](https://github.com/libp2p/go-libp2p-protocol/pull/2))
- github.com/libp2p/go-libp2p-pubsub-router:
  - gx publish 0.5.17 ([libp2p/go-libp2p-pubsub-router#26](https://github.com/libp2p/go-libp2p-pubsub-router/pull/26))
- github.com/libp2p/go-libp2p-quic-transport:
  - update quic-go to v0.11.0 ([libp2p/go-libp2p-quic-transport#54](https://github.com/libp2p/go-libp2p-quic-transport/pull/54))
- github.com/libp2p/go-libp2p-routing-helpers:
  - fix(put): fail if any router fails ([libp2p/go-libp2p-routing-helpers#19](https://github.com/libp2p/go-libp2p-routing-helpers/pull/19))
- github.com/libp2p/go-libp2p-swarm:
  - Add context option to disable dialing when opening a new stream ([libp2p/go-libp2p-swarm#116](https://github.com/libp2p/go-libp2p-swarm/pull/116))
  - return all dial errors if dial has failed ([libp2p/go-libp2p-swarm#115](https://github.com/libp2p/go-libp2p-swarm/pull/115))
  - Differentiate no addresses error from no good addresses ([libp2p/go-libp2p-swarm#113](https://github.com/libp2p/go-libp2p-swarm/pull/113))
- github.com/libp2p/go-libp2p-transport:
  - tests: constrain concurrency with race detector. ([libp2p/go-libp2p-transport#47](https://github.com/libp2p/go-libp2p-transport/pull/47))
  - pick test timeout from env var if available. ([libp2p/go-libp2p-transport#46](https://github.com/libp2p/go-libp2p-transport/pull/46))
  - increase test timeout. ([libp2p/go-libp2p-transport#45](https://github.com/libp2p/go-libp2p-transport/pull/45))
- github.com/libp2p/go-msgio:
  - Improve test coverage ([libp2p/go-msgio#10](https://github.com/libp2p/go-msgio/pull/10))
- github.com/libp2p/go-reuseport:
  - fix: add wasm build tag to wasm module ([libp2p/go-reuseport#70](https://github.com/libp2p/go-reuseport/pull/70))
- github.com/libp2p/go-reuseport-transport:
  - don't set linger to 0 ([libp2p/go-reuseport-transport#14](https://github.com/libp2p/go-reuseport-transport/pull/14))
- github.com/libp2p/go-tcp-transport:
  - set linger to 0 for both inbound and outbound connections ([libp2p/go-tcp-transport#36](https://github.com/libp2p/go-tcp-transport/pull/36))
- github.com/libp2p/go-ws-transport:
  - modernize request handling ([libp2p/go-ws-transport#41](https://github.com/libp2p/go-ws-transport/pull/41))

## 0.4.19 2019-03-01

We're happy to announce go 0.4.19. This release contains a bunch of important
fixes and a slew of new and improved features. Get pumped and upgrade ASAP to benefit from all the new goodies! 🎁

### Features

#### 🔌 Initializing With Random Ports

Go-ipfs can now be configured to listen on a random but _stable_ port (across
restarts) using the new `randomports` configuration profile. This should be
helpful when testing and/or running multiple go-ipfs instances on a single
machine.

To initialize a go-ipfs instance with a randomly chosen port, run:

```bash
> ipfs init --profile=randomports
```

#### 👂 Gateway Directory Listing

IPNS (and/or DNSLink) directory listings on the gateway, e.g.
https://ipfs.io/ipns/dist.ipfs.io/go-ipfs/, will now display the _ipfs_ hash of
the current directory. This way users can more easily create permanent links to
otherwise mutable data.

#### 📡 AutoRelay and AutoNAT

This release introduces two new experimental features (courtesy of libp2p):
AutoRelay and AutoNAT.

AutoRelay is a new service that automatically chooses a public relay when it
detects that the go-ipfs node is behind a NAT. While relaying connections
through a third-party node isn't the most efficient way to route around NATs,
it's a reliable fallback.

To enable AutoRelay, set the `Swarm.EnableAutoRelay` option in the config.

AutoNAT is the service AutoRelay uses to detect if the node is behind a NAT. You
don't have to set any special config flags to enable it.

In this same config section, you may also notice options like `EnableRelayHop`,
`EnableAutoNATService`, etc. You _do not_ need to enable these:

* `EnableRelayHop` -- Allow _other_ nodes to use _your_ node as a relay
  (disabled by default).
* `EnableAutoNATService` -- Help _other_ nodes detect if they're behind a NAT
  (disabled by default).

#### 📵 Offline Operation

There are two new "offline" features in this release: a global `--offline` flag
and an option to configure the gateway to not fetch files.

Most go-ipfs commands now support the `--offline` flag. This causes IPFS to avoid
network operations when performing the requested operation. If you've ever used
the `--local` flag, the `--offline` flag is the (almost) universally supported
replacement.

For example:

* If the daemon is started with `ipfs daemon --offline`, it won't even _connect_
  to the network. (note: this feature isn't new, just an example).
* `ipfs add --offline some_file` won't send out provider records.
* `ipfs cat --offline Qm...` won't fetch any blocks from the network.
* `ipfs block stat --offline Qm...` is a great way to tell if a block is locally
  available.

Note: It doesn't _yet_ work with the `refs`, `urlstore`, or `tar` commands
([#6002](https://github.com/ipfs/go-ipfs/issues/6002)).

On to the gateway, there's a new `Gateway.NoFetch` option to configure the
gateway to only serve locally present files. This makes it possible to run an
IPFS node as a gateway to serve content of _your_ choosing without acting like a
public proxy. 🤫

#### 📍 Adding And Pinning Content

There's a new `--pin` flag for both `ipfs block put` and `ipfs urlstore add` to
match the `--pin` flag in `ipfs add`. This allows one to atomically add and pin
content with these APIs.

**NOTE 1:** For `ipfs urlstore add`, `--pin` has been enabled _by default_ to
match the behavior in `ipfs add`. However, `ipfs block put` _does not_ pin by
default to match the _current_ behavior.

**NOTE 2:** If you had previously used the urlstore and _weren't_ explicitly
pinning content after adding it, it isn't pinned and running the garbage
collector will delete it. While technically documented in the `ipfs urlstore
add` helptext, this behavior was non-obvious and bears mentioning.

#### 🗂 File Listing

The `ipfs ls` command has two significant changes this release: it reports
_file_ sizes instead of _dag_ sizes and has gained a new `--stream` flag.

First up, `ipfs ls` now reports _file_ sizes instead of _dag_ sizes. Previously,
for historical reasons, `ipfs ls` would report the size of a file/directory as
seen by IPFS _including_ all the filesystem datastructures and metadata.
However, this meant that `ls -l` and `ipfs ls` would print _different_ sizes:

```bash
> ipfs ls /ipfs/QmS4ustL54uo8FzR9455qaxZwuMiUhyvMcX9Ba8nUH4uVv

QmZTR5bcpQD7cFgTorqxZDYaew1Wqgfbd2ud9QqGPAkK2V 1688 about
QmYCvbfNbCwFR45HiNP45rwJgvatpiW38D961L5qAhUM5Y 200  contact
QmY5heUM5qgRubMDD1og9fhCPA6QdkMp3QCwd4s7gJsyE7 322  help
QmejvEPop4D7YUadeGqYWmZxHhLc4JBUCzJJHWMzdcMe2y 12   ping
QmXgqKTbzdh83pQtKFb19SpMCpDDcKR2ujqk3pKph9aCNF 1692 quick-start
QmPZ9gcCEpqKTo6aq61g2nXGUhM4iCL3ewB6LDXZCtioEB 1102 readme
QmQ5vhrL7uv6tuoN9KeVBwd4PwfQkXdVVmDLUZuTNxqgvm 1173 security-notes

> ipfs get /ipfs/QmS4ustL54uo8FzR9455qaxZwuMiUhyvMcX9Ba8nUH4uVv
Saving file(s) to QmS4ustL54uo8FzR9455qaxZwuMiUhyvMcX9Ba8nUH4uVv
 6.39 KiB / 6.39 KiB [================================] 100.00% 0s

> ls -l QmS4ustL54uo8FzR9455qaxZwuMiUhyvMcX9Ba8nUH4uVv
total 28
-rw------- 1 user group 1677 Feb 14 17:03 about
-rw------- 1 user group  189 Feb 14 17:03 contact
-rw------- 1 user group  311 Feb 14 17:03 help
-rw------- 1 user group    4 Feb 14 17:03 ping
-rw------- 1 user group 1681 Feb 14 17:03 quick-start
-rw------- 1 user group 1091 Feb 14 17:03 readme
-rw------- 1 user group 1162 Feb 14 17:03 security-notes
```

This is now no longer the case. `ipfs ls` and `ls -l` now return the _same_
sizes. 🙌

Second up, `ipfs ls` now has a new `--stream` flag. In IPFS, very large
directories (e.g., Wikipedia) are split up into multiple chunks (shards) as
there are too many entries to fit in a single block. Unfortunately, `ipfs ls`
buffers the _entire_ file list in memory and then sorts it. This means that
`ipfs ls /ipfs/QmXoypizjW3WknFiJnKLwHCnL72vedxjQkDDP1mXWo6uco/wiki` (wikipedia)
will take a _very_ long time to return anything (it'll also use quite a bit of
memory).

However, the new `--stream` flag makes it possible to stream a directory listing
as new chunks are fetched from the network. To test this, you can run `ipfs ls
--stream --size=false --resolve-type=false
/ipfs/QmXoypizjW3WknFiJnKLwHCnL72vedxjQkDDP1mXWo6uco/wiki`. You probably won't
want to wait for that command to finish, Wikipedia has a _lot_ of entries. 😉

#### 🔁 HTTP Proxy

This release sees a new (experimental) feature contributed by our friends at
[Peergos](https://peergos.org): HTTP proxy over libp2p. When enabled, the local
gateway can act as an HTTP proxy and forward HTTP requests to libp2p peers. When
combined with the `ipfs p2p` command, users can use this to expose HTTP services
to other go-ipfs nodes via their gateways. For details, check out the
[documentation](https://github.com/ipfs/go-ipfs/blob/master/docs/experimental-features.md#p2p-http-proxy).

### Performance And Reliability

This release introduces quite a few performance/reliability improvements and, as
usual, fixes several memory leaks. Below is a non-exhaustive list of noticeable changes.

#### 📞 DHT

This release includes an important DHT fix that should significantly:

1. Reduce dialing.
2. Speed up DHT queries.
3. Improve performance of the gateways.

Basically, in the worst case, a DHT query would turn into a random walk of the
entire IPFS network. Yikes!

Relevant PR: https://github.com/libp2p/go-libp2p-kad-dht/pull/237

#### 🕸 Bitswap

Bitswap sessions have improved and are now used for _all_ requests. Sessions
allow us to group related content and ask peers most likely to _have_ the
content instead of broadcasting the request to all connected peers. This gives
us two significant benefits:

1. Less wasted upload bandwidth. Instead of broadcasting which blocks we want to
   everyone, we can ask fewer peers thus reducing the number of requests we send
   out.
2. Less wasted download bandwidth. Because we _know_ which peers likely have
   content, we can ask an individual peer for a block and expect to get an
   answer. In the past, we'd ask every peer at the same time to optimize for
   latency at the expense of bandwidth (getting the same block from multiple
   peers). We had to do this because we had to assume that _most_ peers didn't
   have the requested block.

#### ‼️ Pubsub

This release includes some significant reliability improvements in pubsub
subscription handling. If you've previously had issues with connected pubsub
peers _not_ seeing each-other's messages, please upgrade ASAP.

#### ♻️ Reuseport

In this release, we've rewritten our previously error-prone `go-reuseport`
library to _not_ duplicate a significant portion of Go's low-level networking
code. This was made possible by Go's new `Control`
[`net.Dialer`](https://golang.org/pkg/net/#Dialer) option.

In the past, our first suggestion to anyone experiencing weird resource or
connectivity issues was to disable `REUSEPORT` (set `IPFS_REUSEPORT` to false).
This should no longer be necessary.

#### 🐺 Badger Datastore

[Badger has reached 1.0][badger-release]. This release brings an audit and
numerous reliability fixes. We are now reasonably confident that badger will
become the default datastore in a future release. 👍

[badger-release]: https://blog.dgraph.io/post/releasing-v1.0/

This release also adds a new `Truncate` configuration option for the badger
datastore (enabled by default for new IPFS nodes). When enabled, badger will
_delete_ any un-synced data on start instead of simply refusing to start. This
should be safe on all filesystems where the `sync` operation is safe and removes
the need for manual intervention when restarting an IPFS node after a crash.

Assuming you initialized your badger repo with `ipfs init --profile=badgerds`,
you can enable truncate on an existing repo by running: `ipfs config --json
"Datastore.Spec.child.truncate" true`.

### Refactors and Endeavors

#### 🕹 Commands Library

The legacy commands library shim has now been completely removed. This won't
mean much for many users but the go-ipfs team is happy to have this behind them.

#### 🌐 Base32 CIDs

This release can now encode CIDs in responses in bases other than base58. This
is primarily useful for web-browser integration as it allows us to (a) encode
CIDs in a lower-case base (e.g., base32) and then use them in the _origin_ part
of URLs. The take away is: this release brings us a step closer to better
browser integration.

Specifically, this release adds two flags:

1. `--cid-base`: When specified, the IPFS CLI will encode all CIDv1 CIDs using the
   requested base.
2. `--upgrade-cidv0-in-output`: When specified, the IPFS CLI will _upgrade_ CIDv0
   CIDs to CIDv1 CIDs when returning them to the user. This upgrade is necessary
   because CIDv0 doesn't support multibase however, it's off by default as it
   changes the _binary_ representation of the CIDs (which could have unintended
   consequences).

#### 🎛 CoreAPI

The work on the CoreAPI refactor ([ipfs/go-ipfs#4498][]) has progressed leaps and
bounds this release. The CoreAPI is a comprehensive programmatic interface
designed to allow go-ipfs be used as a daemon or a library interchangeably.

As of this release, go-ipfs now has:

* External interface definitions in [ipfs/interface-go-ipfs-core][].
* A work-in-progress implementation ([ipfs/go-ipfs-http-client][]) of these
  interfaces that uses the IPFS HTTP API. This will replace the
  ([ipfs/go-ipfs-api][]) library.
* A new plugin type ["Daemon"][daemon-plugin]. Daemon plugins are started and
  stopped along with the go-ipfs daemon and are instantiated with a copy of the
  CoreAPI. This allows them to control and extend the go-ipfs daemon from within
  the daemon itself.

The next steps are:

1. Finishing the remaining API surface area. At the moment, the two key missing
   parts are:
  1. Config manipulation.
  2. The `ipfs files` API.
1. Finalizing the [ipfs/go-ipfs-http-client][] implementation.
2. Creating a simple way to construct and initialize a go-ipfs node when using
   go-ipfs as a library.

[ipfs/go-ipfs#4498]: https://github.com/ipfs/go-ipfs/issues/4498
[ipfs/interface-go-ipfs-core]: https://github.com/ipfs/interface-go-ipfs-core
[ipfs/go-ipfs-http-client]: https://github.com/ipfs/go-ipfs-http-client
[ipfs/go-ipfs-api]: https://github.com/ipfs/go-ipfs-http-client
[daemon-plugin]: https://github.com/ipfs/go-ipfs/blob/master/docs/plugins.md#daemon

### Changelogs

- github.com/ipfs/go-ipfs:
  - fix: show interactive output from install.sh ([ipfs/go-ipfs#6024](https://github.com/ipfs/go-ipfs/pull/6024))
  - fix: return the shortest, completely resolved path in the resolve command ([ipfs/go-ipfs#5704](https://github.com/ipfs/go-ipfs/pull/5704))
  - fix a few interop test issues ([ipfs/go-ipfs#6004](https://github.com/ipfs/go-ipfs/pull/6004))
  - fix HAMT bookmark ln ([ipfs/go-ipfs#6005](https://github.com/ipfs/go-ipfs/pull/6005))
  - docs: document Gateway.NoFetch ([ipfs/go-ipfs#5999](https://github.com/ipfs/go-ipfs/pull/5999))
  - Improve "name publish" ttl option documentation ([ipfs/go-ipfs#5979](https://github.com/ipfs/go-ipfs/pull/5979))
  - fix(cmd/mv): dst filename error ([ipfs/go-ipfs#5964](https://github.com/ipfs/go-ipfs/pull/5964))
  - coreapi: extract interface ([ipfs/go-ipfs#5978](https://github.com/ipfs/go-ipfs/pull/5978))
  - coreapi: cleanup non-gx references ([ipfs/go-ipfs#5976](https://github.com/ipfs/go-ipfs/pull/5976))
  - coreapi: fix seek test on http impl ([ipfs/go-ipfs#5971](https://github.com/ipfs/go-ipfs/pull/5971))
  - block put --pin ([ipfs/go-ipfs#5969](https://github.com/ipfs/go-ipfs/pull/5969))
  - Port `ipfs ls` to CoreAPI ([ipfs/go-ipfs#5962](https://github.com/ipfs/go-ipfs/pull/5962))
  - docs: duplicate default helptext in `name publish` ([ipfs/go-ipfs#5960](https://github.com/ipfs/go-ipfs/pull/5960))
  - plugin: add a daemon plugin with access to the CoreAPI ([ipfs/go-ipfs#5955](https://github.com/ipfs/go-ipfs/pull/5955))
  - coreapi: add some seeker tests ([ipfs/go-ipfs#5934](https://github.com/ipfs/go-ipfs/pull/5934))
  - Refactor ipfs get to use CoreAPI ([ipfs/go-ipfs#5943](https://github.com/ipfs/go-ipfs/pull/5943))
  - refact(cmd/init): change string option to const ([ipfs/go-ipfs#5949](https://github.com/ipfs/go-ipfs/pull/5949))
  - cmds/pin: use coreapi/pin ([ipfs/go-ipfs#5843](https://github.com/ipfs/go-ipfs/pull/5843))
  - Only perform DNSLink lookups on fully qualified domain names (FQDN) ([ipfs/go-ipfs#5950](https://github.com/ipfs/go-ipfs/pull/5950))
  - Fix DontCheckOSXFUSE config command example ([ipfs/go-ipfs#5951](https://github.com/ipfs/go-ipfs/pull/5951))
  - refact(cmd/config): change string option to const ([ipfs/go-ipfs#5948](https://github.com/ipfs/go-ipfs/pull/5948))
  - clarification the document of --resolve flag in name.publish ([ipfs/go-ipfs#5651](https://github.com/ipfs/go-ipfs/pull/5651))
  - Drop some coreunix code ([ipfs/go-ipfs#5938](https://github.com/ipfs/go-ipfs/pull/5938))
  - commands: fix verbose flag ([ipfs/go-ipfs#5940](https://github.com/ipfs/go-ipfs/pull/5940))
  - Fixes #4558 ([ipfs/go-ipfs#5937](https://github.com/ipfs/go-ipfs/pull/5937))
  - Port dag commansds to CoreAPI ([ipfs/go-ipfs#5939](https://github.com/ipfs/go-ipfs/pull/5939))
  - mfs: make sure to flush after mv and chcid ([ipfs/go-ipfs#5936](https://github.com/ipfs/go-ipfs/pull/5936))
  - docs/code-flow : Add code flow documentation for add cmd. ([ipfs/go-ipfs#5864](https://github.com/ipfs/go-ipfs/pull/5864))
  - coreapi: few more error check fixes ([ipfs/go-ipfs#5935](https://github.com/ipfs/go-ipfs/pull/5935))
  - Fixed and cleaned up TestIpfsStressRead ([ipfs/go-ipfs#5920](https://github.com/ipfs/go-ipfs/pull/5920))
  - Clarify that chunker sizes are in bytes ([ipfs/go-ipfs#5923](https://github.com/ipfs/go-ipfs/pull/5923))
  - refact(cmd/patch): change string to const ([ipfs/go-ipfs#5931](https://github.com/ipfs/go-ipfs/pull/5931))
  - refact(cmd/object): change option string to const ([ipfs/go-ipfs#5932](https://github.com/ipfs/go-ipfs/pull/5932))
  - coreapi: replace coreiface.DagAPI with ipld.DAGService ([ipfs/go-ipfs#5922](https://github.com/ipfs/go-ipfs/pull/5922))
  - Add global option to specify the multibase encoding (server side) ([ipfs/go-ipfs#5789](https://github.com/ipfs/go-ipfs/pull/5789))
  - coreapi: Adjust some tests for go-ipfs-http-api ([ipfs/go-ipfs#5926](https://github.com/ipfs/go-ipfs/pull/5926))
  - chore: update to Web UI v2.3.3 ([ipfs/go-ipfs#5928](https://github.com/ipfs/go-ipfs/pull/5928))
  - ls: Report real file size ([ipfs/go-ipfs#5906](https://github.com/ipfs/go-ipfs/pull/5906))
  - Improve the Filestore document ([ipfs/go-ipfs#5927](https://github.com/ipfs/go-ipfs/pull/5927))
  - [CORS] Bubble go-ipfs-cmds 2.0.10 - Updates CORS library ([ipfs/go-ipfs#5919](https://github.com/ipfs/go-ipfs/pull/5919))
  - reduce verbosity of daemon start ([ipfs/go-ipfs#5904](https://github.com/ipfs/go-ipfs/pull/5904))
  - feat: update to Web UI v2.3.2 ([ipfs/go-ipfs#5899](https://github.com/ipfs/go-ipfs/pull/5899))
  - CoreAPI: Don't panic when testing incomplete implementions ([ipfs/go-ipfs#5900](https://github.com/ipfs/go-ipfs/pull/5900))
  - gateway: fix CORs headers ([ipfs/go-ipfs#5893](https://github.com/ipfs/go-ipfs/pull/5893))
  - Local Gateway option ([ipfs/go-ipfs#5649](https://github.com/ipfs/go-ipfs/pull/5649))
  - Show hash on gateway ([ipfs/go-ipfs#5830](https://github.com/ipfs/go-ipfs/pull/5830))
  - fix: ulimit docs mistake ([ipfs/go-ipfs#5894](https://github.com/ipfs/go-ipfs/pull/5894))
  - Move coreapi tests to the interface ([ipfs/go-ipfs#5865](https://github.com/ipfs/go-ipfs/pull/5865))
  - Move checkHelptextRecursive forward a bit ([ipfs/go-ipfs#5889](https://github.com/ipfs/go-ipfs/pull/5889))
  - coreapi/unixfs: Use path instead of raw hash in AddEvent ([ipfs/go-ipfs#5854](https://github.com/ipfs/go-ipfs/pull/5854))
  - Fix name resolve --offline ([ipfs/go-ipfs#5885](https://github.com/ipfs/go-ipfs/pull/5885))
  - testing: slow down republisher sharness test ([ipfs/go-ipfs#5856](https://github.com/ipfs/go-ipfs/pull/5856))
  - docs: flesh out plugin documentation ([ipfs/go-ipfs#5876](https://github.com/ipfs/go-ipfs/pull/5876))
  -  main: move InterruptHandler to util  ([ipfs/go-ipfs#5872](https://github.com/ipfs/go-ipfs/pull/5872))
  - make: fix building source tarball on macos ([ipfs/go-ipfs#5860](https://github.com/ipfs/go-ipfs/pull/5860))
  - fix config data race ([ipfs/go-ipfs#5634](https://github.com/ipfs/go-ipfs/pull/5634))
  - CoreAPI: Global offline option ([ipfs/go-ipfs#5825](https://github.com/ipfs/go-ipfs/pull/5825))
  - Update for go-ipfs-files refactor ([ipfs/go-ipfs#5661](https://github.com/ipfs/go-ipfs/pull/5661))
  - feat: update Web UI to v2.3.0 ([ipfs/go-ipfs#5855](https://github.com/ipfs/go-ipfs/pull/5855))
  - Stateful plugin loading ([ipfs/go-ipfs#4806](https://github.com/ipfs/go-ipfs/pull/4806))
  - startup: always load the private key ([ipfs/go-ipfs#5844](https://github.com/ipfs/go-ipfs/pull/5844))
  - add --dereference-args parameter ([ipfs/go-ipfs#5801](https://github.com/ipfs/go-ipfs/pull/5801))
  - config: document the connection manager ([ipfs/go-ipfs#5839](https://github.com/ipfs/go-ipfs/pull/5839))
  - add pinning support to the urlstore ([ipfs/go-ipfs#5834](https://github.com/ipfs/go-ipfs/pull/5834))
  - refact(cmd/cat): remove useless code ([ipfs/go-ipfs#5836](https://github.com/ipfs/go-ipfs/pull/5836))
  - Really run as non-root user in docker container ([ipfs/go-ipfs#5048](https://github.com/ipfs/go-ipfs/pull/5048))
  - README: document guix package ([ipfs/go-ipfs#5832](https://github.com/ipfs/go-ipfs/pull/5832))
  - docs: Improve config documentation ([ipfs/go-ipfs#5829](https://github.com/ipfs/go-ipfs/pull/5829))
  - block: rm extra output ([ipfs/go-ipfs#5751](https://github.com/ipfs/go-ipfs/pull/5751))
  - merge github-issue-guide with the issue template ([ipfs/go-ipfs#4636](https://github.com/ipfs/go-ipfs/pull/4636))
  - docs: fix inconsistent capitalization of "API". ([ipfs/go-ipfs#5824](https://github.com/ipfs/go-ipfs/pull/5824))
  - Update README.md ([ipfs/go-ipfs#5818](https://github.com/ipfs/go-ipfs/pull/5818))
  - CONTRIBUTING.md link ([ipfs/go-ipfs#5811](https://github.com/ipfs/go-ipfs/pull/5811))
  - README: Update required Go version ([ipfs/go-ipfs#5813](https://github.com/ipfs/go-ipfs/pull/5813))
  - p2p: report-peer-id option for listen ([ipfs/go-ipfs#5771](https://github.com/ipfs/go-ipfs/pull/5771))
  - really fix netcat race ([ipfs/go-ipfs#5803](https://github.com/ipfs/go-ipfs/pull/5803))
  - [http_proxy_over_p2p] ([ipfs/go-ipfs#5526](https://github.com/ipfs/go-ipfs/pull/5526))
  - coreapi/pin: Use CID's directly in maps instead of converting to string ([ipfs/go-ipfs#5809](https://github.com/ipfs/go-ipfs/pull/5809))
  - Gx update go-merkledag and related deps. ([ipfs/go-ipfs#5802](https://github.com/ipfs/go-ipfs/pull/5802))
  - cmds: rm old lib ([ipfs/go-ipfs#5786](https://github.com/ipfs/go-ipfs/pull/5786))
  - badger: add truncate flag ([ipfs/go-ipfs#5625](https://github.com/ipfs/go-ipfs/pull/5625))
  - docker: allow IPFS_PROFILE to choose the profile for `ipfs init` ([ipfs/go-ipfs#5473](https://github.com/ipfs/go-ipfs/pull/5473))
  - Add --stream option to `ls` command ([ipfs/go-ipfs#5611](https://github.com/ipfs/go-ipfs/pull/5611))
  - Switch to using request.Context() ([ipfs/go-ipfs#5782](https://github.com/ipfs/go-ipfs/pull/5782))
  - Update go-ipfs-delay and assoc deps ([ipfs/go-ipfs#5762](https://github.com/ipfs/go-ipfs/pull/5762))
  - Suppress bootstrap error ([ipfs/go-ipfs#5769](https://github.com/ipfs/go-ipfs/pull/5769))
  - ISSUE_TEMPLATE: move the support question comment to the very top ([ipfs/go-ipfs#5770](https://github.com/ipfs/go-ipfs/pull/5770))
  - cmds: use MakeTypedEncoder ([ipfs/go-ipfs#5760](https://github.com/ipfs/go-ipfs/pull/5760))
  - cmds/bitswap: sort wantlist ([ipfs/go-ipfs#5759](https://github.com/ipfs/go-ipfs/pull/5759))
  - cmds/update: use new cmds lib ([ipfs/go-ipfs#5730](https://github.com/ipfs/go-ipfs/pull/5730))
  - cmds/file: use new cmds lib ([ipfs/go-ipfs#5756](https://github.com/ipfs/go-ipfs/pull/5756))
  - cmds: remove reduntant func ([ipfs/go-ipfs#5750](https://github.com/ipfs/go-ipfs/pull/5750))
  - commands/refs: use new cmds ([ipfs/go-ipfs#5679](https://github.com/ipfs/go-ipfs/pull/5679))
  - commands/pin: use new cmds lib ([ipfs/go-ipfs#5674](https://github.com/ipfs/go-ipfs/pull/5674))
  - commands/boostrap: use new cmds ([ipfs/go-ipfs#5678](https://github.com/ipfs/go-ipfs/pull/5678))
  - fix(cmd/add): progressbar output error when input is read from stdin ([ipfs/go-ipfs#5743](https://github.com/ipfs/go-ipfs/pull/5743))
  - unexport GOFLAGS ([ipfs/go-ipfs#5747](https://github.com/ipfs/go-ipfs/pull/5747))
  - refactor(cmds): use new cmds ([ipfs/go-ipfs#5659](https://github.com/ipfs/go-ipfs/pull/5659))
  - commands/filestore: use new cmds lib ([ipfs/go-ipfs#5673](https://github.com/ipfs/go-ipfs/pull/5673))
  - Fix broken links ([ipfs/go-ipfs#5721](https://github.com/ipfs/go-ipfs/pull/5721))
  - fix `ipfs help` bug #5557 ([ipfs/go-ipfs#5573](https://github.com/ipfs/go-ipfs/pull/5573))
  - commands/bitswap: use new cmds lib ([ipfs/go-ipfs#5676](https://github.com/ipfs/go-ipfs/pull/5676))
  - refact(cmd/repo): repo's sub cmds uses new cmd lib ([ipfs/go-ipfs#5677](https://github.com/ipfs/go-ipfs/pull/5677))
  - fix the maketarball script ([ipfs/go-ipfs#5718](https://github.com/ipfs/go-ipfs/pull/5718))
  - output link to WebUI on daemon startup ([ipfs/go-ipfs#5729](https://github.com/ipfs/go-ipfs/pull/5729))
  - Move persistent datastores to plugins ([ipfs/go-ipfs#5695](https://github.com/ipfs/go-ipfs/pull/5695))
  - Update IPTB test ([ipfs/go-ipfs#5636](https://github.com/ipfs/go-ipfs/pull/5636))
  - enhance(cmd/verify): add goroutine count to improve verify speed ([ipfs/go-ipfs#5710](https://github.com/ipfs/go-ipfs/pull/5710))
  - Update go-mfs and go-unixfs ([ipfs/go-ipfs#5714](https://github.com/ipfs/go-ipfs/pull/5714))
  - fix(flag/version): flag `all` should have a higher priority ([ipfs/go-ipfs#5719](https://github.com/ipfs/go-ipfs/pull/5719))
  - commands/p2p: use new cmds lib ([ipfs/go-ipfs#5672](https://github.com/ipfs/go-ipfs/pull/5672))
  - commands/dht: use new cmds lib ([ipfs/go-ipfs#5671](https://github.com/ipfs/go-ipfs/pull/5671))
  - commands/object: use new cmds ([ipfs/go-ipfs#5666](https://github.com/ipfs/go-ipfs/pull/5666))
  - commands/files: use new cmds ([ipfs/go-ipfs#5665](https://github.com/ipfs/go-ipfs/pull/5665))
  - cmds/env: add a config path helper ([ipfs/go-ipfs#5712](https://github.com/ipfs/go-ipfs/pull/5712))
- github.com/ipfs/dir-index-html:
  - show hash if given ([ipfs/dir-index-html#21](https://github.com/ipfs/dir-index-html/pull/21))
  - Add "jpeg" as an alias to "jpg". ([ipfs/dir-index-html#16](https://github.com/ipfs/dir-index-html/pull/16))
- github.com/libp2p/go-addr-util:
  - Improve test coverage ([libp2p/go-addr-util#14](https://github.com/libp2p/go-addr-util/pull/14))
- github.com/ipfs/go-bitswap:
  - fix(prq): fix a bunch of goroutine leaks and deadlocks ([ipfs/go-bitswap#87](https://github.com/ipfs/go-bitswap/pull/87))
  - remove allocations round two ([ipfs/go-bitswap#84](https://github.com/ipfs/go-bitswap/pull/84))
  - fix(bitswap): remove CancelWants function ([ipfs/go-bitswap#80](https://github.com/ipfs/go-bitswap/pull/80))
  - Avoid allocating for wantlist entries ([ipfs/go-bitswap#79](https://github.com/ipfs/go-bitswap/pull/79))
  - ci(Jenkins): remove Jenkinsfile ([ipfs/go-bitswap#83](https://github.com/ipfs/go-bitswap/pull/83))
  - More specific wantlists ([ipfs/go-bitswap#74](https://github.com/ipfs/go-bitswap/pull/74))
  - fix(wantlist): remove races on setup ([ipfs/go-bitswap#72](https://github.com/ipfs/go-bitswap/pull/72))
  - fix multiple data races ([ipfs/go-bitswap#76](https://github.com/ipfs/go-bitswap/pull/76))
  - ci: add travis ([ipfs/go-bitswap#75](https://github.com/ipfs/go-bitswap/pull/75))
  - providers: don't add every connected node as a provider ([ipfs/go-bitswap#59](https://github.com/ipfs/go-bitswap/pull/59))
  - refactor(GetBlocks): Merge session/non-session ([ipfs/go-bitswap#64](https://github.com/ipfs/go-bitswap/pull/64))
  - Feat: A more robust provider finder for sessions (for now) and soon for all bitswap ([ipfs/go-bitswap#60](https://github.com/ipfs/go-bitswap/pull/60))
  - fix(tests): stabilize session tests ([ipfs/go-bitswap#63](https://github.com/ipfs/go-bitswap/pull/63))
  - contexts: make sure to abort when a context is canceled ([ipfs/go-bitswap#58](https://github.com/ipfs/go-bitswap/pull/58))
  - fix(sessions): explicitly connect found peers ([ipfs/go-bitswap#56](https://github.com/ipfs/go-bitswap/pull/56))
  - Speed up sessions Round #1 ([ipfs/go-bitswap#27](https://github.com/ipfs/go-bitswap/pull/27))
  - Fix debug log formatting issues ([ipfs/go-bitswap#37](https://github.com/ipfs/go-bitswap/pull/37))
  - Feat/bandwidth limited tests ([ipfs/go-bitswap#42](https://github.com/ipfs/go-bitswap/pull/42))
  - fix(tests): stabilize unreliable session tests ([ipfs/go-bitswap#44](https://github.com/ipfs/go-bitswap/pull/44))
  - Bitswap Refactor #4: Extract session peer manager from sessions ([ipfs/go-bitswap#26](https://github.com/ipfs/go-bitswap/pull/26))
  - Bitswap Refactor #3: Extract sessions to package ([ipfs/go-bitswap#30](https://github.com/ipfs/go-bitswap/pull/30))
  - docs(comments): end comment sentences to have full-stop ([ipfs/go-bitswap#33](https://github.com/ipfs/go-bitswap/pull/33))
  - Bitswap Refactor #2: Extract PeerManager From Want Manager + Unit Test ([ipfs/go-bitswap#29](https://github.com/ipfs/go-bitswap/pull/29))
  - Bitswap Refactor #1: Session Manager & Extract Want Manager ([ipfs/go-bitswap#28](https://github.com/ipfs/go-bitswap/pull/28))
  - fix(Receiver): Ignore unwanted blocks ([ipfs/go-bitswap#24](https://github.com/ipfs/go-bitswap/pull/24))
  - feat(Benchmarks): Add real world dup blocks test ([ipfs/go-bitswap#25](https://github.com/ipfs/go-bitswap/pull/25))
  - Feat/bitswap pr improvements ([ipfs/go-bitswap#19](https://github.com/ipfs/go-bitswap/pull/19))
- github.com/ipfs/go-blockservice:
  - Don't return errors on closed exchange ([ipfs/go-blockservice#15](https://github.com/ipfs/go-blockservice/pull/15))
- github.com/ipfs/go-cid:
  - fix inline CIDs generated by Prefix.Sum ([ipfs/go-cid#84](https://github.com/ipfs/go-cid/pull/84))
  - Let Cid implement Binary[Un]Marshaler and Text[Un]Marshaler interfaces. ([ipfs/go-cid#81](https://github.com/ipfs/go-cid/pull/81))
  - fix typo in comment ([ipfs/go-cid#80](https://github.com/ipfs/go-cid/pull/80))
  - add codecs for Dash blocks, tx ([ipfs/go-cid#78](https://github.com/ipfs/go-cid/pull/78))
- github.com/ipfs/go-cidutil:
  - Fix Travis CI to run all tests. ([ipfs/go-cidutil#11](https://github.com/ipfs/go-cidutil/pull/11))
  - Changes needed for `--cid-base` option in go-ipfs (simplified vesion) ([ipfs/go-cidutil#10](https://github.com/ipfs/go-cidutil/pull/10))
  - add a utility method for sorting CID slices ([ipfs/go-cidutil#5](https://github.com/ipfs/go-cidutil/pull/5))
- github.com/libp2p/go-conn-security:
  - fix link to usage example in README ([libp2p/go-conn-security#4](https://github.com/libp2p/go-conn-security/pull/4))
- github.com/ipfs/go-datastore:
  - interfaces: make GetBacked* take a Read instead of a Datastore ([ipfs/go-datastore#115](https://github.com/ipfs/go-datastore/pull/115))
  - remove closer type assertions ([ipfs/go-datastore#112](https://github.com/ipfs/go-datastore/pull/112))
  - remove io.Closer from the transaction interface ([ipfs/go-datastore#113](https://github.com/ipfs/go-datastore/pull/113))
  - feat(datastore): expose datastore Close() ([ipfs/go-datastore#111](https://github.com/ipfs/go-datastore/pull/111))
  - query: make datastore ordering act like a user would expect ([ipfs/go-datastore#110](https://github.com/ipfs/go-datastore/pull/110))
  - delayed: implement io.Closer and export datastore type. ([ipfs/go-datastore#108](https://github.com/ipfs/go-datastore/pull/108))
  - split the datastore into a read and a write interface ([ipfs/go-datastore#107](https://github.com/ipfs/go-datastore/pull/107))
  - Describe behavior of Batching datastores ([ipfs/go-datastore#105](https://github.com/ipfs/go-datastore/pull/105))
  - handle concurrent puts/deletes in BasicBatch ([ipfs/go-datastore#103](https://github.com/ipfs/go-datastore/pull/103))
  - add a GetSize method ([ipfs/go-datastore#99](https://github.com/ipfs/go-datastore/pull/99))
- github.com/ipfs/go-ds-badger:
  - removed additional/wasteful Prefix conversion ([ipfs/go-ds-badger#45](https://github.com/ipfs/go-ds-badger/pull/45))
  - Enable Jenkins ([ipfs/go-ds-badger#35](https://github.com/ipfs/go-ds-badger/pull/35))
  - fix application or ordering for interface change ([ipfs/go-ds-badger#44](https://github.com/ipfs/go-ds-badger/pull/44))
  - Update badger ([ipfs/go-ds-badger#40](https://github.com/ipfs/go-ds-badger/pull/40))
- github.com/ipfs/go-ds-flatfs:
  - fix a goroutine leak killing the gateways ([ipfs/go-ds-flatfs#51](https://github.com/ipfs/go-ds-flatfs/pull/51))
- github.com/ipfs/go-ds-leveldb:
  - Expose Datastore type ([ipfs/go-ds-leveldb#20](https://github.com/ipfs/go-ds-leveldb/pull/20))
  - fix application or ordering for interface change ([ipfs/go-ds-leveldb#23](https://github.com/ipfs/go-ds-leveldb/pull/23))
- github.com/ipfs/go-ipfs-cmds:
  - fix sync error with go1.12 on darwin ([ipfs/go-ipfs-cmds#147](https://github.com/ipfs/go-ipfs-cmds/pull/147))
  - cli: fix ignoring std{out,err} sync errors on windows ([ipfs/go-ipfs-cmds#146](https://github.com/ipfs/go-ipfs-cmds/pull/146))
  - roundup of cleanup fixes ([ipfs/go-ipfs-cmds#144](https://github.com/ipfs/go-ipfs-cmds/pull/144))
  - Update cors library ([ipfs/go-ipfs-cmds#139](https://github.com/ipfs/go-ipfs-cmds/pull/139))
  - expand on the api error ([ipfs/go-ipfs-cmds#138](https://github.com/ipfs/go-ipfs-cmds/pull/138))
  - set the connection close header if we have a body to read ([ipfs/go-ipfs-cmds#116](https://github.com/ipfs/go-ipfs-cmds/pull/116))
  - print a nicer error on timeout/cancel ([ipfs/go-ipfs-cmds#137](https://github.com/ipfs/go-ipfs-cmds/pull/137))
  - Add link traversal option ([ipfs/go-ipfs-cmds#96](https://github.com/ipfs/go-ipfs-cmds/pull/96))
  - Don't skip stdin test on Windows ([ipfs/go-ipfs-cmds#136](https://github.com/ipfs/go-ipfs-cmds/pull/136))
  - MakeTypedEncoder: accept results by pointer or value ([ipfs/go-ipfs-cmds#134](https://github.com/ipfs/go-ipfs-cmds/pull/134))
- github.com/ipfs/go-ipfs-config:
  - Gateway.NoFetch ([ipfs/go-ipfs-config#19](https://github.com/ipfs/go-ipfs-config/pull/19))
  - add a Clone function ([ipfs/go-ipfs-config#16](https://github.com/ipfs/go-ipfs-config/pull/16))
  - randomports: give user ability to init ipfs using random port for swarm. ([ipfs/go-ipfs-config#17](https://github.com/ipfs/go-ipfs-config/pull/17))
  - Allow the use of the User-Agent header ([ipfs/go-ipfs-config#15](https://github.com/ipfs/go-ipfs-config/pull/15))
  - autorelay options ([ipfs/go-ipfs-config#21](https://github.com/ipfs/go-ipfs-config/pull/21))
  - profile: add badger truncate option ([ipfs/go-ipfs-config#20](https://github.com/ipfs/go-ipfs-config/pull/20))
- github.com/ipfs/go-ipfs-delay:
  - Feat/refactor wait time ([ipfs/go-ipfs-delay#1](https://github.com/ipfs/go-ipfs-delay/pull/1))
- github.com/ipfs/go-ipfs-files:
  - multipart: fix handling of common prefixes ([ipfs/go-ipfs-files#7](https://github.com/ipfs/go-ipfs-files/pull/7))
  - create implicit directories from multipart requests ([ipfs/go-ipfs-files#6](https://github.com/ipfs/go-ipfs-files/pull/6))
  - TarWriter ([ipfs/go-ipfs-files#4](https://github.com/ipfs/go-ipfs-files/pull/4))
  - Refactor filename - file relation ([ipfs/go-ipfs-files#2](https://github.com/ipfs/go-ipfs-files/pull/2))
- github.com/ipfs/go-ipld-cbor:
  - cbor: decode undefined as null ([ipfs/go-ipld-cbor#54](https://github.com/ipfs/go-ipld-cbor/pull/54))
  - error when trying to encode an empty link ([ipfs/go-ipld-cbor#52](https://github.com/ipfs/go-ipld-cbor/pull/52))
  - test for struct with both a cid and a bigint ([ipfs/go-ipld-cbor#51](https://github.com/ipfs/go-ipld-cbor/pull/51))
- github.com/ipfs/go-ipld-format:
  - Add a DAG walker with support for IPLD `Node`s ([ipfs/go-ipld-format#39](https://github.com/ipfs/go-ipld-format/pull/39))
  - Add BufferedDAG wrapping Batch as a DAGService. ([ipfs/go-ipld-format#48](https://github.com/ipfs/go-ipld-format/pull/48))
- github.com/ipfs/go-ipld-git:
  - Fix blob marshalling ([ipfs/go-ipld-git#37](https://github.com/ipfs/go-ipld-git/pull/37))
  - Re-enable assertion on commit size -- it is correct after #31 ([ipfs/go-ipld-git#33](https://github.com/ipfs/go-ipld-git/pull/33))
  - Use OS path separator in testing, fixes #30 ([ipfs/go-ipld-git#34](https://github.com/ipfs/go-ipld-git/pull/34))
  - Use rawdata length for size, fixes #7 ([ipfs/go-ipld-git#31](https://github.com/ipfs/go-ipld-git/pull/31))
  - Cache RawData for Commit, Tag, & Tree, fixes #6 ([ipfs/go-ipld-git#28](https://github.com/ipfs/go-ipld-git/pull/28))
  - Precompute Blob CID, fixes #21 ([ipfs/go-ipld-git#27](https://github.com/ipfs/go-ipld-git/pull/27))
  - Enable Jenkins ([ipfs/go-ipld-git#29](https://github.com/ipfs/go-ipld-git/pull/29))
- github.com/ipfs/go-ipns:
  - fix community/CONTRIBUTING.md link in README.md ([ipfs/go-ipns#20](https://github.com/ipfs/go-ipns/pull/20))
  - fix typo in README.md ([ipfs/go-ipns#21](https://github.com/ipfs/go-ipns/pull/21))
  - testing: disable inline peer ID test ([ipfs/go-ipns#19](https://github.com/ipfs/go-ipns/pull/19))
- github.com/libp2p/go-libp2p:
  - Fixed race conditions in mock package mock_stream and mock_conn ([libp2p/go-libp2p#535](https://github.com/libp2p/go-libp2p/pull/535))
  - increase initial relay advertisement delay to 30s ([libp2p/go-libp2p#534](https://github.com/libp2p/go-libp2p/pull/534))
  - Use PeerRouting in autorelay to find relay peer addresses ([libp2p/go-libp2p#531](https://github.com/libp2p/go-libp2p/pull/531))
  - docs: update broken links in NEWS.md ([libp2p/go-libp2p#517](https://github.com/libp2p/go-libp2p/pull/517))
  - don't advertise the raw public address in autorelay ([libp2p/go-libp2p#511](https://github.com/libp2p/go-libp2p/pull/511))
  - mock: export ratelimiter as RateLimiter ([libp2p/go-libp2p#507](https://github.com/libp2p/go-libp2p/pull/507))
  - readme: remove duplicate repo entries in README and package-list.json ([libp2p/go-libp2p#506](https://github.com/libp2p/go-libp2p/pull/506))
  - explicit option to enable autorelay ([libp2p/go-libp2p#500](https://github.com/libp2p/go-libp2p/pull/500))
  - Add delay in initial relay advertisement to allow the dht time to bootstrap ([libp2p/go-libp2p#495](https://github.com/libp2p/go-libp2p/pull/495))
  - suppressing error msg for NoSecurity option ([libp2p/go-libp2p#498](https://github.com/libp2p/go-libp2p/pull/498))
  - pulling updates ([libp2p/go-libp2p#4](https://github.com/libp2p/go-libp2p/pull/4))
  - fix contributing link in README ([libp2p/go-libp2p#494](https://github.com/libp2p/go-libp2p/pull/494))
  - Fix badges and links on README.md ([libp2p/go-libp2p#485](https://github.com/libp2p/go-libp2p/pull/485))
  - mocknet: fix NewStream and self dials ([libp2p/go-libp2p#480](https://github.com/libp2p/go-libp2p/pull/480))
  - deflake identify test ([libp2p/go-libp2p#479](https://github.com/libp2p/go-libp2p/pull/479))
  - mocknet: use peer ID in peer address ([libp2p/go-libp2p#476](https://github.com/libp2p/go-libp2p/pull/476))
  - autorelay ([libp2p/go-libp2p#454](https://github.com/libp2p/go-libp2p/pull/454))
  - Getting updates ([libp2p/go-libp2p#3](https://github.com/libp2p/go-libp2p/pull/3))
- github.com/libp2p/go-libp2p-autonat:
  - track autonat peer addresses ([libp2p/go-libp2p-autonat#7](https://github.com/libp2p/go-libp2p-autonat/pull/7))
- github.com/libp2p/go-libp2p-circuit:
  - Don't log raw binary ([libp2p/go-libp2p-circuit#53](https://github.com/libp2p/go-libp2p-circuit/pull/53))
- github.com/libp2p/go-libp2p-connmgr:
  - Fix concurrency and silence period not being honoured ([libp2p/go-libp2p-connmgr#26](https://github.com/libp2p/go-libp2p-connmgr/pull/26))
- github.com/libp2p/go-libp2p-crypto:
  - Fix: Remove redundant Ed25519 public key (#36). ([libp2p/go-libp2p-crypto#54](https://github.com/libp2p/go-libp2p-crypto/pull/54))
  - libp2p badges, remove IPFS ([libp2p/go-libp2p-crypto#52](https://github.com/libp2p/go-libp2p-crypto/pull/52))
  - Fix broken contribute link in README ([libp2p/go-libp2p-crypto#46](https://github.com/libp2p/go-libp2p-crypto/pull/46))
  - forbid RSA keys smaller than 512 bits ([libp2p/go-libp2p-crypto#43](https://github.com/libp2p/go-libp2p-crypto/pull/43))
  - Added ECDSA; Added RSA tests; Fixed linting errors; Handling all un-handled errors ([libp2p/go-libp2p-crypto#35](https://github.com/libp2p/go-libp2p-crypto/pull/35))
  - switch to the go-crypto ed25519 implementation ([libp2p/go-libp2p-crypto#38](https://github.com/libp2p/go-libp2p-crypto/pull/38))
  - update gogo protobuf ([libp2p/go-libp2p-crypto#37](https://github.com/libp2p/go-libp2p-crypto/pull/37))
- github.com/libp2p/go-libp2p-discovery:
  - add a timeout to Provide in routing.Advertise ([libp2p/go-libp2p-discovery#12](https://github.com/libp2p/go-libp2p-discovery/pull/12))
  - correctly encode ns to CID ([libp2p/go-libp2p-discovery#11](https://github.com/libp2p/go-libp2p-discovery/pull/11))
  - use 6hrs as ttl for routing based advertisements ([libp2p/go-libp2p-discovery#8](https://github.com/libp2p/go-libp2p-discovery/pull/8))
- github.com/libp2p/go-libp2p-host:
  - Helper to get PeerInfo from Host ([libp2p/go-libp2p-host#20](https://github.com/libp2p/go-libp2p-host/pull/20))
- github.com/libp2p/go-libp2p-kad-dht:
  - fix(dialQueue): account for failed dials ([libp2p/go-libp2p-kad-dht#277](https://github.com/libp2p/go-libp2p-kad-dht/pull/277))
  - Fix Bootstrap sub-queries ([libp2p/go-libp2p-kad-dht#264](https://github.com/libp2p/go-libp2p-kad-dht/pull/264))
  - dial queue: fix possible goroutine leak ([libp2p/go-libp2p-kad-dht#262](https://github.com/libp2p/go-libp2p-kad-dht/pull/262))
  - Alter some logging ([libp2p/go-libp2p-kad-dht#269](https://github.com/libp2p/go-libp2p-kad-dht/pull/269))
  - Revert #236: Test go mod in travis and use major versioning in import paths ([libp2p/go-libp2p-kad-dht#259](https://github.com/libp2p/go-libp2p-kad-dht/pull/259))
  - fix tests on freebsd ([libp2p/go-libp2p-kad-dht#255](https://github.com/libp2p/go-libp2p-kad-dht/pull/255))
  - Fix "no protocol with name dnsaddr" error ([libp2p/go-libp2p-kad-dht#247](https://github.com/libp2p/go-libp2p-kad-dht/pull/247))
  - Fix a race in dial queue ([libp2p/go-libp2p-kad-dht#248](https://github.com/libp2p/go-libp2p-kad-dht/pull/248))
  - Fix races with DialQueue variables ([libp2p/go-libp2p-kad-dht#241](https://github.com/libp2p/go-libp2p-kad-dht/pull/241))
  - Fix CircleCI ([libp2p/go-libp2p-kad-dht#238](https://github.com/libp2p/go-libp2p-kad-dht/pull/238))
  - Adaptive queue for staging dials ([libp2p/go-libp2p-kad-dht#237](https://github.com/libp2p/go-libp2p-kad-dht/pull/237))
  - Add the full libp2p default bootstrap peer list ([libp2p/go-libp2p-kad-dht#226](https://github.com/libp2p/go-libp2p-kad-dht/pull/226))
  - Revert "Tidy up bootstrapping" ([libp2p/go-libp2p-kad-dht#232](https://github.com/libp2p/go-libp2p-kad-dht/pull/232))
  - Tidy up bootstrapping ([libp2p/go-libp2p-kad-dht#225](https://github.com/libp2p/go-libp2p-kad-dht/pull/225))
  - Revert "Remove signal bootstrapping" ([libp2p/go-libp2p-kad-dht#227](https://github.com/libp2p/go-libp2p-kad-dht/pull/227))
  - Remove signal bootstrapping ([libp2p/go-libp2p-kad-dht#224](https://github.com/libp2p/go-libp2p-kad-dht/pull/224))
  - fix a potential DHT query hang ([libp2p/go-libp2p-kad-dht#219](https://github.com/libp2p/go-libp2p-kad-dht/pull/219))
  - docs: duplicate pkg documentation ([libp2p/go-libp2p-kad-dht#218](https://github.com/libp2p/go-libp2p-kad-dht/pull/218))
  - tests: skip key inlining test ([libp2p/go-libp2p-kad-dht#212](https://github.com/libp2p/go-libp2p-kad-dht/pull/212))
  - Rephrase "betterPeersToQuery" method comment to be less cryptic ([libp2p/go-libp2p-kad-dht#206](https://github.com/libp2p/go-libp2p-kad-dht/pull/206))
- github.com/libp2p/go-libp2p-loggables:
  - test: add unit tests ([libp2p/go-libp2p-loggables#21](https://github.com/libp2p/go-libp2p-loggables/pull/21))
- github.com/libp2p/go-libp2p-netutil:
  - Add tests ([libp2p/go-libp2p-netutil#28](https://github.com/libp2p/go-libp2p-netutil/pull/28))
- github.com/libp2p/go-libp2p-peer:
  - fix: re-enable peer ID inlining but make it configurable ([libp2p/go-libp2p-peer#42](https://github.com/libp2p/go-libp2p-peer/pull/42))
  - Protobuf and JSON (un-)marshalling methods for peer.ID ([libp2p/go-libp2p-peer#41](https://github.com/libp2p/go-libp2p-peer/pull/41))
  - disable key inlining ([libp2p/go-libp2p-peer#40](https://github.com/libp2p/go-libp2p-peer/pull/40))
- github.com/libp2p/go-libp2p-peerstore:
  - Add unit test to verify AddAddr doesn't shorten TTL ([libp2p/go-libp2p-peerstore#52](https://github.com/libp2p/go-libp2p-peerstore/pull/52))
  - disable inline-peer id test ([libp2p/go-libp2p-peerstore#49](https://github.com/libp2p/go-libp2p-peerstore/pull/49))
  - README: Update contributing guideline linkrot. ([libp2p/go-libp2p-peerstore#48](https://github.com/libp2p/go-libp2p-peerstore/pull/48))
  - Deterministic benchmark order; Keybook interface benchmarks ([libp2p/go-libp2p-peerstore#43](https://github.com/libp2p/go-libp2p-peerstore/pull/43))
  - PeerInfo UnMarshal Error #393 ([libp2p/go-libp2p-peerstore#45](https://github.com/libp2p/go-libp2p-peerstore/pull/45))
  - fix the inline key test ([libp2p/go-libp2p-peerstore#44](https://github.com/libp2p/go-libp2p-peerstore/pull/44))
- github.com/libp2p/go-libp2p-pubsub:
  - move timecache check/update after validation ([libp2p/go-libp2p-pubsub#156](https://github.com/libp2p/go-libp2p-pubsub/pull/156))
  - fix nonsensical check ([libp2p/go-libp2p-pubsub#154](https://github.com/libp2p/go-libp2p-pubsub/pull/154))
  - Extend validator interface to include message source ([libp2p/go-libp2p-pubsub#151](https://github.com/libp2p/go-libp2p-pubsub/pull/151))
  - Implement peer blacklist ([libp2p/go-libp2p-pubsub#149](https://github.com/libp2p/go-libp2p-pubsub/pull/149))
  - make timecache duration configurable ([libp2p/go-libp2p-pubsub#148](https://github.com/libp2p/go-libp2p-pubsub/pull/148))
  - godoc is not html either ([libp2p/go-libp2p-pubsub#147](https://github.com/libp2p/go-libp2p-pubsub/pull/147))
  - godoc documentation is not markdown ([libp2p/go-libp2p-pubsub#146](https://github.com/libp2p/go-libp2p-pubsub/pull/146))
  - Add documentation for subscribe's non-instanteneous semantics ([libp2p/go-libp2p-pubsub#145](https://github.com/libp2p/go-libp2p-pubsub/pull/145))
  - Some documentation ([libp2p/go-libp2p-pubsub#140](https://github.com/libp2p/go-libp2p-pubsub/pull/140))
  - rework peer tracking logic to handle multiple connections ([libp2p/go-libp2p-pubsub#132](https://github.com/libp2p/go-libp2p-pubsub/pull/132))
- github.com/libp2p/go-libp2p-pubsub-router:
  - encode record-store keys in pubsub ([libp2p/go-libp2p-pubsub-router#17](https://github.com/libp2p/go-libp2p-pubsub-router/pull/17))
- github.com/libp2p/go-libp2p-quic-transport:
  - fix badges in README ([libp2p/go-libp2p-quic-transport#39](https://github.com/libp2p/go-libp2p-quic-transport/pull/39))
  - Fix missing transport parameter in dialed connection ([libp2p/go-libp2p-quic-transport#38](https://github.com/libp2p/go-libp2p-quic-transport/pull/38))
- github.com/libp2p/go-libp2p-routing:
  - Update the comment on IpfsRouting.Bootstrap ([libp2p/go-libp2p-routing#36](https://github.com/libp2p/go-libp2p-routing/pull/36))
- github.com/libp2p/go-libp2p-swarm:
  - Make FD limits configurable by environment property ([libp2p/go-libp2p-swarm#102](https://github.com/libp2p/go-libp2p-swarm/pull/102))
  - Fix logging race ([libp2p/go-libp2p-swarm#100](https://github.com/libp2p/go-libp2p-swarm/pull/100))
  - Add CircleCI config ([libp2p/go-libp2p-swarm#99](https://github.com/libp2p/go-libp2p-swarm/pull/99))
  - Enhance debug logging in dial limiter ([libp2p/go-libp2p-swarm#98](https://github.com/libp2p/go-libp2p-swarm/pull/98))
  - dialer: handle dial cancel and/or completion before trying new addresses ([libp2p/go-libp2p-swarm#96](https://github.com/libp2p/go-libp2p-swarm/pull/96))
  - avoid spawning goroutines for canceled dials ([libp2p/go-libp2p-swarm#95](https://github.com/libp2p/go-libp2p-swarm/pull/95))
  - warn when we encounter a useless transport ([libp2p/go-libp2p-swarm#90](https://github.com/libp2p/go-libp2p-swarm/pull/90))
- github.com/libp2p/go-libp2p-transport:
  - fix transport tests for quic ([libp2p/go-libp2p-transport#39](https://github.com/libp2p/go-libp2p-transport/pull/39))
  - fix: fully close streams before returning ([libp2p/go-libp2p-transport#37](https://github.com/libp2p/go-libp2p-transport/pull/37))
  - fix typo in README ([libp2p/go-libp2p-transport#36](https://github.com/libp2p/go-libp2p-transport/pull/36))
- github.com/libp2p/go-libp2p-transport-upgrader:
  - annotate errors ([libp2p/go-libp2p-transport-upgrader#11](https://github.com/libp2p/go-libp2p-transport-upgrader/pull/11))
- github.com/ipfs/go-log:
  - uglify the (event) logs ([ipfs/go-log#53](https://github.com/ipfs/go-log/pull/53))
  - add environment variable for writing tracing information to a file ([ipfs/go-log#52](https://github.com/ipfs/go-log/pull/52))
  - correctly display the line number when FinishWithErr fails ([ipfs/go-log#51](https://github.com/ipfs/go-log/pull/51))
- github.com/libp2p/go-maddr-filter:
  - test: extend test to improve coverage ([libp2p/go-maddr-filter#7](https://github.com/libp2p/go-maddr-filter/pull/7))
- github.com/ipfs/go-merkledag:
  - Increase FetchGraphConcurrency to 32 ([ipfs/go-merkledag#29](https://github.com/ipfs/go-merkledag/pull/29))
  - Enable CI ([ipfs/go-merkledag#9](https://github.com/ipfs/go-merkledag/pull/9))
  - fix a fetch deadlock on error ([ipfs/go-merkledag#21](https://github.com/ipfs/go-merkledag/pull/21))
  - Wait for all go routines to finish before function returns ([ipfs/go-merkledag#19](https://github.com/ipfs/go-merkledag/pull/19))
- github.com/ipfs/go-metrics-prometheus:
  - use prometheus instead of gxed ([ipfs/go-metrics-prometheus#3](https://github.com/ipfs/go-metrics-prometheus/pull/3))
- github.com/ipfs/go-mfs:
  - fix(mv): dst filename error ([ipfs/go-mfs#62](https://github.com/ipfs/go-mfs/pull/62))
  - fix over-wait in WaitPub ([ipfs/go-mfs#53](https://github.com/ipfs/go-mfs/pull/53))
  - Fix/32/pr ports from go-ipfs to go-mfs ([ipfs/go-mfs#49](https://github.com/ipfs/go-mfs/pull/49))
  - remove the `fullSync` option from `updateChildEntry` ([ipfs/go-mfs#45](https://github.com/ipfs/go-mfs/pull/45))
  - Various refactorings ([ipfs/go-mfs#36](https://github.com/ipfs/go-mfs/pull/36))
  - use RW lock for the `File`'s lock ([ipfs/go-mfs#43](https://github.com/ipfs/go-mfs/pull/43))
  - add documentation links in README ([ipfs/go-mfs#41](https://github.com/ipfs/go-mfs/pull/41))
  - [WIP] documentation notes ([ipfs/go-mfs#27](https://github.com/ipfs/go-mfs/pull/27))
  - feat(inode): add inode struct ([ipfs/go-mfs#12](https://github.com/ipfs/go-mfs/pull/12))
- github.com/libp2p/go-mplex:
  - fix deadlock ([libp2p/go-mplex#39](https://github.com/libp2p/go-mplex/pull/39))
  - When a stream is closed, cancel pending writes ([libp2p/go-mplex#35](https://github.com/libp2p/go-mplex/pull/35))
  - make sure to but the buffer back in the pool ([libp2p/go-mplex#34](https://github.com/libp2p/go-mplex/pull/34))
  - reduce the packet count ([libp2p/go-mplex#29](https://github.com/libp2p/go-mplex/pull/29))
- github.com/ipfs/go-path:
  - fix: no components error ([ipfs/go-path#18](https://github.com/ipfs/go-path/pull/18))
  - nit: validate CIDs in IPLD paths ([ipfs/go-path#16](https://github.com/ipfs/go-path/pull/16))
- github.com/libp2p/go-reuseport:
  - Fix build on wasm ([libp2p/go-reuseport#59](https://github.com/libp2p/go-reuseport/pull/59))
  - Use Go Control API ([libp2p/go-reuseport#56](https://github.com/libp2p/go-reuseport/pull/56))
  - Support WASM ([libp2p/go-reuseport#54](https://github.com/libp2p/go-reuseport/pull/54))
- github.com/libp2p/go-reuseport-transport:
  - Update to go-reuseport 0.2.0 ([libp2p/go-reuseport-transport#6](https://github.com/libp2p/go-reuseport-transport/pull/6))
- github.com/libp2p/go-stream-muxer:
  - add standard reset error ([libp2p/go-stream-muxer#23](https://github.com/libp2p/go-stream-muxer/pull/23))
  - ci: fix ([libp2p/go-stream-muxer#24](https://github.com/libp2p/go-stream-muxer/pull/24))
  - Document Reset versus Close ([libp2p/go-stream-muxer#18](https://github.com/libp2p/go-stream-muxer/pull/18))
  - WIP document Conn.Close ([libp2p/go-stream-muxer#19](https://github.com/libp2p/go-stream-muxer/pull/19))
- github.com/libp2p/go-tcp-transport:
  - Deprecate IPFS_REUSEPORT, use LIBP2P_TCP_REUSEPORT ([libp2p/go-tcp-transport#27](https://github.com/libp2p/go-tcp-transport/pull/27))
- github.com/ipfs/go-unixfs:
  - unixfile: precalc dir size ([ipfs/go-unixfs#61](https://github.com/ipfs/go-unixfs/pull/61))
  - Archive refactor ([ipfs/go-unixfs#59](https://github.com/ipfs/go-unixfs/pull/59))
  - decouple the DAG traversal logic from the DAG reader (local branch) ([ipfs/go-unixfs#60](https://github.com/ipfs/go-unixfs/pull/60))
  - Unixfs: enforce refs on files when using nocopy ([ipfs/go-unixfs#56](https://github.com/ipfs/go-unixfs/pull/56))
  - Fix/handle overflow ([ipfs/go-unixfs#53](https://github.com/ipfs/go-unixfs/pull/53))
  - feat(Directory): Add EnumLinksAsync method ([ipfs/go-unixfs#39](https://github.com/ipfs/go-unixfs/pull/39))



## 0.4.18 2018-10-26

This is probably one of the largest go-ipfs releases in recent history, 3 months
in the making.

### Features

The headline features this release are experimental QUIC support, the gossipsub
pubsub routing algorithm, pubsub message signing, and a refactored `ipfs p2p`
command. However, that's just scratching the surface.

#### QUIC

First up, on the networking front, this release has also introduced experimental
support for the QUIC protocol. QUIC is a new UDP-based network transport that
solves many of the long standing issues with TCP.

For us, this means (eventually):

* **Fewer local resources.** TCP requires a file-descriptor per connection while
  QUIC (and most UDP based transports) can share a single file descriptor
  between all connections. This should allow us to dial faster and keep more
  connections open.
* **Faster connection establishment.** When client authentication is included,
  QUIC has a three-way handshake like TCP. However, unlike TCP, this handshake
  brings us from all the way from 0 to a fully encrypted, authenticated, and
  multiplexed connection. In theory (not yet in practice), this should
  significantly reduce the latency of DHT queries.
* **Behaves better on lossy networks.** When multiplexing multiple requests over
  a single TCP connection, a single dropped packet will bring the entire
  connection to a halt while the packet is re-transmitted. However, because QUIC
  handles multiplexing internally, dropping a single packets affects only the
  related stream.
* **Better NAT traversal.** TL;DR: NAT hole-punching is significantly easier
  and, in many cases, more reliable with UDP than with TCP.

However, we still have a long way to go. While we encourage users to test this,
the IETF QUIC protocol is still being actively developed and *will* change. You
can find instructions for enabling it
[here](https://github.com/ipfs/go-ipfs/blob/master/docs/experimental-features.md#QUIC).

#### Pubsub

In terms of pubsub, go-ipfs now supports the gossipsub routing algorithm and
message signing.

The gossipsub routing algorithm is *significantly* more efficient than the
current floodsub routing algorithm. Even better, it's fully backwards compatible
so you can enable it and still talk to nodes using the floodsub algorithm. You
can find instructions to enable gossipsub in go-ipfs
[here](https://github.com/ipfs/go-ipfs/blob/master/docs/experimental-features.md#gossipsub).

Messages are now signed by their authors. While signing has now been enabled by
default, strict signature verification has not been and will not be for at least
one release (probably multiple) to avoid breaking existing applications. You can
read about how to configure this feature
[here](https://github.com/ipfs/go-ipfs/blob/master/docs/experimental-features.md#message-signing).

#### Commands

In terms of new toys, this release introduces a new `ipfs cid` subcommand for
working with CIDs, a completely refactored `ipfs p2p` command, streaming name
resolution, and complete inline block support.

The new `ipfs cid` command allows users to both inspect CIDs and convert them
between various formats and versions. For example:

```sh
# Print out the CID metadata (prefix)
> ipfs cid format -f %P QmT78zSuBmuS4z925WZfrqQ1qHaJ56DQaTfyMUF7F8ff5o
cidv0-protobuf-sha2-256-32

# Get the hex sha256 hash from the CID.
> ipfs cid format -b base16 -f '0x%D' QmT78zSuBmuS4z925WZfrqQ1qHaJ56DQaTfyMUF7F8ff5o
0x46d44814b9c5af141c3aaab7c05dc5e844ead5f91f12858b021eba45768b4c0e

# Convert a base58 v0 CID to a base32 v1 CID.
> ipfs cid base32 QmT78zSuBmuS4z925WZfrqQ1qHaJ56DQaTfyMUF7F8ff5o
bafybeicg2rebjoofv4kbyovkw7af3rpiitvnl6i7ckcywaq6xjcxnc2mby
```

The refactored `ipfs p2p` command allows forwarding TCP streams through two IPFS
nodes from one host to another. It's `ssh -L` but for IPFS. You can find
documentation 
[here](https://github.com/ipfs/go-ipfs/blob/master/docs/experimental-features.md#ipfs-p2p).
It's still experimental but we don't expect too many breaking changes at this
point (it will very likely be stabilized in the next release). Quick summary of
breaking changes:

* We don't stop listening for local (forwarded) connections after accepting a
  single connection.
* `ipfs p2p stream ls` output now returns more useful output, first address is
  always the initiator address.
* `ipfs p2p listener ls` is renamed to `ipfs p2p ls`
* `ipfs p2p listener close` is renamed to `ipfs p2p close`
* Protocol names have to be prefixed with `/x/` and are now just passed to
  libp2p as handler name. Previous version did this 'under the hood' and with
  `/p2p/` prefix. There is a `--allow-custom-protocol` flag which allows you
  to use any libp2p handler name.
* `ipfs p2p listener open` and `ipfs p2p stream dial` got renamed:
    * `ipfs p2p listener open p2p-test /ip4/127.0.0.1/tcp/10101`
      new becomes `ipfs p2p listen /x/p2p-test /ip4/127.0.0.1/tcp/10101`
    * `ipfs p2p stream dial $NODE_A_PEERID p2p-test /ip4/127.0.0.1/tcp/10102`
      is now `ipfs p2p forward /x/p2p-test /ip4/127.0.0.1/tcp/10102 /ipfs/$NODE_A_PEERID`

There is now a new flag for `ipfs name resolve` - `--stream`. When the command
is invoked with the flag set, it will start returning results as soon as they
are discovered in the DHT and other routing mechanisms. This enables certain
applications to start prefetching/displaying data while the discovery is still
running. Note that this command will likely return many outdated records
before it finding and returning the latest. However, it will always return
*valid* records (even if a bit stale).

Finally, in the previous release, we added support for extracting blocks inlined
into CIDs. In this release, we've added support for creating these CIDs. You can
now run `ipfs add` with the `--inline` flag to inline blocks less than or equal
to 32 bytes in length into a CID, instead of writing an actual block. This
should significantly reduce the size of filesystem trees with many empty
directories and tiny files.

#### IPNS

You can now publish and resolve paths with namespaces *other* than `/ipns` and
`/ipfs` through IPNS. Critically, IPNS can now be used with IPLD paths (paths
starting with `/ipld`).

#### WebUI

Finally, this release includes the shiny [updated
webui](https://github.com/ipfs-shipyard/ipfs-webui). You can view it by
installing go-ipfs and visiting http://localhost:5001/webui.

### Performance

This release includes some significant performance improvements, both in terms
of resource utilization and speed. This section will go into some technical
details so feel free to skip it if you're just looking for shiny new features.

#### Resource Utilization

In this release, we've (a) fixed a slow memory leak in libp2p and (b)
significantly reduced the allocation load. Together, these should improve both
memory and CPU usage.

##### Datastructures

We've changed two of our most frequently used datastructures, CIDs and
Multiaddrs, to reduce allocation load.

First, we now store CIDs *encode* as strings, instead of decoded in structs
(behind pointers). In addition to being more compact, our `Cid` type is now a
valid `map` key so we no longer have to encode CIDs every time we want to use
them in a map/set. Allocations when inserting CIDs into maps/sets was showing up
as a significant source of allocations under heavy load so this change should
improve memory usage.

Second, we've changed many of our multiaddr parsing/processing/formatting
functions to allocate less. Much of our DHT related-work includes processing
multiaddrs so this should reduce CPU utilization when heavily using the DHT.

##### Streams and Yamux

Streams have always plagued us in terms of memory utilization. This was
partially solved by introducing the connection manager, keeping our maximum
connection count to a reasonable number but they're still a major memory sink.

This release sees two improvements on this front:

1. A memory [leak in identify](https://github.com/libp2p/go-libp2p/issues/419)
   has been fixed. This was slowly causing us to leak connections (locking up
   the memory used by the connections' streams).
2. Yamux streams now use a buffer-pool backed, auto shrinking read buffer.
   Before, this read buffer would grow to its maximum size (a few megabytes) and
   never shrink but these buffers now shrink as they're emptied.

#### Bitswap Performance

Bitswap will now pack *multiple* small blocks into a single message thanks
[ipfs/go-bitswap#5](https://github.com/ipfs/go-bitswap/pull/5). While this won't
help when transferring large files (with large blocks), this should help when
transferring many tiny files.

### Refactors and Endeavors

This release saw yet another commands-library refactor, work towards the
CoreAPI, and the first step towards reliable base32 CID support.

#### Commands Lib

We've completely refactored our commands library (again). While it still needs
quite a bit of work, it now requires significantly less boilerplate and should
be significantly more robust. The refactor immediately found two broken tests
and probably fixed quite a few bugs around properly returning and handling
errors.

#### CoreAPI

CoreAPI is a new way to interact with IPFS from Go. While it's still not
final, most things you can do via the CLI or HTTP interfaces, can now be done
through the new API.

Currently there is only one implementation, backed by go-ipfs node, and there are
plans to start http-api backed one soon. We are also looking into creating RPC
interface using this API, which could help performance in some use cases.

You can track progress in https://github.com/ipfs/go-ipfs/issues/4498

#### IPLD paths

We introduced new path type which introduces distinction between IPLD and
IPFS (unixfs) paths. From now on paths prefixed with `/ipld/` will always
use IPLD link traversal and `/ipfs/` will use unixfs path resolver, which
takes things like shardnig into account.

Note that this is only initial support and there likely are some bugs in
how the paths are handled internally, so consider this feature
experimental for now.

#### CIDv1/Base32 Migration

Currently, IPFS is usually used in browsers by browsing to
`https://SOME_GATEWAY/ipfs/CID/...`. There are two significant drawbacks to this
approach:

1. From a browser security standpoint, all IPFS "sites" will live under the same
   origin (SOME_GATEWAY).
2. From a UX standpoint, this doesn't feel very "native" (even if the gateway is
   a local IPFS node).

To fix the security issue, we intend to switch IPFS gateway links
`https://ipfs.io/ipfs/CID` to to `https://CID.ipfs.dweb.link`. This way, the CID
will be a part of the
["origin"](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Origin) so
each IPFS website will get a separate security origin.

To fix the UX issue, we've been working on adding support for `ipfs://CID/...`
to web browsers through our
[ipfs-companion](https://github.com/ipfs/ipfs-companion/) add-on and some new,
experimental extension APIs from Mozilla. This has the same effect of putting
the CID in the URL origin but has the added benefit of looking "native".

Unfortunately, origins must be *case insensitive*. Currently, most CIDs users
see are *CIDv0* CIDs (those starting with `Qm`) which are *always* base58
encoded and are therefore case-sensitive.

Fortunately, CIDv1 (the latest CID format) supports arbitrary bases using the
[multibase](https://github.com/multiformats/multibase/) standard. Unfortunately,
IPFS has always treated equivalent CIDv0 and CIDv1 CIDs as distinct. This means
that files added with CIDv0 CIDs (the default) can't be looked up using the
equivalent CIDv1.

This release makes some significant progress towards solving this issue by
introducing two features:

(1) The previous mentioned `ipfs cid base32` command for converting CID to a
case intensive encoding required by domain names. This command converts a CID to
version 1 and encodes it using base32.

(2) A hack to allow locally looking up blocks associated with a CIDv0 CID using
the equivalent CIDv1 CID (or the reverse). This hack will eventually
be replaced with a multihash indexed blockstore, which is agnostic to both the
CID version and multicodec content type.

### go-ipfs changelog

Features (i.e., users take heed):
  - gossipsub ([ipfs/go-ipfs#5373](https://github.com/ipfs/go-ipfs/pull/5373))
  - support /ipfs/CID in `ipfs dht findprovs` ([ipfs/go-ipfs#5329](https://github.com/ipfs/go-ipfs/pull/5329))
  - return a json object from config show ([ipfs/go-ipfs#5345](https://github.com/ipfs/go-ipfs/pull/5345))
  - Set filename in Content-Disposition if filename=x is passed in URI query ([ipfs/go-ipfs#4177](https://github.com/ipfs/go-ipfs/pull/4177))
  - Allow mfs files.write command to create parent directories ([ipfs/go-ipfs#5359](https://github.com/ipfs/go-ipfs/pull/5359))
  - Run DNS lookup for --api endpoint provided in CLI ([ipfs/go-ipfs#5372](https://github.com/ipfs/go-ipfs/pull/5372))
  - Add support for inlinling blocks into CIDs the id-hash ([ipfs/go-ipfs#5281](https://github.com/ipfs/go-ipfs/pull/5281))
  - depth limited refs -r ([ipfs/go-ipfs#5337](https://github.com/ipfs/go-ipfs/pull/5337))
  - remove bitswap unwant ([ipfs/go-ipfs#5308](https://github.com/ipfs/go-ipfs/pull/5308))
  - add experimental QUIC support ([ipfs/go-ipfs#5350](https://github.com/ipfs/go-ipfs/pull/5350))
  - add a --stdin-name flag for naming files from stdin ([ipfs/go-ipfs#5399](https://github.com/ipfs/go-ipfs/pull/5399))
  - Refactor `ipfs p2p` ([ipfs/go-ipfs#4929](https://github.com/ipfs/go-ipfs/pull/4929))
  - add dns support in`ipfs p2p forward` and refactor code ([ipfs/go-ipfs#5533](https://github.com/ipfs/go-ipfs/pull/5533))
  - feat(command): expose connection direction ([ipfs/go-ipfs#5457](https://github.com/ipfs/go-ipfs/pull/5457))
  - error when publishing ipns records without a running daemon ([ipfs/go-ipfs#5477](https://github.com/ipfs/go-ipfs/pull/5477))
  - feat(daemon): print version on start ([ipfs/go-ipfs#5503](https://github.com/ipfs/go-ipfs/pull/5503))
  - add quieter option to name publish ([ipfs/go-ipfs#5494](https://github.com/ipfs/go-ipfs/pull/5494))
  - Provide new "cid" sub-command. ([ipfs/go-ipfs#5385](https://github.com/ipfs/go-ipfs/pull/5385))
  - feat(command): add force flag for files rm ([ipfs/go-ipfs#5555](https://github.com/ipfs/go-ipfs/pull/5555))
  - Add support for datastore plugins ([ipfs/go-ipfs#5187](https://github.com/ipfs/go-ipfs/pull/5187))
  - files ls: append slash to directory names ([ipfs/go-ipfs#5605](https://github.com/ipfs/go-ipfs/pull/5605))
  - ipfs name resolve --stream ([ipfs/go-ipfs#5404](https://github.com/ipfs/go-ipfs/pull/5404))
  - update webui to 2.1.0 ([ipfs/go-ipfs#5627](https://github.com/ipfs/go-ipfs/pull/5627))
  - feat: add dry-run flag for config profile apply command ([ipfs/go-ipfs#5455](https://github.com/ipfs/go-ipfs/pull/5455))
  - configurable pubsub signing ([ipfs/go-ipfs#5647](https://github.com/ipfs/go-ipfs/pull/5647))

Fixes (i.e., users take note):
  - pin update fixes ([ipfs/go-ipfs#5265](https://github.com/ipfs/go-ipfs/pull/5265))
  - Fix inability to pin two things at once ([ipfs/go-ipfs#5512](https://github.com/ipfs/go-ipfs/pull/5512))
  - wait for all connections to close before exiting on shutdown. ([ipfs/go-ipfs#5322](https://github.com/ipfs/go-ipfs/pull/5322))
  - Fixed ipns address resolution in fuse unix mount ([ipfs/go-ipfs#5384](https://github.com/ipfs/go-ipfs/pull/5384))
  - core/commands/ls: wrap `NewDirectoryFromNode` error ([ipfs/go-ipfs#5166](https://github.com/ipfs/go-ipfs/pull/5166))
  - fix goroutine leaks in filestore.go ([ipfs/go-ipfs#5427](https://github.com/ipfs/go-ipfs/pull/5427))
  - move VersionOption after GatewayOption to fix #5422 ([ipfs/go-ipfs#5424](https://github.com/ipfs/go-ipfs/pull/5424))
  - fix(commands): fix filestore.go goroutine leak ([ipfs/go-ipfs#5439](https://github.com/ipfs/go-ipfs/pull/5439))
  - fix(commands): goroutine leaks in ping.go ([ipfs/go-ipfs#5444](https://github.com/ipfs/go-ipfs/pull/5444))
  - fix output of object command ([ipfs/go-ipfs#5459](https://github.com/ipfs/go-ipfs/pull/5459))
  - add warning when no bootstrap in config ([ipfs/go-ipfs#5445](https://github.com/ipfs/go-ipfs/pull/5445))
  - fix behaviour of key rename to same name ([ipfs/go-ipfs#5465](https://github.com/ipfs/go-ipfs/pull/5465))
  - fix(object): print object diff error ([ipfs/go-ipfs#5469](https://github.com/ipfs/go-ipfs/pull/5469))
  - fix(pin): goroutine leaks ([ipfs/go-ipfs#5453](https://github.com/ipfs/go-ipfs/pull/5453))
  - fix offline id bug ([ipfs/go-ipfs#5486](https://github.com/ipfs/go-ipfs/pull/5486))
  - files cp: improve flush error message ([ipfs/go-ipfs#5485](https://github.com/ipfs/go-ipfs/pull/5485))
  - resolve: fix unixfs resolution through sharded directories ([ipfs/go-ipfs#5484](https://github.com/ipfs/go-ipfs/pull/5484))
  - Switch name publish/resolve to coreapi ([ipfs/go-ipfs#5563](https://github.com/ipfs/go-ipfs/pull/5563))
  - use CoreAPI resolver everywhere (fixes sharded directory resolution) ([ipfs/go-ipfs#5492](https://github.com/ipfs/go-ipfs/pull/5492))
  - add pin lock in AddallPin function ([ipfs/go-ipfs#5506](https://github.com/ipfs/go-ipfs/pull/5506))
  - take the pinlock when updating pins ([ipfs/go-ipfs#5550](https://github.com/ipfs/go-ipfs/pull/5550))
  - fix(object): add support for raw leaves in object diff ([ipfs/go-ipfs#5472](https://github.com/ipfs/go-ipfs/pull/5472))
  - don't use the domain name as a filename in /ipns/a.com ([ipfs/go-ipfs#5564](https://github.com/ipfs/go-ipfs/pull/5564))
  - refactor(command): modify int to int64 ([ipfs/go-ipfs#5612](https://github.com/ipfs/go-ipfs/pull/5612))
  - fix(core): ipns config RecordLifetime panic ([ipfs/go-ipfs#5648](https://github.com/ipfs/go-ipfs/pull/5648))
  - simplify dag put and correctly take pin lock ([ipfs/go-ipfs#5667](https://github.com/ipfs/go-ipfs/pull/5667))
  - fix prometheus concurrent map write bug ([ipfs/go-ipfs#5706](https://github.com/ipfs/go-ipfs/pull/5706))

Regressions Fixes (fixes for bugs introduced since the last release):
  - namesys: properly attach path in name.Resolve ([ipfs/go-ipfs#5660](https://github.com/ipfs/go-ipfs/pull/5660))
  - fix(p2p): issue #5523 ([ipfs/go-ipfs#5529](https://github.com/ipfs/go-ipfs/pull/5529))
  - fix infinite loop in `stats bw` ([ipfs/go-ipfs#5598](https://github.com/ipfs/go-ipfs/pull/5598))
  - make warnings on no bootstrap peers less noisy ([ipfs/go-ipfs#5466](https://github.com/ipfs/go-ipfs/pull/5466))
  - fix two transport related bugs ([ipfs/go-ipfs#5417](https://github.com/ipfs/go-ipfs/pull/5417))
  - Fix pin ls output when hash is specified ([ipfs/go-ipfs#5699](https://github.com/ipfs/go-ipfs/pull/5699))
  - ping: switch to the ping service enabled in the libp2p constructor ([ipfs/go-ipfs#5698](https://github.com/ipfs/go-ipfs/pull/5698))
  - commands: fix a bunch of tiny commands-lib issues ([ipfs/go-ipfs#5697](https://github.com/ipfs/go-ipfs/pull/5697))
  - cleanup the ping command ([ipfs/go-ipfs#5680](https://github.com/ipfs/go-ipfs/pull/5680))
  - fix gossipsub goroutine explosion ([ipfs/go-ipfs#5688](https://github.com/ipfs/go-ipfs/pull/5688))
  - fix(cmd/gc): Run func does not return error when Emit func returns error ([ipfs/go-ipfs#5687](https://github.com/ipfs/go-ipfs/pull/5687))

Extractions:
  - Extract bitswap to go-bitswap ([ipfs/go-ipfs#5294](https://github.com/ipfs/go-ipfs/pull/5294))
  - Extract blockservice and verifcid ([ipfs/go-ipfs#5296](https://github.com/ipfs/go-ipfs/pull/5296))
  - Extract merkledag package, move dagutils to top level ([ipfs/go-ipfs#5298](https://github.com/ipfs/go-ipfs/pull/5298))
  - Extract path and resolver ([ipfs/go-ipfs#5306](https://github.com/ipfs/go-ipfs/pull/5306))
  - Extract config package ([ipfs/go-ipfs#5277](https://github.com/ipfs/go-ipfs/pull/5277))
  - Extract unixfs and importers to go-unixfs ([ipfs/go-ipfs#5316](https://github.com/ipfs/go-ipfs/pull/5316))
  - delete unixfs code... ([ipfs/go-ipfs#5319](https://github.com/ipfs/go-ipfs/pull/5319))
  - Extract /mfs to github.com/ipfs/go-mfs ([ipfs/go-ipfs#5391](https://github.com/ipfs/go-ipfs/pull/5391))
  - re-format log output as ndjson ([ipfs/go-ipfs#5708](https://github.com/ipfs/go-ipfs/pull/5708))
  - error on resolving non-terminal paths ([ipfs/go-ipfs#5705](https://github.com/ipfs/go-ipfs/pull/5705))

Documentation:
  - document the fact that we now publish releases on GitHub ([ipfs/go-ipfs#5301](https://github.com/ipfs/go-ipfs/pull/5301))
  - docs: add url to dev weekly sync to the README ([ipfs/go-ipfs#5371](https://github.com/ipfs/go-ipfs/pull/5371))
  - docs: README refresh, add cli-http-api-core diagram ([ipfs/go-ipfs#5396](https://github.com/ipfs/go-ipfs/pull/5396))
  - add some basic gateway documentation ([ipfs/go-ipfs#5393](https://github.com/ipfs/go-ipfs/pull/5393))
  - fix the default gateway port ([ipfs/go-ipfs#5419](https://github.com/ipfs/go-ipfs/pull/5419))
  - fix order of events in the release process ([ipfs/go-ipfs#5434](https://github.com/ipfs/go-ipfs/pull/5434))
  - docs: add some minimal read-only API documentation ([ipfs/go-ipfs#5437](https://github.com/ipfs/go-ipfs/pull/5437))
  - feat: use package-table ([ipfs/go-ipfs#5395](https://github.com/ipfs/go-ipfs/pull/5395))
  - link to go-{libp2p,ipld} package tables ([ipfs/go-ipfs#5446](https://github.com/ipfs/go-ipfs/pull/5446))
  - api: fix outdated HTTPHeaders config documentation ([ipfs/go-ipfs#5451](https://github.com/ipfs/go-ipfs/pull/5451))
  - add version, usage, and planning info for urlstore ([ipfs/go-ipfs#5552](https://github.com/ipfs/go-ipfs/pull/5552))
  - debug-guide.md added memory statistics command ([ipfs/go-ipfs#5546](https://github.com/ipfs/go-ipfs/pull/5546))
  - Change to point to combined go contributing guidelines ([ipfs/go-ipfs#5607](https://github.com/ipfs/go-ipfs/pull/5607))
  - docs: Update link format ([ipfs/go-ipfs#5617](https://github.com/ipfs/go-ipfs/pull/5617))
  - Fix link in readme ([ipfs/go-ipfs#5632](https://github.com/ipfs/go-ipfs/pull/5632))
  - docs: add a note for dns command ([ipfs/go-ipfs#5629](https://github.com/ipfs/go-ipfs/pull/5629))
  - Dockerfile: Specifies comments on exposed ports ([ipfs/go-ipfs#5615](https://github.com/ipfs/go-ipfs/pull/5615))
  - document pubsub message signing ([ipfs/go-ipfs#5669](https://github.com/ipfs/go-ipfs/pull/5669))

Testing:
  - Include cid-fmt binary in test/bin. ([ipfs/go-ipfs#5297](https://github.com/ipfs/go-ipfs/pull/5297))
  - wait for the nodes to fully stop ([ipfs/go-ipfs#5315](https://github.com/ipfs/go-ipfs/pull/5315))
  - apply timeout for build steps after getting node ([ipfs/go-ipfs#5313](https://github.com/ipfs/go-ipfs/pull/5313))
  - ci: check for gx deps dupes ([ipfs/go-ipfs#5338](https://github.com/ipfs/go-ipfs/pull/5338))
  - ci: call cleanWs after each step ([ipfs/go-ipfs#5374](https://github.com/ipfs/go-ipfs/pull/5374))
  - add correct test for GC completeness ([ipfs/go-ipfs#5364](https://github.com/ipfs/go-ipfs/pull/5364))
  - fix the urlstore tests ([ipfs/go-ipfs#5397](https://github.com/ipfs/go-ipfs/pull/5397))
  - improve gateway options test ([ipfs/go-ipfs#5433](https://github.com/ipfs/go-ipfs/pull/5433))
  - coreapi name: Increase test swarm size ([ipfs/go-ipfs#5481](https://github.com/ipfs/go-ipfs/pull/5481))
  - fix fuse unmount test ([ipfs/go-ipfs#5476](https://github.com/ipfs/go-ipfs/pull/5476))
  - test(add): add test for issue \#5456 ([ipfs/go-ipfs#5493](https://github.com/ipfs/go-ipfs/pull/5493))
  - fixed tests of raised fd limits ([ipfs/go-ipfs#5496](https://github.com/ipfs/go-ipfs/pull/5496))
  - pprof: create HTTP endpoint for setting MutexProfileFraction ([ipfs/go-ipfs#5527](https://github.com/ipfs/go-ipfs/pull/5527))
  - fix(command):update `add --chunker` test ([ipfs/go-ipfs#5571](https://github.com/ipfs/go-ipfs/pull/5571))
  - switch to go 1.11 ([ipfs/go-ipfs#5483](https://github.com/ipfs/go-ipfs/pull/5483))
  - fix: sharness race in directory_size if file is removed ([ipfs/go-ipfs#5586](https://github.com/ipfs/go-ipfs/pull/5586))
  - Bump Go versions and use '.x' to always get latest minor versions ([ipfs/go-ipfs#5682](https://github.com/ipfs/go-ipfs/pull/5682))
  - add rabin min error test ([ipfs/go-ipfs#5449](https://github.com/ipfs/go-ipfs/pull/5449))
  - Use CircleCI 2.0 ([ipfs/go-ipfs#5691](https://github.com/ipfs/go-ipfs/pull/5691))

Internal:
  - Add ability to retrieve blocks even if given using a different CID version ([ipfs/go-ipfs#5285](https://github.com/ipfs/go-ipfs/pull/5285))
  - update gogo-protobuf ([ipfs/go-ipfs#5355](https://github.com/ipfs/go-ipfs/pull/5355))
  - update protobuf files in go-ipfs ([ipfs/go-ipfs#5356](https://github.com/ipfs/go-ipfs/pull/5356))
  - string-backed CIDs ([ipfs/go-ipfs#5441](https://github.com/ipfs/go-ipfs/pull/5441))
  - commands: switch object to CoreAPI ([ipfs/go-ipfs#4643](https://github.com/ipfs/go-ipfs/pull/4643))
  - coreapi: dag: Batching interface ([ipfs/go-ipfs#5340](https://github.com/ipfs/go-ipfs/pull/5340))
  - key cmd: Refactor to use coreapi ([ipfs/go-ipfs#5339](https://github.com/ipfs/go-ipfs/pull/5339))
  - coreapi: DHT API ([ipfs/go-ipfs#4804](https://github.com/ipfs/go-ipfs/pull/4804))
  - block cmd: Use coreapi ([ipfs/go-ipfs#5331](https://github.com/ipfs/go-ipfs/pull/5331))
  - mk: embed CurrentCommit in the right place ([ipfs/go-ipfs#5507](https://github.com/ipfs/go-ipfs/pull/5507))
  - added binary executable files to .dockerignore ([ipfs/go-ipfs#5544](https://github.com/ipfs/go-ipfs/pull/5544))
  - Add sessions when fetching MerkleDAG in LS ([ipfs/go-ipfs#5509](https://github.com/ipfs/go-ipfs/pull/5509))
  - coreapi: Swarm API ([ipfs/go-ipfs#4803](https://github.com/ipfs/go-ipfs/pull/4803))
  - coreapi swarm: unify impl type with other apis ([ipfs/go-ipfs#5551](https://github.com/ipfs/go-ipfs/pull/5551))
  - Refactor UnixFS CoreAPI ([ipfs/go-ipfs#5501](https://github.com/ipfs/go-ipfs/pull/5501))
  - coreapi: PubSub API ([ipfs/go-ipfs#4805](https://github.com/ipfs/go-ipfs/pull/4805))
  - fix: maketarball.sh for OSX ([ipfs/go-ipfs#5575](https://github.com/ipfs/go-ipfs/pull/5575))
  - test the correct return value when checking directory size ([ipfs/go-ipfs#5580](https://github.com/ipfs/go-ipfs/pull/5580))
  - coreapi unixfs: remove Cat ([ipfs/go-ipfs#5574](https://github.com/ipfs/go-ipfs/pull/5574))
  - Explicitally use BufferedDAG after removing Batch from importers ([ipfs/go-ipfs#5626](https://github.com/ipfs/go-ipfs/pull/5626))

Cleanup:
  - Fix some weird code in core/coreunix/add.go ([ipfs/go-ipfs#5354](https://github.com/ipfs/go-ipfs/pull/5354))
  - name cmd: move subcommands to subdirectory ([ipfs/go-ipfs#5392](https://github.com/ipfs/go-ipfs/pull/5392))
  - directly parse peer IDs as peer IDs ([ipfs/go-ipfs#5409](https://github.com/ipfs/go-ipfs/pull/5409))
  - don't bother caching if we're using a nil repo ([ipfs/go-ipfs#5414](https://github.com/ipfs/go-ipfs/pull/5414))
  - object:refactor data encode error ([ipfs/go-ipfs#5426](https://github.com/ipfs/go-ipfs/pull/5426))
  - remove Godeps ([ipfs/go-ipfs#5440](https://github.com/ipfs/go-ipfs/pull/5440))
  - update for the go-ipfs-cmds refactor ([ipfs/go-ipfs#5035](https://github.com/ipfs/go-ipfs/pull/5035))
  - fix(unixfs): issue #5217 (Avoid use of `pb.Data`) ([ipfs/go-ipfs#5505](https://github.com/ipfs/go-ipfs/pull/5505))
  - fix(unixfs): issue #5055 ([ipfs/go-ipfs#5525](https://github.com/ipfs/go-ipfs/pull/5525))
  - add offline id test #4978 and refactor command code ([ipfs/go-ipfs#5562](https://github.com/ipfs/go-ipfs/pull/5562))
  - refact(command): replace option name with const string ([ipfs/go-ipfs#5642](https://github.com/ipfs/go-ipfs/pull/5642))
  - remove p2p-circuit addr hack in ipfs swarm peers ([ipfs/go-ipfs#5645](https://github.com/ipfs/go-ipfs/pull/5645))
  - refactor(commands/id): use new command ([ipfs/go-ipfs#5646](https://github.com/ipfs/go-ipfs/pull/5646))
  - object patch rm-link: change arg from 'link' to 'name' ([ipfs/go-ipfs#5638](https://github.com/ipfs/go-ipfs/pull/5638))
  - refactor(cmds): use new cmds lib in version, tar and dns ([ipfs/go-ipfs#5650](https://github.com/ipfs/go-ipfs/pull/5650))
  - cmds/dag: use new cmds lib ([ipfs/go-ipfs#5662](https://github.com/ipfs/go-ipfs/pull/5662))
  - commands/ping: use new cmds lib ([ipfs/go-ipfs#5675](https://github.com/ipfs/go-ipfs/pull/5675))

### related changelogs

Changes to sub-packages go-ipfs depends on. This *does not* include libp2p or multiformats.

github.com/ipfs/go-log
  - update gogo protobuf ([ipfs/go-log#39](https://github.com/ipfs/go-log/pull/39))
  - rename the protobuf to loggabletracer ([ipfs/go-log#41](https://github.com/ipfs/go-log/pull/41))
  - protect loggers with rwmutex ([ipfs/go-log#44](https://github.com/ipfs/go-log/pull/44))
  - make logging prettier ([ipfs/go-log#45](https://github.com/ipfs/go-log/pull/45))
  - add env vars for logging to file and syslog ([ipfs/go-log#46](https://github.com/ipfs/go-log/pull/46))
  - remove syslogger ([ipfs/go-log#47](https://github.com/ipfs/go-log/pull/47))

github.com/ipfs/go-datastore
  - implement DiskUsage for the rest of the datastores ([ipfs/go-datastore#86](https://github.com/ipfs/go-datastore/pull/86))
  - switch to google's uuid library ([ipfs/go-datastore#89](https://github.com/ipfs/go-datastore/pull/89))
  - return ErrNotFound from the NullDatastore instead of nil, nil ([ipfs/go-datastore#92](https://github.com/ipfs/go-datastore/pull/92))
  - Add TTL and Transactional interfaces ([ipfs/go-datastore#91](https://github.com/ipfs/go-datastore/pull/91))
  - improve testing ([ipfs/go-datastore#93](https://github.com/ipfs/go-datastore/pull/93))
  - Add support for querying entry expiration ([ipfs/go-datastore#96](https://github.com/ipfs/go-datastore/pull/96))
  - Allow ds.NewTransaction() to return an error ([ipfs/go-datastore#98](https://github.com/ipfs/go-datastore/pull/98))
  - add a GetSize method ([ipfs/go-datastore#99](https://github.com/ipfs/go-datastore/pull/99))

github.com/ipfs/go-cid
  - Add tests for Set type ([ipfs/go-cid#63](https://github.com/ipfs/go-cid/pull/63))
  - Create new Builder interface for creating CIDs. ([ipfs/go-cid#53](https://github.com/ipfs/go-cid/pull/53))
  - cid-fmt Enhancments ([ipfs/go-cid#61](https://github.com/ipfs/go-cid/pull/61))
  - add String benchmark ([ipfs/go-cid#44](https://github.com/ipfs/go-cid/pull/44))
  - add a streaming CID set ([ipfs/go-cid#67](https://github.com/ipfs/go-cid/pull/67))
  - Extract non-core functionality from go-cid into go-cidutil ([ipfs/go-cid#69](https://github.com/ipfs/go-cid/pull/69))
  - cid implementation research ([ipfs/go-cid#70](https://github.com/ipfs/go-cid/pull/70))
  - cid implementation variations++ ([ipfs/go-cid#72](https://github.com/ipfs/go-cid/pull/72))
  - Create a new Encode method that is like StringOfBase but never errors ([ipfs/go-cid#60](https://github.com/ipfs/go-cid/pull/60))
  - add codecs for Dash blocks, tx ([ipfs/go-cid#78](https://github.com/ipfs/go-cid/pull/78))

github.com/ipfs/go-ds-flatfs
  - check error before defer-removing disk usage file ([ipfs/go-ds-flatfs#47](https://github.com/ipfs/go-ds-flatfs/pull/47))
  - add GetSize function ([ipfs/go-ds-flatfs#48](https://github.com/ipfs/go-ds-flatfs/pull/48))

github.com/ipfs/go-ds-measure
  -  ([ipfs/go-ds-measure#](https://github.com/ipfs/go-ds-measure/pull/))

github.com/ipfs/go-ds-leveldb
  - recover datastore on corruption ([ipfs/go-ds-leveldb#15](https://github.com/ipfs/go-ds-leveldb/pull/15))
  - Add transactional support to leveldb datastore. ([ipfs/go-ds-leveldb#17](https://github.com/ipfs/go-ds-leveldb/pull/17))
  - implement GetSize ([ipfs/go-ds-leveldb#18](https://github.com/ipfs/go-ds-leveldb/pull/18))

github.com/ipfs/go-metrics-prometheus
  - use an existing metric when it has already been registered ([ipfs/go-metrics-prometheus#1](https://github.com/ipfs/go-metrics-prometheus/pull/1))

github.com/ipfs/go-metrics-interface
  - update the counter interface to match prometheus ([ipfs/go-metrics-interface#2](https://github.com/ipfs/go-metrics-interface/pull/2))

github.com/ipfs/go-ipld-format
  - add copy dagservice function ([ipfs/go-ipld-format#41](https://github.com/ipfs/go-ipld-format/pull/41))

github.com/ipfs/go-ipld-cbor
  - Refactor to refmt ([ipfs/go-ipld-cbor#30](https://github.com/ipfs/go-ipld-cbor/pull/30))
  - import changes from the filecoin branch ([ipfs/go-ipld-cbor#41](https://github.com/ipfs/go-ipld-cbor/pull/41))
  - register the BitIntAtlasEntry for the tests ([ipfs/go-ipld-cbor#43](https://github.com/ipfs/go-ipld-cbor/pull/43))
  - attempt to allocate a bit less ([ipfs/go-ipld-cbor#45](https://github.com/ipfs/go-ipld-cbor/pull/45))

github.com/ipfs/go-ipfs-cmds
  - check if we can decode an error before trying ([ipfs/go-ipfs-cmds#108](https://github.com/ipfs/go-ipfs-cmds/pull/108))
  - fix(option): print error message for error timeout option ([ipfs/go-ipfs-cmds#118](https://github.com/ipfs/go-ipfs-cmds/pull/118))
  - Create Jenkinsfile ([ipfs/go-ipfs-cmds#89](https://github.com/ipfs/go-ipfs-cmds/pull/89))
  - fix(add): refer to ipfs issue #5456 ([ipfs/go-ipfs-cmds#121](https://github.com/ipfs/go-ipfs-cmds/pull/121))
  - commands refactor 2.0 ([ipfs/go-ipfs-cmds#112](https://github.com/ipfs/go-ipfs-cmds/pull/112))
  - always assign keks to review new PRs ([ipfs/go-ipfs-cmds#123](https://github.com/ipfs/go-ipfs-cmds/pull/123))
  - extract go-ipfs-files ([ipfs/go-ipfs-cmds#125](https://github.com/ipfs/go-ipfs-cmds/pull/125))
  - split the value encoder and the error encoder ([ipfs/go-ipfs-cmds#128](https://github.com/ipfs/go-ipfs-cmds/pull/128))

github.com/ipfs/go-ipfs-cmdkit
  - all: gofmt ([ipfs/go-ipfs-cmdkit#22](https://github.com/ipfs/go-ipfs-cmdkit/pull/22))
  - add standard ci scripts ([ipfs/go-ipfs-cmdkit#23](https://github.com/ipfs/go-ipfs-cmdkit/pull/23))
  - only count size for regular files ([ipfs/go-ipfs-cmdkit#25](https://github.com/ipfs/go-ipfs-cmdkit/pull/25))
  - Create Jenkinsfile ([ipfs/go-ipfs-cmdkit#16](https://github.com/ipfs/go-ipfs-cmdkit/pull/16))
  - Feat: add WebFile File implementation. ([ipfs/go-ipfs-cmdkit#26](https://github.com/ipfs/go-ipfs-cmdkit/pull/26))
  - feat(type): fix issue #28 ([ipfs/go-ipfs-cmdkit#29](https://github.com/ipfs/go-ipfs-cmdkit/pull/29))
  - Extract files package ([ipfs/go-ipfs-cmdkit#31](https://github.com/ipfs/go-ipfs-cmdkit/pull/31))

github.com/ipfs/go-ds-badger
  - update protobuf ([ipfs/go-ds-badger#26](https://github.com/ipfs/go-ds-badger/pull/26))
  - exported type datastore => Datastore ([ipfs/go-ds-badger#1](https://github.com/ipfs/go-ds-badger/pull/1))
  - using exported Datastore type ([ipfs/go-ds-badger#2](https://github.com/ipfs/go-ds-badger/pull/2))
  - exported type datastore => Datastore ([ipfs/go-ds-badger#28](https://github.com/ipfs/go-ds-badger/pull/28))
  - Implement new TxDatastore and Txn interfaces ([ipfs/go-ds-badger#27](https://github.com/ipfs/go-ds-badger/pull/27))
  - Avoid discarding transaction too early in queries ([ipfs/go-ds-badger#31](https://github.com/ipfs/go-ds-badger/pull/31))
  - Ability to get entry expirations ([ipfs/go-ds-badger#32](https://github.com/ipfs/go-ds-badger/pull/32))
  - Update badger to 2.8.0 ([ipfs/go-ds-badger#33](https://github.com/ipfs/go-ds-badger/pull/33))
  - ds.NewTransaction() now returns an error parameter ([ipfs/go-ds-badger#36](https://github.com/ipfs/go-ds-badger/pull/36))
  - make has faster ([ipfs/go-ds-badger#37](https://github.com/ipfs/go-ds-badger/pull/37))
  - Implement GetSize and update badger ([ipfs/go-ds-badger#38](https://github.com/ipfs/go-ds-badger/pull/38))

github.com/ipfs/go-ipfs-addr
  - Remove dependency on libp2p-circuit ([ipfs/go-ipfs-addr#7](https://github.com/ipfs/go-ipfs-addr/pull/7))

github.com/ipfs/go-ipfs-chunker
  - return err when rabin min less than 16 ([ipfs/go-ipfs-chunker#3](https://github.com/ipfs/go-ipfs-chunker/pull/3))
  - switch to go-buffer-pool ([ipfs/go-ipfs-chunker#8](https://github.com/ipfs/go-ipfs-chunker/pull/8))
  - fix size-0 chunker bug ([ipfs/go-ipfs-chunker#9](https://github.com/ipfs/go-ipfs-chunker/pull/9))

github.com/ipfs/go-ipfs-routing
  - update protobuf ([ipfs/go-ipfs-routing#8](https://github.com/ipfs/go-ipfs-routing/pull/8))
  - Implement SearchValue ([ipfs/go-ipfs-routing#12](https://github.com/ipfs/go-ipfs-routing/pull/12))

github.com/ipfs/go-ipfs-blockstore
  - blockstore: Adding Stat method to map from Cid to BlockSize ([ipfs/go-ipfs-blockstore#5](https://github.com/ipfs/go-ipfs-blockstore/pull/5))
  - correctly convert the datastore not found errors ([ipfs/go-ipfs-blockstore#10](https://github.com/ipfs/go-ipfs-blockstore/pull/10))
  - Fix typo: Change 'should not' to 'should' ([ipfs/go-ipfs-blockstore#14](https://github.com/ipfs/go-ipfs-blockstore/pull/14))
  - fix test race condition ([ipfs/go-ipfs-blockstore#9](https://github.com/ipfs/go-ipfs-blockstore/pull/9))
  - make arccache.GetSize return ErrNotFound when not found ([ipfs/go-ipfs-blockstore#16](https://github.com/ipfs/go-ipfs-blockstore/pull/16))
  - use datastore.GetSize ([ipfs/go-ipfs-blockstore#17](https://github.com/ipfs/go-ipfs-blockstore/pull/17))

github.com/ipfs/go-ipns
  - update gogo protobuf ([ipfs/go-ipns#16](https://github.com/ipfs/go-ipns/pull/16))
  - use new ExtractPublicKey signature ([ipfs/go-ipns#17](https://github.com/ipfs/go-ipns/pull/17))

github.com/ipfs/go-bitswap
  - update gogo protobuf ([ipfs/go-bitswap#2](https://github.com/ipfs/go-bitswap/pull/2))
  - ci: add jenkins ([ipfs/go-bitswap#9](https://github.com/ipfs/go-bitswap/pull/9))
  - bitswap: Bitswap now sends multiple blocks per message ([ipfs/go-bitswap#5](https://github.com/ipfs/go-bitswap/pull/5))
  - reduce allocations ([ipfs/go-bitswap#12](https://github.com/ipfs/go-bitswap/pull/12))
  - buffer writes ([ipfs/go-bitswap#15](https://github.com/ipfs/go-bitswap/pull/15))
  - delay finding providers ([ipfs/go-bitswap#17](https://github.com/ipfs/go-bitswap/pull/17))
github.com/ipfs/go-blockservice
  - Avoid allocating a session unless we need it ([ipfs/go-blockservice#6](https://github.com/ipfs/go-blockservice/pull/6))

github.com/ipfs/go-cidutil
  - add a utility method for sorting CID slices ([ipfs/go-cidutil#5](https://github.com/ipfs/go-cidutil/pull/5))

github.com/ipfs/go-ipfs-config
  - Add pubsub configuration options ([ipfs/go-ipfs-config#3](https://github.com/ipfs/go-ipfs-config/pull/3))
  - add QUIC experiment ([ipfs/go-ipfs-config#4](https://github.com/ipfs/go-ipfs-config/pull/4))
  - Add Gateway.APICommands for /api allowlists ([ipfs/go-ipfs-config#10](https://github.com/ipfs/go-ipfs-config/pull/10))
  - allow multiple API/Gateway addresses ([ipfs/go-ipfs-config#11](https://github.com/ipfs/go-ipfs-config/pull/11))
  - Fix handling of null strings ([ipfs/go-ipfs-config#12](https://github.com/ipfs/go-ipfs-config/pull/12))
  - add experiment for p2p http proxy ([ipfs/go-ipfs-config#13](https://github.com/ipfs/go-ipfs-config/pull/13))
  - add message signing config options ([ipfs/go-ipfs-config#18](https://github.com/ipfs/go-ipfs-config/pull/18))

github.com/ipfs/go-merkledag
  - Add FetchGraphWithDepthLimit to specify depth-limited graph fetching. ([ipfs/go-merkledag#2](https://github.com/ipfs/go-merkledag/pull/2))
  - update gogo protobuf ([ipfs/go-merkledag#4](https://github.com/ipfs/go-merkledag/pull/4))
  - Update to use new Builder interface for creating CIDs. ([ipfs/go-merkledag#6](https://github.com/ipfs/go-merkledag/pull/6))
  - perf: avoid allocations when filtering nodes ([ipfs/go-merkledag#11](https://github.com/ipfs/go-merkledag/pull/11))

github.com/ipfs/go-mfs
  - fix(unixfs): issue #6 ([ipfs/go-mfs#7](https://github.com/ipfs/go-mfs/pull/7))
  - fix(type): issue #13 ([ipfs/go-mfs#14](https://github.com/ipfs/go-mfs/pull/14))

github.com/ipfs/go-path
  - fix: don't dag.Get in ResolveToLastNode when not needed ([ipfs/go-path#1](https://github.com/ipfs/go-path/pull/1))

github.com/ipfs/go-unixfs
  - update gogo protobuf ([ipfs/go-unixfs#6](https://github.com/ipfs/go-unixfs/pull/6))
  - Update to use new Builder interface for creating CIDs. ([ipfs/go-unixfs#7](https://github.com/ipfs/go-unixfs/pull/7))
  - nit: make dagTruncate a method on DagModifier ([ipfs/go-unixfs#13](https://github.com/ipfs/go-unixfs/pull/13))
  - fix(fsnode): issue #17 ([ipfs/go-unixfs#18](https://github.com/ipfs/go-unixfs/pull/18))
  - Use EnumerateChildrenAsync in for enumerating HAMT links ([ipfs/go-unixfs#19](https://github.com/ipfs/go-unixfs/pull/19))

## 0.4.17 2018-07-27

Ipfs 0.4.17 is a quick release to fix a major performance regression in bitswap
(mostly affecting go-ipfs -> js-ipfs transfers). However, while motivated by
this fix, this release contains a few other goodies that will excite some users.

The headline feature in this release is [urlstore][] support. Urlstore is a
generalization of the filestore backend that can fetch file blocks from remote
URLs on-demand instead of storing them in the local datastore.

Additionally, we've added support for extracting inline blocks from CIDs (blocks
inlined into CIDs using the identity hash function). However, go-ipfs won't yet
*create* such CIDs so you're unlikely to see any in the wild.

[urlstore]: https://github.com/ipfs/go-ipfs/blob/master/docs/experimental-features.md#ipfs-urlstore

Features:

* URLStore ([ipfs/go-ipfs#4896](https://github.com/ipfs/go-ipfs/pull/4896))
* Add trickle-dag support to the urlstore ([ipfs/go-ipfs#5245](https://github.com/ipfs/go-ipfs/pull/5245)).
* Allow specifying how the data field in the `object get` is encoded ([ipfs/go-ipfs#5139](https://github.com/ipfs/go-ipfs/pull/5139))
* Add a `-U` flag to `files ls` to disable sorting ([ipfs/go-ipfs#5219](https://github.com/ipfs/go-ipfs/pull/5219))
* Add an efficient `--size-only` flag to the `repo stat` ([ipfs/go-ipfs#5010](https://github.com/ipfs/go-ipfs/pull/5010))
* Inline blocks in CIDs ([ipfs/go-ipfs#5117](https://github.com/ipfs/go-ipfs/pull/5117))

Changes/Fixes:

* Make `ipfs files ls -l` correctly report the hash and size of files ([ipfs/go-ipfs#5045](https://github.com/ipfs/go-ipfs/pull/5045))
* Fix sorting of `files ls` ([ipfs/go-ipfs#5219](https://github.com/ipfs/go-ipfs/pull/5219))
* Improve prefetching in `ipfs cat` and related commands ([ipfs/go-ipfs#5162](https://github.com/ipfs/go-ipfs/pull/5162))
* Better error message when `ipfs cp` fails ([ipfs/go-ipfs#5218](https://github.com/ipfs/go-ipfs/pull/5218))
* Don't wait for the peer to close it's end of a bitswap stream before considering the block "sent" ([ipfs/go-ipfs#5258](https://github.com/ipfs/go-ipfs/pull/5258))
* Fix resolving links in sharded directories via the gateway ([ipfs/go-ipfs#5271](https://github.com/ipfs/go-ipfs/pull/5271))
* Fix building when there's a space in the current directory ([ipfs/go-ipfs#5261](https://github.com/ipfs/go-ipfs/pull/5261))

Documentation:

* Improve documentation about the bloomfilter config options ([ipfs/go-ipfs#4924](https://github.com/ipfs/go-ipfs/pull/4924))

General refactorings and internal bug fixes:

* Remove the `Offset()` method from the DAGReader ([ipfs/go-ipfs#5190](https://github.com/ipfs/go-ipfs/pull/5190))
* Fix TestLargeWriteChunks seek behavior ([ipfs/go-ipfs#5276](https://github.com/ipfs/go-ipfs/pull/5276))
* Add a build tag to disable dynamic plugins ([ipfs/go-ipfs#5274](https://github.com/ipfs/go-ipfs/pull/5274))
* Use FSNode instead of the Protobuf structure in PBDagReader ([ipfs/go-ipfs#5189](https://github.com/ipfs/go-ipfs/pull/5189))
* Remove support for non-directory MFS roots ([ipfs/go-ipfs#5170](https://github.com/ipfs/go-ipfs/pull/5170))
* Remove `UnixfsNode` from the balanced builder ([ipfs/go-ipfs#5118](https://github.com/ipfs/go-ipfs/pull/5118))
* Fix truncating files (internal) when already at the correct size ([ipfs/go-ipfs#5253](https://github.com/ipfs/go-ipfs/pull/5253))
* Fix `dagTruncate` (internal) to preserve the node type ([ipfs/go-ipfs#5216](https://github.com/ipfs/go-ipfs/pull/5216))
* Add an internal interface for unixfs directories ([ipfs/go-ipfs#5160](https://github.com/ipfs/go-ipfs/pull/5160))
* Refactor the CoreAPI path types and interfaces ([ipfs/go-ipfs#4672](https://github.com/ipfs/go-ipfs/pull/4672))
* Refactor `precalcNextBuf` in the dag reader ([ipfs/go-ipfs#5237](https://github.com/ipfs/go-ipfs/pull/5237))
* Update a bunch of dependencies that haven't been updated for a while ([ipfs/go-ipfs#5268](https://github.com/ipfs/go-ipfs/pull/5268))

## 0.4.16 2018-07-13

Ipfs 0.4.16 is a fairly small release in terms of changes to the ipfs codebase,
but it contains a huge amount of changes and improvements from the libraries we
depend on, notably libp2p.

This release includes small a repo migration to account for some changes to the
DHT. It should only take a second to run but, depending on your configuration,
you may need to run it manually.

You can run a migration by either:

1. Selecting "Yes" when the daemon prompts you to migrate.
2. Running the daemon with the `--migrate=true` flag.
3. Manually [running](https://github.com/ipfs/fs-repo-migrations/blob/master/run.md#running-repo-migrations) the migration.

### Libp2p

This version of ipfs contains the changes made in libp2p from v5.0.14 through
v6.0.5. In that time, we have made significant changes to the codebase to allow
for easier integration of future transports and modules along with the usual
performance and reliability improvements. You can find many of these
improvements in the libp2p 6.0 [release blog
post](https://ipfs.io/blog/39-go-libp2p-6-0-0/).

The primary motivation for this refactor was adding support for network
transports like QUIC that have built-in support for encryption, authentication,
and stream multiplexing. It will also allow us to plug-in new security
transports (like TLS) without hard-coding them.

For example, our [QUIC
transport](https://github.com/libp2p/go-libp2p-quic-transport) currently works,
and can be plugged into libp2p manually (though note that it is still
experimental, as the upstream spec is still in flux). Further work is needed to
make enabling this inside ipfs easy and not require recompilation.

On the user-visible side of things, we've improved our dialing logic and
timeouts. We now abort dials to local subnets after 5 seconds and abort all
dials if the TCP handshake takes longer than 5 seconds. This should
significantly improve performance in some cases as we limit the number of
concurrent dials and slow dials to non-responsive peers have been known to clog
the dialer, blocking dials to reachable peers. Importantly, this should improve
DHT performance as it tends to spend a disproportional amount of time connecting
to peers.

We have also made a few noticeable changes to the DHT: we've significantly
improved the chances of finding a value on the DHT, tightened up some of our
validation logic, and fixed some issues that should reduce traffic to nodes
running in dhtclient mode over time.

Of these, the first one will likely see the most impact. In the past, when
putting a value (e.g., an IPNS entry) into the DHT, we'd try to put the value to
K peers (where K for us is 20). However, we'd often fail to connect to many of
these peers so we'd end up putting the value to significantly fewer than K
peers. We now try to put the value to the K peers we can actually connect to.

Finally, we've fixed JavaScript interoperability in go-multiplex, the one stream
muxer that both go-libp2p and js-libp2p implement. This should significantly
improve go-libp2p and js-libp2p interoperability.

### Multiformats

We are also changing the way that people write 'ipfs' multiaddrs. Currently,
ipfs multiaddrs look something like
`/ip4/104.131.131.82/tcp/4001/ipfs/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ`.
However, calling them 'ipfs' multiaddrs is a bit misleading, as this is actually
the multiaddr of a libp2p peer that happens to run ipfs. Other protocols built
on libp2p right now still have to use multiaddrs that say 'ipfs', even if they
have nothing to do with ipfs. Therefore, we are renaming them to 'p2p'
multiaddrs. Moving forward, these addresses will be written as:
`/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ`.

This release adds support for *parsing* both types of addresses (`.../ipfs/...`
and `.../p2p/...`) into the same network format, and the network format is
remaining exactly the same. A future release will have the ipfs daemon switch to
*printing* out addresses this way once a large enough portion of the network
has upgraded.

N.B., these addresses are *not* related to IPFS *file* names (`/ipfs/Qm...`).
Disambiguating the two was yet another motivation to switch the protocol name to
`/p2p/`.

### IPFS

On the ipfs side of things, we've started embedding public keys inside IPNS
records and have enabled the Git plugin by default.

Embedding public keys inside IPNS records allows lookups to be faster as we only
need to fetch the record itself (and not the public key separately). It also
fixes an issue where DHT peers wouldn't store a record for a peer if they didn't
have their public key already. Combined with some of the DHT and dialing fixes,
this should improve the performance of IPNS (once a majority of the network
updates).

Second, our public builds now include the Git plugin (in past builds, you could
add it yourself, but doing so was not easy). With this, ipfs can ingest and
operate over Git repositories and commit graphs directly. For more information
on this, see [the go-ipld-git repo](https://github.com/ipfs/go-ipld-git).

Finally, we've included many smaller bugfixes, refactorings, improved
documentation, and a good bit more. For the full details, see the changelog
below.

## 0.4.16-rc3 2018-07-09
- Bugfixes
  - Fix dht commands when ipns over pubsub is enabled ([ipfs/go-ipfs#5200](https://github.com/ipfs/go-ipfs/pull/5200))
  - Fix content routing when ipns over pubsub is enabled ([ipfs/go-ipfs#5200](https://github.com/ipfs/go-ipfs/pull/5200))
  - Correctly handle multi-hop dnslink resolution ([ipfs/go-ipfs#5202](https://github.com/ipfs/go-ipfs/pull/5202))

## 0.4.16-rc2 2018-07-05
- Bugfixes
  - Fix usage of file name vs path name in adder ([ipfs/go-ipfs#5167](https://github.com/ipfs/go-ipfs/pull/5167))
  - Fix `ipfs update` working with migrations ([ipfs/go-ipfs#5194](https://github.com/ipfs/go-ipfs/pull/5194))
- Documentation
  - Grammar fix in fuse docs ([ipfs/go-ipfs#5164](https://github.com/ipfs/go-ipfs/pull/5164))

## 0.4.16-rc1 2018-06-27
- Features
  - Embed public keys inside ipns records, use for validation ([ipfs/go-ipfs#5079](https://github.com/ipfs/go-ipfs/pull/5079))
  - Preload git plugin by default ([ipfs/go-ipfs#4991](https://github.com/ipfs/go-ipfs/pull/4991))
- Improvements
  - Only resolve dnslinks once in the gateway ([ipfs/go-ipfs#4977](https://github.com/ipfs/go-ipfs/pull/4977))
  - Libp2p transport refactor update ([ipfs/go-ipfs#4817](https://github.com/ipfs/go-ipfs/pull/4817))
  - Improve swarm connect/disconnect commands ([ipfs/go-ipfs#5107](https://github.com/ipfs/go-ipfs/pull/5107))
- Documentation
  - Fix typo of sudo install command ([ipfs/go-ipfs#5001](https://github.com/ipfs/go-ipfs/pull/5001))
  - Fix experimental features Table of Contents ([ipfs/go-ipfs#4976](https://github.com/ipfs/go-ipfs/pull/4976))
  - Fix link to systemd init scripts in the README ([ipfs/go-ipfs#4968](https://github.com/ipfs/go-ipfs/pull/4968))
  - Add package overview comments to coreapi ([ipfs/go-ipfs#5108](https://github.com/ipfs/go-ipfs/pull/5108))
  - Add README to docs folder ([ipfs/go-ipfs#5095](https://github.com/ipfs/go-ipfs/pull/5095))
  - Add system requirements to README ([ipfs/go-ipfs#5137](https://github.com/ipfs/go-ipfs/pull/5137))
- Bugfixes
  - Fix goroutine leak in pin verify ([ipfs/go-ipfs#5011](https://github.com/ipfs/go-ipfs/pull/5011))
  - Fix commit string in version ([ipfs/go-ipfs#4982](https://github.com/ipfs/go-ipfs/pull/4982))
  - Fix `key rename` command output error ([ipfs/go-ipfs#4962](https://github.com/ipfs/go-ipfs/pull/4962))
  - Report error source when failing to construct private network ([ipfs/go-ipfs#4952](https://github.com/ipfs/go-ipfs/pull/4952))
  - Fix build on DragonFlyBSD ([ipfs/go-ipfs#5031](https://github.com/ipfs/go-ipfs/pull/5031))
  - Fix goroutine leak in dag put ([ipfs/go-ipfs#5016](https://github.com/ipfs/go-ipfs/pull/5016))
  - Fix goroutine leaks in refs.go ([ipfs/go-ipfs#5018](https://github.com/ipfs/go-ipfs/pull/5018))
  - Fix panic, Don't handle errors with fallthrough ([ipfs/go-ipfs#5072](https://github.com/ipfs/go-ipfs/pull/5072))
  - Fix how filestore is hooked up with caching ([ipfs/go-ipfs#5122](https://github.com/ipfs/go-ipfs/pull/5122))
  - Add record validation to offline routing ([ipfs/go-ipfs#5116](https://github.com/ipfs/go-ipfs/pull/5116))
  - Fix `ipfs update` working with migrations ([ipfs/go-ipfs#5194](https://github.com/ipfs/go-ipfs/pull/5194))
- General Changes and Refactorings
  - Remove leftover bits of dead code ([ipfs/go-ipfs#5022](https://github.com/ipfs/go-ipfs/pull/5022))
  - Remove fuse platform build constraints ([ipfs/go-ipfs#5033](https://github.com/ipfs/go-ipfs/pull/5033))
  - Warning when legacy NoSync setting is set ([ipfs/go-ipfs#5036](https://github.com/ipfs/go-ipfs/pull/5036))
  - Clean up and refactor namesys module ([ipfs/go-ipfs#5007](https://github.com/ipfs/go-ipfs/pull/5007))
  - When raw-leaves are used for empty files use 'Raw' nodes ([ipfs/go-ipfs#4693](https://github.com/ipfs/go-ipfs/pull/4693))
  - Update dist_root in build scripts ([ipfs/go-ipfs#5093](https://github.com/ipfs/go-ipfs/pull/5093))
  - Integrate `pb.Data` into `FSNode` to avoid duplicating fields ([ipfs/go-ipfs#5098](https://github.com/ipfs/go-ipfs/pull/5098))
  - Reduce log level when we can't republish ([ipfs/go-ipfs#5091](https://github.com/ipfs/go-ipfs/pull/5091))
  - Extract ipns record logic to go-ipns ([ipfs/go-ipfs#5124](https://github.com/ipfs/go-ipfs/pull/5124))
- Testing
  - Collect test times for sharness ([ipfs/go-ipfs#4959](https://github.com/ipfs/go-ipfs/pull/4959))
  - Fix sharness iptb connect timeout ([ipfs/go-ipfs#4966](https://github.com/ipfs/go-ipfs/pull/4966))
  - Add more timeouts to the jenkins pipeline ([ipfs/go-ipfs#4958](https://github.com/ipfs/go-ipfs/pull/4958))
  - Use go 1.10 on jenkins ([ipfs/go-ipfs#5009](https://github.com/ipfs/go-ipfs/pull/5009))
  - Speed up multinode sharness test ([ipfs/go-ipfs#4967](https://github.com/ipfs/go-ipfs/pull/4967))
  - Print out iptb logs on iptb test failure (for debugging CI) ([ipfs/go-ipfs#5069](https://github.com/ipfs/go-ipfs/pull/5069))
  - Disable the MacOS tests in jenkins ([ipfs/go-ipfs#5119](https://github.com/ipfs/go-ipfs/pull/5119))
  - Make republisher test robust against timing issues ([ipfs/go-ipfs#5125](https://github.com/ipfs/go-ipfs/pull/5125))
  - Archive sharness trash dirs in jenkins ([ipfs/go-ipfs#5071](https://github.com/ipfs/go-ipfs/pull/5071))
  - Fixup DHT sharness tests ([ipfs/go-ipfs#5114](https://github.com/ipfs/go-ipfs/pull/5114))
- Dependencies
  - Update go-ipld-git to fix mergetag resolving ([ipfs/go-ipfs#4988](https://github.com/ipfs/go-ipfs/pull/4988))
  - Fix duplicate /x/sys imports ([ipfs/go-ipfs#5068](https://github.com/ipfs/go-ipfs/pull/5068))
  - Update stream multiplexers ([ipfs/go-ipfs#5075](https://github.com/ipfs/go-ipfs/pull/5075))
  - Update dependencies: go-log, sys, go-crypto ([ipfs/go-ipfs#5100](https://github.com/ipfs/go-ipfs/pull/5100))
  - Explicitly import go-multiaddr-dns in config/bootstrap_peers ([ipfs/go-ipfs#5144](https://github.com/ipfs/go-ipfs/pull/5144))
  - Gx update with dht and dialing improvements ([ipfs/go-ipfs#5158](https://github.com/ipfs/go-ipfs/pull/5158))

## 0.4.15 2018-05-09

This release is significantly smaller than the last as much of the work on
improving our datastores, and other libraries libp2p has yet to be merged.
However, it still includes many welcome improvements.

As with 0.4.12 and 0.4.14 (0.4.13 was a patch), this release has a negative
diff-stat. Unfortunately, much of this code isn't actually going away but at
least it's being moved out into separate repositories.

Much of the work that made it into this release is under the hood. We've cleaned
up some code, extracted several packages into their own repositories, and made
some long neglected optimizations (e.g., handling of sharded directories).
Additionally, this release includes a bunch of tests for our CLI commands that
should help us avoid some of the issues we've seen in the past few releases.

More visibly, thanks to @djdv's efforts, this release includes some significant
Windows improvements (with more on the way). Specifically, this release includes
better handling of repo lockfiles (no more `ipfs repo fsck`), stdin command-line
support, and, last but not least, IPFS no longer writes random files with scary
garbage in the drive root. To read more about future windows improvements, take
a look at this [blog post](https://blog.ipfs.io/36-a-look-at-windows/).

To better support low-power devices, we've added a low-power config profile.
This can be enabled when initializing a repo by running `ipfs init` with the
`--profile=lowpower` flag or later by running `ipfs config profile apply lowpower`.

Finally, with this release we have begun distributing self-contained source
archives of go-ipfs and its dependencies. This should be a welcome improvement
for both packagers and those living in countries with harmonized internet
access.

- Features
  - Add options for record count and timeout for resolving DHT paths ([ipfs/go-ipfs#4733](https://github.com/ipfs/go-ipfs/pull/4733))
  - Add low power init profile ([ipfs/go-ipfs#4154](https://github.com/ipfs/go-ipfs/pull/4154))
  - Add Opentracing plugin support ([ipfs/go-ipfs#4506](https://github.com/ipfs/go-ipfs/pull/4506))
  - Add make target to build source tarballs ([ipfs/go-ipfs#4920](https://github.com/ipfs/go-ipfs/pull/4920))

- Improvements
  - Add BlockedFetched/Added/Removed events to Blockservice ([ipfs/go-ipfs#4649](https://github.com/ipfs/go-ipfs/pull/4649))
  - Improve performance of HAMT code ([ipfs/go-ipfs#4889](https://github.com/ipfs/go-ipfs/pull/4889))
  - Avoid unnecessarily resolving child nodes when listing a sharded directory ([ipfs/go-ipfs#4884](https://github.com/ipfs/go-ipfs/pull/4884))
  - Tar writer now supports sharded ipfs directories ([ipfs/go-ipfs#4873](https://github.com/ipfs/go-ipfs/pull/4873))
  - Infer type from CID when possible in `ipfs ls` ([ipfs/go-ipfs#4890](https://github.com/ipfs/go-ipfs/pull/4890))
  - Deduplicate keys in GetMany ([ipfs/go-ipfs#4888](https://github.com/ipfs/go-ipfs/pull/4888))

- Documentation
  - Fix spelling of retrieval ([ipfs/go-ipfs#4819](https://github.com/ipfs/go-ipfs/pull/4819))
  - Update broken links ([ipfs/go-ipfs#4798](https://github.com/ipfs/go-ipfs/pull/4798))
  - Remove roadmap.md ([ipfs/go-ipfs#4834](https://github.com/ipfs/go-ipfs/pull/4834))
  - Remove link to IPFS paper in contribute.md ([ipfs/go-ipfs#4812](https://github.com/ipfs/go-ipfs/pull/4812))
  - Fix broken todo link in readme.md ([ipfs/go-ipfs#4865](https://github.com/ipfs/go-ipfs/pull/4865))
  - Document ipns pubsub ([ipfs/go-ipfs#4903](https://github.com/ipfs/go-ipfs/pull/4903))
  - Fix missing profile docs ([ipfs/go-ipfs#4846](https://github.com/ipfs/go-ipfs/pull/4846))
  - Fix a few typos ([ipfs/go-ipfs#4835](https://github.com/ipfs/go-ipfs/pull/4835))
  - Fix typo in fsrepo error message ([ipfs/go-ipfs#4933](https://github.com/ipfs/go-ipfs/pull/4933))
  - Remove go-ipfs version from issue template ([ipfs/go-ipfs#4943](https://github.com/ipfs/go-ipfs/pull/4943))
  - Add docs for --profile=lowpower ([ipfs/go-ipfs#4970](https://github.com/ipfs/go-ipfs/pull/4970))
  - Improve Windows build documentation ([ipfs/go-ipfs#4691](https://github.com/ipfs/go-ipfs/pull/4691))

- Bugfixes
  - Check CIDs in base case when diffing nodes ([ipfs/go-ipfs#4767](https://github.com/ipfs/go-ipfs/pull/4767))
  - Support for CIDv1 with custom mhtype in `ipfs block put` ([ipfs/go-ipfs#4563](https://github.com/ipfs/go-ipfs/pull/4563))
  - Clean path in DagArchive ([ipfs/go-ipfs#4743](https://github.com/ipfs/go-ipfs/pull/4743))
  - Set the prefix for MFS root in `ipfs add --hash-only` ([ipfs/go-ipfs#4755](https://github.com/ipfs/go-ipfs/pull/4755))
  - Fix get output path ([ipfs/go-ipfs#4809](https://github.com/ipfs/go-ipfs/pull/4809))
  - Fix incorrect Read calls ([ipfs/go-ipfs#4792](https://github.com/ipfs/go-ipfs/pull/4792))
  - Use prefix in bootstrapWritePeers ([ipfs/go-ipfs#4832](https://github.com/ipfs/go-ipfs/pull/4832))
  - Fix mfs Directory.Path not working ([ipfs/go-ipfs#4844](https://github.com/ipfs/go-ipfs/pull/4844))
  - Remove header in `ipfs stats bw` if not polling ([ipfs/go-ipfs#4856](https://github.com/ipfs/go-ipfs/pull/4856))
  - Match Go's GOPATH defaults behaviour in build scripts ([ipfs/go-ipfs#4678](https://github.com/ipfs/go-ipfs/pull/4678))
  - Fix default-net profile not reverting bootstrap config ([ipfs/go-ipfs#4845](https://github.com/ipfs/go-ipfs/pull/4845))
  - Fix excess goroutines in bitswap caused by insecure CIDs ([ipfs/go-ipfs#4946](https://github.com/ipfs/go-ipfs/pull/4946))

- General Changes and Refactorings
  - Refactor trickle DAG builder ([ipfs/go-ipfs#4730](https://github.com/ipfs/go-ipfs/pull/4730))
  - Split the coreapi interface into multiple files ([ipfs/go-ipfs#4802](https://github.com/ipfs/go-ipfs/pull/4802))
  - Make `ipfs init` command use new cmds lib ([ipfs/go-ipfs#4732](https://github.com/ipfs/go-ipfs/pull/4732))
  - Extract thirdparty/tar package ([ipfs/go-ipfs#4857](https://github.com/ipfs/go-ipfs/pull/4857))
  - Reduce log level when for disconnected peers to info ([ipfs/go-ipfs#4811](https://github.com/ipfs/go-ipfs/pull/4811))
  - Only visit nodes in EnumerateChildrenAsync when asked ([ipfs/go-ipfs#4885](https://github.com/ipfs/go-ipfs/pull/4885))
  - Refactor coreapi options ([ipfs/go-ipfs#4807](https://github.com/ipfs/go-ipfs/pull/4807))
  - Fix error style for most errors ([ipfs/go-ipfs#4829](https://github.com/ipfs/go-ipfs/pull/4829))
  - Ensure `--help` always works, even with /dev/null stdin ([ipfs/go-ipfs#4849](https://github.com/ipfs/go-ipfs/pull/4849))
  - Deduplicate AddNodeLinkClean into AddNodeLink ([ipfs/go-ipfs#4940](https://github.com/ipfs/go-ipfs/pull/4940))
  - Remove some dead code ([ipfs/go-ipfs#4833](https://github.com/ipfs/go-ipfs/pull/4833))
  - Remove unused imports ([ipfs/go-ipfs#4955](https://github.com/ipfs/go-ipfs/pull/4955))
  - Fix go vet warnings ([ipfs/go-ipfs#4859](https://github.com/ipfs/go-ipfs/pull/4859))

- Testing
  - Generate JUnit test reports for sharness tests ([ipfs/go-ipfs#4530](https://github.com/ipfs/go-ipfs/pull/4530))
  - Fix t0063-daemon-init.sh by adding test profile to daemon ([ipfs/go-ipfs#4816](https://github.com/ipfs/go-ipfs/pull/4816))
  - Remove circular dependencies in merkledag package tests ([ipfs/go-ipfs#4704](https://github.com/ipfs/go-ipfs/pull/4704))
  - Check that all the commands fail when passed a bad flag ([ipfs/go-ipfs#4848](https://github.com/ipfs/go-ipfs/pull/4848))
  - Allow for some small margin of code coverage dropping on commit ([ipfs/go-ipfs#4867](https://github.com/ipfs/go-ipfs/pull/4867))
  - Add confirmation to archive-branches script ([ipfs/go-ipfs#4797](https://github.com/ipfs/go-ipfs/pull/4797))

- Dependencies
  - Update lock package ([ipfs/go-ipfs#4855](https://github.com/ipfs/go-ipfs/pull/4855))
  - Update to latest go-datastore. Remove thirdparty/datastore2 ([ipfs/go-ipfs#4742](https://github.com/ipfs/go-ipfs/pull/4742))
  - Extract fs lock into go-fs-lock ([ipfs/go-ipfs#4631](https://github.com/ipfs/go-ipfs/pull/4631))
  - Extract: exchange/interface.go, blocks/blocksutil, exchange/offline ([ipfs/go-ipfs#4912](https://github.com/ipfs/go-ipfs/pull/4912))
  - Remove unused lock dep ([ipfs/go-ipfs#4971](https://github.com/ipfs/go-ipfs/pull/4971))
  - Update iptb ([ipfs/go-ipfs#4965](https://github.com/ipfs/go-ipfs/pull/4965))
  - Update go-ipfs-cmds to fix stdin on windows ([ipfs/go-ipfs#4975](https://github.com/ipfs/go-ipfs/pull/4975))
  - Update go-ds-flatfs to fix windows corruption issue ([ipfs/go-ipfs#4872](https://github.com/ipfs/go-ipfs/pull/4872))

## 0.4.14 2018-03-22

Ipfs 0.4.14 is a big release with a large number of improvements and bugfixes.
It is also the first release of 2018, and our first release in over three
months. The release took longer than expected due to our refactoring and
extracting of our commands library. This refactor had two stages.  The first
round of the refactor disentangled the commands code from core ipfs code,
allowing us to move it out into a [separate
repository](https://github.com/ipfs/go-ipfs-cmds).  The code was previously
very entangled with the go-ipfs codebase and not usable for other projects.
The second round of the refactor had the goal of fixing several major issues
around streaming outputs, progress bars, and error handling. It also paved the
way for us to more easily provide an API over other transports, such as
websockets and unix domain sockets.  It took a while to flush out all the kinks
on such a massive change.  We're pretty sure we've got most of them, but if you
notice anything weird, please let us know.

Beyond that, we've added a new experimental way to use IPNS. With the new
pubsub IPNS resolver and publisher, you can subscribe to updates of an IPNS
entry, and the owner can publish out changes in real time. With this, IPNS can
become nearly instantaneous. To make use of this, simply start your ipfs daemon
with the `--enable-namesys-pubsub` option, and all IPNS resolution and
publishing will use pubsub. Note that resolving an IPNS name via pubsub without
someone publishing it via pubsub will result in a fallback to using the DHT.
Please give this a try and let us know how it goes!

Memory and CPU usage should see a noticeable improvement in this release. We
have spent considerable time fixing excess memory usage throughout the codebase
and down into libp2p. Fixes in peer tracking, bitswap allocation, pinning, and
many other places have brought down both peak and average memory usage. An
upgraded hashing library, base58 encoding library, and improved allocation
patterns all contribute to overall lower CPU usage across the board. See the
full changelist below for more memory and CPU usage improvements.

This release also brings the beginning of the ipfs 'Core API'. Once finished,
the Core API will be the primary way to interact with go-ipfs using go. Both
embedded nodes and nodes accessed over the http API will have the same
interface. Stay tuned for future updates and documentation.

These are only a sampling of the changes that made it into this release, the
full list (almost 100 PRs!) is below.

Finally, I'd like to thank everyone who contributed to this release, whether
you're just contributing a typo fix or driving new features. We are really
grateful to everyone who has spent their their time pushing ipfs forward.

SECURITY NOTE:

This release of ipfs disallows the usage of insecure hash functions and
lengths. Ipfs does not create these insecure objects for any purpose, but it
did allow manually creating them and fetching them from other peers. If you
currently have objects using insecure hashes in your local ipfs repo, please
remove them before updating.

#### Changes from rc2 to rc3
- Fix bug in stdin argument parsing ([ipfs/go-ipfs#4827](https://github.com/ipfs/go-ipfs/pull/4827))
- Revert commands back to sending a single response ([ipfs/go-ipfs#4822](https://github.com/ipfs/go-ipfs/pull/4822))

#### Changes from rc1 to rc2
- Fix issue in ipfs get caused by go1.10 changes ([ipfs/go-ipfs#4790](https://github.com/ipfs/go-ipfs/pull/4790))

- Features
  - Pubsub IPNS Publisher and Resolver (experimental) ([ipfs/go-ipfs#4047](https://github.com/ipfs/go-ipfs/pull/4047))
  - Implement coreapi Dag interface ([ipfs/go-ipfs#4471](https://github.com/ipfs/go-ipfs/pull/4471))
  - Add --offset flag to ipfs cat ([ipfs/go-ipfs#4538](https://github.com/ipfs/go-ipfs/pull/4538))
  - Command to apply config profile after init ([ipfs/go-ipfs#4195](https://github.com/ipfs/go-ipfs/pull/4195))
  - Implement coreapi Name and Key interfaces ([ipfs/go-ipfs#4477](https://github.com/ipfs/go-ipfs/pull/4477))
  - Add --length flag to ipfs cat ([ipfs/go-ipfs#4553](https://github.com/ipfs/go-ipfs/pull/4553))
  - Implement coreapi Object interface ([ipfs/go-ipfs#4492](https://github.com/ipfs/go-ipfs/pull/4492))
  - Implement coreapi Block interface ([ipfs/go-ipfs#4548](https://github.com/ipfs/go-ipfs/pull/4548))
  - Implement coreapi Pin interface ([ipfs/go-ipfs#4575](https://github.com/ipfs/go-ipfs/pull/4575))
  - Add a --with-local flag to ipfs files stat ([ipfs/go-ipfs#4638](https://github.com/ipfs/go-ipfs/pull/4638))
  - Disallow usage of blocks with insecure hashes ([ipfs/go-ipfs#4751](https://github.com/ipfs/go-ipfs/pull/4751))
- Improvements
  - Add uuid to event logs ([ipfs/go-ipfs#4392](https://github.com/ipfs/go-ipfs/pull/4392))
  - Add --quiet flag to object put ([ipfs/go-ipfs#4411](https://github.com/ipfs/go-ipfs/pull/4411))
  - Pinning memory improvements and fixes ([ipfs/go-ipfs#4451](https://github.com/ipfs/go-ipfs/pull/4451))
  - Update WebUI version ([ipfs/go-ipfs#4449](https://github.com/ipfs/go-ipfs/pull/4449))
  - Check strong and weak ETag validator ([ipfs/go-ipfs#3983](https://github.com/ipfs/go-ipfs/pull/3983))
  - Improve and refactor FD limit handling ([ipfs/go-ipfs#3801](https://github.com/ipfs/go-ipfs/pull/3801))
  - Support linking to non-dagpb objects in ipfs object patch ([ipfs/go-ipfs#4460](https://github.com/ipfs/go-ipfs/pull/4460))
  - Improve allocation patterns of slices in bitswap ([ipfs/go-ipfs#4458](https://github.com/ipfs/go-ipfs/pull/4458))
  - Secio handshake now happens synchronously ([libp2p/go-libp2p-secio#25](https://github.com/libp2p/go-libp2p-secio/pull/25))
  - Don't block closing connections on pending writes ([libp2p/go-msgio#7](https://github.com/libp2p/go-msgio/pull/7))
  - Improve memory usage of multiaddr parsing ([multiformats/go-multiaddr#56](https://github.com/multiformats/go-multiaddr/pull/56))
  - Don't lock up 256KiB buffers when adding small files ([ipfs/go-ipfs#4508](https://github.com/ipfs/go-ipfs/pull/4508))
  - Clear out memory after reads from the dagreader ([ipfs/go-ipfs#4525](https://github.com/ipfs/go-ipfs/pull/4525))
  - Improve error handling in ipfs ping ([ipfs/go-ipfs#4546](https://github.com/ipfs/go-ipfs/pull/4546))
  - Allow install.sh to be run without being the script dir ([ipfs/go-ipfs#4547](https://github.com/ipfs/go-ipfs/pull/4547))
  - Much faster base58 encoding ([libp2p/go-libp2p-peer#24](https://github.com/libp2p/go-libp2p-peer/pull/24))
  - Use faster sha256 and blake2b libs ([multiformats/go-multihash#63](https://github.com/multiformats/go-multihash/pull/63))
  - Greatly improve peerstore memory usage ([libp2p/go-libp2p-peerstore#22](https://github.com/libp2p/go-libp2p-peerstore/pull/22))
  - Improve dht memory usage and peer tracking ([libp2p/go-libp2p-kad-dht#111](https://github.com/libp2p/go-libp2p-kad-dht/pull/111))
  - New libp2p metrics lib with lower overhead ([libp2p/go-libp2p-metrics#8](https://github.com/libp2p/go-libp2p-metrics/pull/8))
  - Fix memory leak that occurred when dialing many peers ([libp2p/go-libp2p-swarm#51](https://github.com/libp2p/go-libp2p-swarm/pull/51))
  - Wire up new dag interfaces to make sessions easier ([ipfs/go-ipfs#4641](https://github.com/ipfs/go-ipfs/pull/4641))
- Documentation
  - Correct StorageMax config description ([ipfs/go-ipfs#4388](https://github.com/ipfs/go-ipfs/pull/4388))
  - Add how to download IPFS with IPFS doc ([ipfs/go-ipfs#4390](https://github.com/ipfs/go-ipfs/pull/4390))
  - Document gx release checklist item ([ipfs/go-ipfs#4480](https://github.com/ipfs/go-ipfs/pull/4480))
  - Add some documentation to CoreAPI ([ipfs/go-ipfs#4493](https://github.com/ipfs/go-ipfs/pull/4493))
  - Add interop tests to the release checklist ([ipfs/go-ipfs#4501](https://github.com/ipfs/go-ipfs/pull/4501))
  - Add badgerds to experimental-features ToC ([ipfs/go-ipfs#4537](https://github.com/ipfs/go-ipfs/pull/4537))
  - Fix typos and inconsistencies in commands documentation ([ipfs/go-ipfs#4552](https://github.com/ipfs/go-ipfs/pull/4552))
  - Add a document to help troubleshoot data transfers ([ipfs/go-ipfs#4332](https://github.com/ipfs/go-ipfs/pull/4332))
  - Add a bunch of documentation on public interfaces ([ipfs/go-ipfs#4599](https://github.com/ipfs/go-ipfs/pull/4599))
  - Expand the issue template and remove the severity field ([ipfs/go-ipfs#4624](https://github.com/ipfs/go-ipfs/pull/4624))
  - Add godocs for importers module ([ipfs/go-ipfs#4640](https://github.com/ipfs/go-ipfs/pull/4640))
  - Document make targets ([ipfs/go-ipfs#4653](https://github.com/ipfs/go-ipfs/pull/4653))
  - Add godocs for merkledag module ([ipfs/go-ipfs#4665](https://github.com/ipfs/go-ipfs/pull/4665))
  - Add godocs for unixfs module ([ipfs/go-ipfs#4664](https://github.com/ipfs/go-ipfs/pull/4664))
  - Add sharding to experimental features list ([ipfs/go-ipfs#4569](https://github.com/ipfs/go-ipfs/pull/4569))
  - Add godocs for routing module ([ipfs/go-ipfs#4676](https://github.com/ipfs/go-ipfs/pull/4676))
  - Add godocs for path module ([ipfs/go-ipfs#4689](https://github.com/ipfs/go-ipfs/pull/4689))
  - Add godocs for pin module ([ipfs/go-ipfs#4696](https://github.com/ipfs/go-ipfs/pull/4696))
  - Update link to filestore experimental status ([ipfs/go-ipfs#4557](https://github.com/ipfs/go-ipfs/pull/4557))
- Bugfixes
  - Remove trailing slash in ipfs get paths, fixes #3729 ([ipfs/go-ipfs#4365](https://github.com/ipfs/go-ipfs/pull/4365))
  - fix deadlock in bitswap sessions ([ipfs/go-ipfs#4407](https://github.com/ipfs/go-ipfs/pull/4407))
  - Fix two race conditions (and possibly go routine leaks) in commands ([ipfs/go-ipfs#4406](https://github.com/ipfs/go-ipfs/pull/4406))
  - Fix output delay in ipfs pubsub sub ([ipfs/go-ipfs#4402](https://github.com/ipfs/go-ipfs/pull/4402))
  - Use correct context in AddWithContext ([ipfs/go-ipfs#4433](https://github.com/ipfs/go-ipfs/pull/4433))
  - Fix various IPNS republisher issues ([ipfs/go-ipfs#4440](https://github.com/ipfs/go-ipfs/pull/4440))
  - Fix error handling in commands add and get ([ipfs/go-ipfs#4454](https://github.com/ipfs/go-ipfs/pull/4454))
  - Fix hamt (sharding) delete issue ([ipfs/go-ipfs#4398](https://github.com/ipfs/go-ipfs/pull/4398))
  - More correctly check for reuseport support ([libp2p/go-reuseport#40](https://github.com/libp2p/go-reuseport/pull/40))
  - Fix goroutine leak in websockets transport ([libp2p/go-ws-transport#21](https://github.com/libp2p/go-ws-transport/pull/21))
  - Update badgerds to fix i386 windows build ([ipfs/go-ipfs#4464](https://github.com/ipfs/go-ipfs/pull/4464))
  - Only construct bitswap event loggable if necessary ([ipfs/go-ipfs#4533](https://github.com/ipfs/go-ipfs/pull/4533))
  - Ensure that flush on the mfs root flushes its directory ([ipfs/go-ipfs#4509](https://github.com/ipfs/go-ipfs/pull/4509))
  - Fix deferred unlock of pin lock in AddR ([ipfs/go-ipfs#4562](https://github.com/ipfs/go-ipfs/pull/4562))
  - Fix iOS builds ([ipfs/go-ipfs#4610](https://github.com/ipfs/go-ipfs/pull/4610))
  - Calling repo gc now frees up space with badgerds ([ipfs/go-ipfs#4578](https://github.com/ipfs/go-ipfs/pull/4578))
  - Fix leak in bitswap sessions shutdown ([ipfs/go-ipfs#4658](https://github.com/ipfs/go-ipfs/pull/4658))
  - Fix make on windows ([ipfs/go-ipfs#4682](https://github.com/ipfs/go-ipfs/pull/4682))
  - Ignore invalid key files in keystore directory ([ipfs/go-ipfs#4700](https://github.com/ipfs/go-ipfs/pull/4700))
- General Changes and Refactorings
  - Extract and refactor commands library ([ipfs/go-ipfs#3856](https://github.com/ipfs/go-ipfs/pull/3856))
  - Remove all instances of `Default(false)` ([ipfs/go-ipfs#4042](https://github.com/ipfs/go-ipfs/pull/4042))
  - Build for all supported platforms when testing ([ipfs/go-ipfs#4445](https://github.com/ipfs/go-ipfs/pull/4445))
  - Refine gateway and namesys logging ([ipfs/go-ipfs#4428](https://github.com/ipfs/go-ipfs/pull/4428))
  - Demote bitswap error to an info ([ipfs/go-ipfs#4472](https://github.com/ipfs/go-ipfs/pull/4472))
  - Extract posinfo package to github.com/ipfs/go-ipfs-posinfo ([ipfs/go-ipfs#4669](https://github.com/ipfs/go-ipfs/pull/4669))
  - Move signature verification to ipns validator ([ipfs/go-ipfs#4628](https://github.com/ipfs/go-ipfs/pull/4628))
  - Extract importers/chunk module as go-ipfs-chunker ([ipfs/go-ipfs#4661](https://github.com/ipfs/go-ipfs/pull/4661))
  - Extract go-detect-race from Godeps ([ipfs/go-ipfs#4686](https://github.com/ipfs/go-ipfs/pull/4686))
  - Extract flags, delay, ds-help ([ipfs/go-ipfs#4685](https://github.com/ipfs/go-ipfs/pull/4685))
  - Extract routing package to go-ipfs-routing ([ipfs/go-ipfs#4703](https://github.com/ipfs/go-ipfs/pull/4703))
  - Extract blocks/blockstore package to go-ipfs-blockstore ([ipfs/go-ipfs#4707](https://github.com/ipfs/go-ipfs/pull/4707))
  - Add exchange.SessionExchange interface for exchanges that support sessions ([ipfs/go-ipfs#4709](https://github.com/ipfs/go-ipfs/pull/4709))
  - Extract thirdparty/pq to go-ipfs-pq ([ipfs/go-ipfs#4711](https://github.com/ipfs/go-ipfs/pull/4711))
  - Separate "path" from "path/resolver" ([ipfs/go-ipfs#4713](https://github.com/ipfs/go-ipfs/pull/4713))
- Testing
  - Increase verbosity of t0088-repo-stat-symlink.sh test ([ipfs/go-ipfs#4434](https://github.com/ipfs/go-ipfs/pull/4434))
  - Make repo size test pass deterministically ([ipfs/go-ipfs#4443](https://github.com/ipfs/go-ipfs/pull/4443))
  - Always set IPFS_PATH in test-lib.sh ([ipfs/go-ipfs#4469](https://github.com/ipfs/go-ipfs/pull/4469))
  - Fix sharness docker ([ipfs/go-ipfs#4489](https://github.com/ipfs/go-ipfs/pull/4489))
  - Fix loops in sharness tests to fail the test if the inner command fails ([ipfs/go-ipfs#4482](https://github.com/ipfs/go-ipfs/pull/4482))
  - Improve bitswap tests, fix race conditions ([ipfs/go-ipfs#4499](https://github.com/ipfs/go-ipfs/pull/4499))
  - Fix circleci cache directory list ([ipfs/go-ipfs#4564](https://github.com/ipfs/go-ipfs/pull/4564))
  - Only run the build test on test_go_expensive ([ipfs/go-ipfs#4645](https://github.com/ipfs/go-ipfs/pull/4645))
  - Fix go test on Windows ([ipfs/go-ipfs#4632](https://github.com/ipfs/go-ipfs/pull/4632))
  - Fix some tests on FreeBSD ([ipfs/go-ipfs#4662](https://github.com/ipfs/go-ipfs/pull/4662))

## 0.4.13 2017-11-16

Ipfs 0.4.13 is a patch release that fixes two high priority issues that were
discovered in the 0.4.12 release.

Bugfixes:
  - Fix periodic bitswap deadlock ([ipfs/go-ipfs#4386](https://github.com/ipfs/go-ipfs/pull/4386))
  - Fix badgerds crash on startup ([ipfs/go-ipfs#4384](https://github.com/ipfs/go-ipfs/pull/4384))


## 0.4.12 2017-11-09

Ipfs 0.4.12 brings with it many important fixes for the huge spike in network
size we've seen this past month. These changes include the Connection Manager,
faster batching in `ipfs add`, libp2p fixes that reduce CPU usage, and a bunch
of new documentation.

The most critical change is the 'Connection Manager': it allows an ipfs node to
maintain a limited set of connections to other peers in the network. By default
(and with no config changes required by the user), ipfs nodes will now try to
maintain between 600 and 900 open connections. These limits are still likely
higher than needed, and future releases may lower the default recommendation,
but for now we want to make changes gradually. The rationale for this selection
of numbers is as follows:

- The DHT routing table for a large network may rise to around 400 peers
- Bitswap connections tend to be separate from the DHT
- PubSub connections also generally are another distinct set of peers
  (including js-ipfs nodes)

Because of this, we selected 600 as a 'LowWater' number, and 900 as a
'HighWater' number to avoid having to clear out connections too frequently.
You can configure different numbers as you see fit via the `Swarm.ConnMgr`
field in your ipfs config file. See
[here](https://github.com/ipfs/go-ipfs/blob/master/docs/config.md#connmgr) for
more details.

Disk utilization during `ipfs add` has been optimized for large files by doing
batch writes in parallel. Previously, when adding a large file, users might have
noticed that the add progressed by about 8MB at a time, with brief pauses in between.
This was caused by quickly filling up the batch, then blocking while it was
writing to disk. We now write to disk in the background while continuing to add
the remainder of the file.

Other changes in this release have noticeably reduced memory consumption and CPU
usage. This was done by optimising some frequently called functions in libp2p
that were expensive in terms of both CPU usage and memory allocations. We also
lowered the yamux accept buffer sizes which were raised over a year ago to
combat a separate bug that has since been fixed.

And finally, thank you to everyone who filed bugs, tested out the release candidates,
filed pull requests, and contributed in any other way to this release!

- Features
  - Implement Connection Manager ([ipfs/go-ipfs#4288](https://github.com/ipfs/go-ipfs/pull/4288))
  - Support multiple files in dag put ([ipfs/go-ipfs#4254](https://github.com/ipfs/go-ipfs/pull/4254))
  - Add 'raw' support to the dag put command ([ipfs/go-ipfs#4285](https://github.com/ipfs/go-ipfs/pull/4285))
- Improvements
  - Parallelize dag batch flushing ([ipfs/go-ipfs#4296](https://github.com/ipfs/go-ipfs/pull/4296))
  - Update go-peerstream to improve CPU usage ([ipfs/go-ipfs#4323](https://github.com/ipfs/go-ipfs/pull/4323))
  - Add full support for CidV1 in Files API and Dag Modifier ([ipfs/go-ipfs#4026](https://github.com/ipfs/go-ipfs/pull/4026))
  - Lower yamux accept buffer size ([ipfs/go-ipfs#4326](https://github.com/ipfs/go-ipfs/pull/4326))
  - Optimise `ipfs pin update` command ([ipfs/go-ipfs#4348](https://github.com/ipfs/go-ipfs/pull/4348))
- Documentation
  - Add some docs on plugins ([ipfs/go-ipfs#4255](https://github.com/ipfs/go-ipfs/pull/4255))
  - Add more info about private network bootstrap ([ipfs/go-ipfs#4270](https://github.com/ipfs/go-ipfs/pull/4270))
  - Add more info about `ipfs add` chunker option ([ipfs/go-ipfs#4306](https://github.com/ipfs/go-ipfs/pull/4306))
  - Remove cruft in readme and mention discourse forum ([ipfs/go-ipfs#4345](https://github.com/ipfs/go-ipfs/pull/4345))
  - Add note about updating before reporting issues ([ipfs/go-ipfs#4361](https://github.com/ipfs/go-ipfs/pull/4361))
- Bugfixes
  - Fix FreeBSD build issues ([ipfs/go-ipfs#4275](https://github.com/ipfs/go-ipfs/pull/4275))
  - Don't crash when Datastore.StorageMax is not defined ([ipfs/go-ipfs#4246](https://github.com/ipfs/go-ipfs/pull/4246))
  - Do not call 'Connect' on NewStream in bitswap ([ipfs/go-ipfs#4317](https://github.com/ipfs/go-ipfs/pull/4317))
  - Filter out "" from active peers in bitswap sessions ([ipfs/go-ipfs#4316](https://github.com/ipfs/go-ipfs/pull/4316))
  - Fix "seeker can't seek" on specific files ([ipfs/go-ipfs#4320](https://github.com/ipfs/go-ipfs/pull/4320))
  - Do not set "gecos" field in Dockerfile ([ipfs/go-ipfs#4331](https://github.com/ipfs/go-ipfs/pull/4331))
  - Handle sym links in when calculating repo size ([ipfs/go-ipfs#4305](https://github.com/ipfs/go-ipfs/pull/4305))
- General Changes and Refactorings
  - Fix indent in sharness tests ([ipfs/go-ipfs#4212](https://github.com/ipfs/go-ipfs/pull/4212))
  - Remove supernode routing ([ipfs/go-ipfs#4302](https://github.com/ipfs/go-ipfs/pull/4302))
  - Extract go-ipfs-addr ([ipfs/go-ipfs#4340](https://github.com/ipfs/go-ipfs/pull/4340))
  - Remove dead code and config files ([ipfs/go-ipfs#4357](https://github.com/ipfs/go-ipfs/pull/4357))
  - Update badgerds to 1.0 ([ipfs/go-ipfs#4327](https://github.com/ipfs/go-ipfs/pull/4327))
  - Wrap help descriptions under 80 chars ([ipfs/go-ipfs#4121](https://github.com/ipfs/go-ipfs/pull/4121))
- Testing
  - Make sharness t0180-p2p less racy ([ipfs/go-ipfs#4310](https://github.com/ipfs/go-ipfs/pull/4310))


### 0.4.11 2017-09-14

Ipfs 0.4.11 is a larger release that brings many long-awaited features and
performance improvements. These include new datastore options, more efficient
bitswap transfers, greatly improved resource consumption, circuit relay
support, ipld plugins, and more! Take a look at the full changelog below for a
detailed list of every change.

The ipfs datastore has, until now, been a combination of leveldb and a custom
git-like storage backend called 'flatfs'. This works well enough for the
average user, but different ipfs usecases demand different backend
configurations. To address this, we have changed the configuration file format
for datastores to be a modular way of specifying exactly how you want the
datastore to be structured. You will now be able to configure ipfs to use
flatfs, leveldb, badger, an in-memory datastore, and more to suit your needs.
See the new [datastore
documentation](https://github.com/ipfs/go-ipfs/blob/master/docs/datastores.md)
for more information.

Bitswap received some much needed attention during this release cycle. The
concept of 'Bitswap Sessions' allows bitswap to associate requests for
different blocks to the same underlying session, and from that infer better
ways of requesting that data. In more concrete terms, parts of the ipfs
codebase that take advantage of sessions (currently, only `ipfs pin add`) will
cause much less extra traffic than before. This is done by making optimistic
guesses about which nodes might be providing given blocks and not sending
wantlist updates to every connected bitswap partner, as well as searching the
DHT for providers less frequently. In future releases we will migrate over more
ipfs commands to take advantage of bitswap sessions. As nodes update to this
and future versions, expect to see idle bandwidth usage on the ipfs network
go down noticeably.

The never ending effort to reduce resource consumption had a few important
updates this release. First, the bitswap sessions changes discussed above will
help with improving bandwidth usage. Aside from that there are two important
libp2p updates that improved things significantly. The first was a fix to a bug
in the dial limiter code that was causing it to not limit outgoing dials
correctly. This resulted in ipfs running out of file descriptors very
frequently (as well as incurring a decent amount of excess outgoing bandwidth),
this has now been fixed. Users who previously received "too many open files"
errors should see this much less often in 0.4.11. The second change was a
memory leak in the DHT that was identified and fixed. Streams being tracked in
a map in the DHT weren't being cleaned up after the peer disconnected leading
to the multiplexer session not being cleaned up properly. This issue has been
resolved, and now memory usage appears to be stable over time. There is still a
lot of work to be done improving memory usage, but we feel this is a solid
victory.

It is often said that NAT traversal is the hardest problem in peer to peer
technology, we tend to agree with this. In an effort to provide a more
ubiquitous p2p mesh, we have implemented a relay mechanism that allows willing
peers to relay traffic for other peers who might not otherwise be able to
communicate with each other.  This feature is still pretty early, and currently
users have to manually connect through a relay. The next step in this endeavour
is automatic relaying, and research for this is currently in progress. We
expect that when it lands, it will improve the perceived performance of ipfs by
spending less time attempting connections to hard to reach nodes. A short guide
on using the circuit relay feature can be found
[here](https://github.com/ipfs/go-ipfs/blob/master/docs/experimental-features.md#circuit-relay).

The last feature we want to highlight (but by no means the last feature in this
release) is our new plugin system. There are many different workflows and
usecases that ipfs should be able to support, but not everyone wants to be able
to use every feature. We could simply merge in all these features, but that
causes problems for several reasons: first off, the size of the ipfs binary
starts to get very large very quickly. Second, each of these different pieces
needs to be maintained and updated independently, which would cause significant
churn in the codebase. To address this, we have come up with a system that
allows users to install plugins to the vanilla ipfs daemon that augment its
capabilities. The first of these plugins are a [git
plugin](https://github.com/ipfs/go-ipfs/blob/master/plugin/plugins/git/git.go)
that allows ipfs to natively address git objects and an [ethereum
plugin](https://github.com/ipfs/go-ipld-eth) that lets ipfs ingest and operate
on all ethereum blockchain data. Soon to come are plugins for the bitcoin and
zcash data formats. In the future, we will be adding plugins for other things
like datastore backends and specialized libp2p network transports.
You can read more on this topic in [Plugin docs](docs/plugins.md)

In order to simplify its integration with fs-repo-migrations, we've switched
the ipfs/go-ipfs docker image from a musl base to a glibc base. For most users
this will not be noticeable, but if you've been building your own images based
off this image, you'll have to update your dockerfile. We recommend a
multi-stage dockerfile, where the build stage is based off of a regular Debian or
other glibc-based image, and the assembly stage is based off of the ipfs/go-ipfs
image, and you copy build artifacts from the build stage to the assembly
stage. Note, if you are using the docker image and see a deprecation message,
please update your usage. We will stop supporting the old method of starting
the dockerfile in the next release.

Finally, I would like to thank all of our contributors, users, supporters, and
friends for helping us along the way. Ipfs would not be where it is without
you.


- Features
  - Add `--pin` option to `ipfs dag put` ([ipfs/go-ipfs#4004](https://github.com/ipfs/go-ipfs/pull/4004))
  - Add `--pin` option to `ipfs object put` ([ipfs/go-ipfs#4095](https://github.com/ipfs/go-ipfs/pull/4095))
  - Implement `--profile` option on `ipfs init` ([ipfs/go-ipfs#4001](https://github.com/ipfs/go-ipfs/pull/4001))
  - Add CID Codecs to `ipfs block put` ([ipfs/go-ipfs#4022](https://github.com/ipfs/go-ipfs/pull/4022))
  - Bitswap sessions ([ipfs/go-ipfs#3867](https://github.com/ipfs/go-ipfs/pull/3867))
  - Create plugin API and loader, add ipld-git plugin ([ipfs/go-ipfs#4033](https://github.com/ipfs/go-ipfs/pull/4033))
  - Make announced swarm addresses configurable ([ipfs/go-ipfs#3948](https://github.com/ipfs/go-ipfs/pull/3948))
  - Reprovider strategies ([ipfs/go-ipfs#4113](https://github.com/ipfs/go-ipfs/pull/4113))
  - Circuit Relay integration ([ipfs/go-ipfs#4091](https://github.com/ipfs/go-ipfs/pull/4091))
  - More configurable datastore configs ([ipfs/go-ipfs#3575](https://github.com/ipfs/go-ipfs/pull/3575))
  - Add experimental support for badger datastore ([ipfs/go-ipfs#4007](https://github.com/ipfs/go-ipfs/pull/4007))
- Improvements
  - Add better support for Raw Nodes in MFS and elsewhere ([ipfs/go-ipfs#3996](https://github.com/ipfs/go-ipfs/pull/3996))
  - Added file size to response of `ipfs add` command ([ipfs/go-ipfs#4082](https://github.com/ipfs/go-ipfs/pull/4082))
  - Add /dnsaddr bootstrap nodes ([ipfs/go-ipfs#4127](https://github.com/ipfs/go-ipfs/pull/4127))
  - Do not publish public keys extractable from ID ([ipfs/go-ipfs#4020](https://github.com/ipfs/go-ipfs/pull/4020))
- Documentation
  - Adding documentation that PubSub Sub can be encoded. ([ipfs/go-ipfs#3909](https://github.com/ipfs/go-ipfs/pull/3909))
  - Add Comms items from js-ipfs, including blog ([ipfs/go-ipfs#3936](https://github.com/ipfs/go-ipfs/pull/3936))
  - Add Developer Certificate of Origin ([ipfs/go-ipfs#4006](https://github.com/ipfs/go-ipfs/pull/4006))
  - Add `transports.md` document ([ipfs/go-ipfs#4034](https://github.com/ipfs/go-ipfs/pull/4034))
  - Add `experimental-features.md` document ([ipfs/go-ipfs#4036](https://github.com/ipfs/go-ipfs/pull/4036))
  - Update release docs ([ipfs/go-ipfs#4165](https://github.com/ipfs/go-ipfs/pull/4165))
  - Add documentation for datastore configs ([ipfs/go-ipfs#4223](https://github.com/ipfs/go-ipfs/pull/4223))
  - General update and clean-up of docs ([ipfs/go-ipfs#4222](https://github.com/ipfs/go-ipfs/pull/4222))
- Bugfixes
  - Fix shutdown check in t0023 ([ipfs/go-ipfs#3969](https://github.com/ipfs/go-ipfs/pull/3969))
  - Fix pinning of unixfs sharded directories ([ipfs/go-ipfs#3975](https://github.com/ipfs/go-ipfs/pull/3975))
  - Show escaped url in gateway 404 message ([ipfs/go-ipfs#4005](https://github.com/ipfs/go-ipfs/pull/4005))
  - Fix early opening of bitswap message sender ([ipfs/go-ipfs#4069](https://github.com/ipfs/go-ipfs/pull/4069))
  - Fix determination of 'root' node in dag put ([ipfs/go-ipfs#4072](https://github.com/ipfs/go-ipfs/pull/4072))
  - Fix bad multipart message panic in gateway ([ipfs/go-ipfs#4053](https://github.com/ipfs/go-ipfs/pull/4053))
  - Add blocks to the blockstore before returning them from blockservice sessions ([ipfs/go-ipfs#4169](https://github.com/ipfs/go-ipfs/pull/4169))
  - Various fixes for /ipfs fuse code ([ipfs/go-ipfs#4194](https://github.com/ipfs/go-ipfs/pull/4194))
  - Fix memory leak in dht stream tracking ([ipfs/go-ipfs#4251](https://github.com/ipfs/go-ipfs/pull/4251))
- General Changes and Refactorings
  - Require go 1.8 ([ipfs/go-ipfs#4044](https://github.com/ipfs/go-ipfs/pull/4044))
  - Change IPFS to use the new pluggable Block to IPLD decoding framework. ([ipfs/go-ipfs#4060](https://github.com/ipfs/go-ipfs/pull/4060))
  - Remove tour command from ipfs ([ipfs/go-ipfs#4123](https://github.com/ipfs/go-ipfs/pull/4123))
  - Add support for Go 1.9 ([ipfs/go-ipfs#4156](https://github.com/ipfs/go-ipfs/pull/4156))
  - Remove some dead code ([ipfs/go-ipfs#4204](https://github.com/ipfs/go-ipfs/pull/4204))
  - Switch docker image from musl to glibc ([ipfs/go-ipfs#4219](https://github.com/ipfs/go-ipfs/pull/4219))

### 0.4.10 - 2017-06-27

Ipfs 0.4.10 is a patch release that contains several exciting new features,
bugfixes and general improvements. Including new commands, easier corruption
recovery, and a generally cleaner codebase.

The `ipfs pin` command has two new subcommands, `verify` and `update`. `ipfs
pin verify` is used to scan the repo for pinned object graphs and check their
integrity. Any issues are reported back with helpful error text to make error
recovery simpler.  This subcommand was added to help recover from datastore
corruptions, particularly if using the experimental filestore and accidentally
deleting tracked files.
`ipfs pin update` was added to make the task of keeping a large, frequently
changing object graph pinned. Previously users had to call `ipfs pin rm` on the
old pin, and `ipfs pin add` on the new one. The 'new' `ipfs pin add` call would
be very expensive as it would need to verify the entirety of the graph again.
The `ipfs pin update` command takes shortcuts, portions of the graph that were
covered under the old pin are assumed to be fine, and the command skips
checking them.

Next up, we have finally implemented an `ipfs shutdown` command so users can
shut down their ipfs daemons via the API. This is especially useful on
platforms that make it difficult to control processes (Android, for example),
and is also useful when needing to shut down a node remotely and you do not
have access to the machine itself.

`ipfs add` has gained a new flag; the `--hash` flag allows you to select which
hash function to use and we have given it the ability to select `blake2b-256`.
This pushes us one step closer to shifting over to using blake2b as the
default. Blake2b is significantly faster than sha2-256, and also is conjectured
to provide superior security.

We have also finally implemented a very early (and experimental) `ipfs p2p`.
This command and its subcommands will allow you to open up arbitrary streams to
other ipfs peers through libp2p. The interfaces are a little bit clunky right
now, but shouldn't get in the way of anyone wanting to try building a fully
peer to peer application on top of ipfs and libp2p. For more info on this
command, to ask questions, or to provide feedback, head over to the [feedback
issue](https://github.com/ipfs/go-ipfs/issues/3994) for the command.

A few other subcommands and flags were added around the API, as well as many
other requested improvements. See below for the full list of changes.


- Features
  - Add support for specifying the hash function in `ipfs add` ([ipfs/go-ipfs#3919](https://github.com/ipfs/go-ipfs/pull/3919))
  - Implement `ipfs key {rm, rename}` ([ipfs/go-ipfs#3892](https://github.com/ipfs/go-ipfs/pull/3892))
  - Implement `ipfs shutdown` command ([ipfs/go-ipfs#3884](https://github.com/ipfs/go-ipfs/pull/3884))
  - Implement `ipfs pin update` ([ipfs/go-ipfs#3846](https://github.com/ipfs/go-ipfs/pull/3846))
  - Implement `ipfs pin verify` ([ipfs/go-ipfs#3843](https://github.com/ipfs/go-ipfs/pull/3843))
  - Implemented experimental p2p commands ([ipfs/go-ipfs#3943](https://github.com/ipfs/go-ipfs/pull/3943))
- Improvements
  - Add MaxStorage field to output of "repo stat" ([ipfs/go-ipfs#3915](https://github.com/ipfs/go-ipfs/pull/3915))
  - Add Suborigin header to gateway responses ([ipfs/go-ipfs#3914](https://github.com/ipfs/go-ipfs/pull/3914))
  - Add "--file-order" option to "filestore ls" and "verify" ([ipfs/go-ipfs#3938](https://github.com/ipfs/go-ipfs/pull/3938))
  - Allow selecting ipns keys by Peer ID ([ipfs/go-ipfs#3882](https://github.com/ipfs/go-ipfs/pull/3882))
  - Don't redirect to trailing slash in gateway for `go get` ([ipfs/go-ipfs#3963](https://github.com/ipfs/go-ipfs/pull/3963))
  - Add 'ipfs dht findprovs --num-providers' to allow choosing number of providers to find ([ipfs/go-ipfs#3966](https://github.com/ipfs/go-ipfs/pull/3966))
  - Make sure all keystore keys get republished ([ipfs/go-ipfs#3951](https://github.com/ipfs/go-ipfs/pull/3951))
- Documentation
  - Adding documentation on PubSub encodings ([ipfs/go-ipfs#3909](https://github.com/ipfs/go-ipfs/pull/3909))
  - Change 'neccessary' to 'necessary' ([ipfs/go-ipfs#3941](https://github.com/ipfs/go-ipfs/pull/3941))
  - README.md: add Nix to the linux package managers ([ipfs/go-ipfs#3939](https://github.com/ipfs/go-ipfs/pull/3939))
  - More verbose errors in filestore ([ipfs/go-ipfs#3964](https://github.com/ipfs/go-ipfs/pull/3964))
- Bugfixes
  - Fix typo in message when file size check fails ([ipfs/go-ipfs#3895](https://github.com/ipfs/go-ipfs/pull/3895))
  - Clean up bitswap ledgers when disconnecting ([ipfs/go-ipfs#3437](https://github.com/ipfs/go-ipfs/pull/3437))
  - Make odds of 'process added after close' panic less likely ([ipfs/go-ipfs#3940](https://github.com/ipfs/go-ipfs/pull/3940))
- General Changes and Refactorings
  - Remove 'ipfs diag net' from codebase ([ipfs/go-ipfs#3916](https://github.com/ipfs/go-ipfs/pull/3916))
  - Update to dht code with provide announce option ([ipfs/go-ipfs#3928](https://github.com/ipfs/go-ipfs/pull/3928))
  - Apply the megacheck code vetting tool ([ipfs/go-ipfs#3949](https://github.com/ipfs/go-ipfs/pull/3949))
  - Expose port 8081 in docker container for /ws listener ([ipfs/go-ipfs#3954](https://github.com/ipfs/go-ipfs/pull/3954))

### 0.4.9 - 2017-04-30

Ipfs 0.4.9 is a maintenance release that contains several useful bugfixes and
improvements. Notably, `ipfs add` has gained the ability to select which CID
version will be output. The common ipfs hash that looks like this:
`QmRjNgF2mRLDT8AzCPsQbw1EYF2hDTFgfUmJokJPhCApYP` is a multihash. Multihashes
allow us to specify the hashing algorithm that was used to verify the data, but
it doesn't give us any indication of what format that data might be. To address
that issue, we are adding another couple of bytes to the prefix that will allow us
to indicate the format of the data referenced by the hash. This new format is
called a Content ID, or CID for short. The previous bare multihashes will still
be fully supported throughout the entire application as CID version 0. The new
format with the type information will be CID version 1. To give an example,
the content referenced by the hash above is "Hello Ipfs!". That same content,
in the same format (dag-protobuf) using CIDv1 is
`zb2rhkgXZVkT2xvDiuUsJENPSbWJy7fdYnsboLBzzEjjZMRoG`.

CIDv1 hashes are supported in ipfs versions back to 0.4.5. Nodes running 0.4.4
and older will not be able to load content via CIDv1 and we recommend that they
update to a newer version.

There are many other use cases for CIDs. Plugins can be written to
allow ipfs to natively address content from any other merkletree based system,
such as git, bitcoin, zcash and ethereum -- a few systems we've already started work on.

Aside from the CID flag, there were many other changes as noted below:

- Features
  - Add support for using CidV1 in 'ipfs add' ([ipfs/go-ipfs#3743](https://github.com/ipfs/go-ipfs/pull/3743))
- Improvements
  - Use CID as an ETag strong validator ([ipfs/go-ipfs#3869](https://github.com/ipfs/go-ipfs/pull/3869))
  - Update go-multihash with keccak and bitcoin hashes ([ipfs/go-ipfs#3833](https://github.com/ipfs/go-ipfs/pull/3833))
  - Update go-is-domain to contain new gTLD ([ipfs/go-ipfs#3873](https://github.com/ipfs/go-ipfs/pull/3873))
  - Periodically flush cached directories during ipfs add ([ipfs/go-ipfs#3888](https://github.com/ipfs/go-ipfs/pull/3888))
  - improved gateway directory listing for sharded nodes ([ipfs/go-ipfs#3897](https://github.com/ipfs/go-ipfs/pull/3897))
- Documentation
  - Change issue template to use Severity instead of Priority ([ipfs/go-ipfs#3834](https://github.com/ipfs/go-ipfs/pull/3834))
  - Fix link to commit hook script in contribute.md ([ipfs/go-ipfs#3863](https://github.com/ipfs/go-ipfs/pull/3863))
  - Fix install_unsupported for openbsd, add docs ([ipfs/go-ipfs#3880](https://github.com/ipfs/go-ipfs/pull/3880))
- Bugfixes
  - Fix wanlist typo in prometheus metric name ([ipfs/go-ipfs#3841](https://github.com/ipfs/go-ipfs/pull/3841))
  - Fix `make install` not using ldflags for git hash ([ipfs/go-ipfs#3838](https://github.com/ipfs/go-ipfs/pull/3838))
  - Fix `make install` not installing dependencies ([ipfs/go-ipfs#3848](https://github.com/ipfs/go-ipfs/pull/3848))
  - Fix erroneous Cache-Control: immutable on dir listings ([ipfs/go-ipfs#3870](https://github.com/ipfs/go-ipfs/pull/3870))
  - Fix bitswap accounting of 'BytesSent' in ledger ([ipfs/go-ipfs#3876](https://github.com/ipfs/go-ipfs/pull/3876))
  - Fix gateway handling of sharded directories ([ipfs/go-ipfs#3889](https://github.com/ipfs/go-ipfs/pull/3889))
  - Fix sharding memory growth, and fix resolver for unixfs paths ([ipfs/go-ipfs#3890](https://github.com/ipfs/go-ipfs/pull/3890))
- General Changes and Refactorings
  - Use ctx var consistently in daemon.go ([ipfs/go-ipfs#3864](https://github.com/ipfs/go-ipfs/pull/3864))
  - Handle 404 correctly in dist_get tool ([ipfs/go-ipfs#3879](https://github.com/ipfs/go-ipfs/pull/3879))
- Testing
  - Fix go fuse tests ([ipfs/go-ipfs#3840](https://github.com/ipfs/go-ipfs/pull/3840))

### 0.4.8 - 2017-03-29

Ipfs 0.4.8 brings with it several improvements, bugfixes, documentation
improvements, and the long awaited directory sharding code.

Currently, when too many items are added into a unixfs directory, the object
gets too large and you may experience issues. To pervent this problem, and
generally make working really large directories more efficient, we have
implemented a HAMT structure for unixfs. To enable this feature, run:
```
ipfs config --json Experimental.ShardingEnabled true
```

And restart your daemon if it was running.

Note: With this setting enabled, the hashes of any newly added directories will
be different than they previously were, as the new code will use the sharded
HAMT structure for all directories. Also, nodes running ipfs 0.4.7 and earlier
will not be able to access directories created with this option.

That said, please do give it a try, let us know how it goes, and then take a
look at all the other cool things added in 0.4.8 below.

- Features
	- Implement unixfs directory sharding ([ipfs/go-ipfs#3042](https://github.com/ipfs/go-ipfs/pull/3042))
	- Add DisableNatPortMap option ([ipfs/go-ipfs#3798](https://github.com/ipfs/go-ipfs/pull/3798))
	- Basic Filestore utilty commands ([ipfs/go-ipfs#3653](https://github.com/ipfs/go-ipfs/pull/3653))
- Improvements
	- More Robust GC ([ipfs/go-ipfs#3712](https://github.com/ipfs/go-ipfs/pull/3712))
	- Automatically fix permissions for docker volumes ([ipfs/go-ipfs#3744](https://github.com/ipfs/go-ipfs/pull/3744))
	- Core API refinements and efficiency improvements ([ipfs/go-ipfs#3493](https://github.com/ipfs/go-ipfs/pull/3493))
	- Improve IsPinned() lookups for indirect pins ([ipfs/go-ipfs#3809](https://github.com/ipfs/go-ipfs/pull/3809))
- Documentation
	- Improve 'name' and 'key' helptexts ([ipfs/go-ipfs#3806](https://github.com/ipfs/go-ipfs/pull/3806))
	- Update link to paper in dev.md ([ipfs/go-ipfs#3812](https://github.com/ipfs/go-ipfs/pull/3812))
	- Add test to enforce helptext on commands ([ipfs/go-ipfs#2648](https://github.com/ipfs/go-ipfs/pull/2648))
- Bugfixes
	- Remove bloom filter check on Put call in blockstore ([ipfs/go-ipfs#3782](https://github.com/ipfs/go-ipfs/pull/3782))
	- Re-add the GOPATH checking functionality ([ipfs/go-ipfs#3787](https://github.com/ipfs/go-ipfs/pull/3787))
	- Use fsrepo.IsInitialized to test for initialization ([ipfs/go-ipfs#3805](https://github.com/ipfs/go-ipfs/pull/3805))
	- Return 404 Not Found for failed path resolutions ([ipfs/go-ipfs#3777](https://github.com/ipfs/go-ipfs/pull/3777))
	- Fix 'dist\_get' failing without failing ([ipfs/go-ipfs#3818](https://github.com/ipfs/go-ipfs/pull/3818))
	- Update iptb with fix for t0130 hanging issue ([ipfs/go-ipfs#3823](https://github.com/ipfs/go-ipfs/pull/3823))
	- fix hidden file detection on windows ([ipfs/go-ipfs#3829](https://github.com/ipfs/go-ipfs/pull/3829))
- General Changes and Refactorings
	- Fix multiple govet warnings ([ipfs/go-ipfs#3824](https://github.com/ipfs/go-ipfs/pull/3824))
	- Make Golint happy in the blocks submodule ([ipfs/go-ipfs#3827](https://github.com/ipfs/go-ipfs/pull/3827))
- Testing
	- Enable codeclimate for automated linting and vetting ([ipfs/go-ipfs#3821](https://github.com/ipfs/go-ipfs/pull/3821))
	- Fix EOF test failure with Multipart.Read ([ipfs/go-ipfs#3804](https://github.com/ipfs/go-ipfs/pull/3804))

### 0.4.7 - 2017-03-15

Ipfs 0.4.7 contains several exciting new features!
First off, The long awaited filestore feature has been merged, allowing users
the option to not have ipfs store chunked copies of added files in the
blockstore, pushing to burden of ensuring those files are not changed to the
user. The filestore feature is currently still experimental, and must be
enabled in your config with:
```
ipfs config --json Experimental.FilestoreEnabled true
```
before it can be used. Please see [this issue](https://github.com/ipfs/go-ipfs/issues/3397#issuecomment-284337564) for more details.

Next up, We have merged initial support for ipfs 'Private Networks'. This
feature allows users to run ipfs in a mode that will only connect to other
peers in the private network. This feature, like the filestore is being
released experimentally, but if you're interested please try it out.
Instructions for setting it up can be found
[here](https://github.com/ipfs/go-ipfs/issues/3397#issuecomment-284341649).

This release also enables support for the 'mplex' stream muxer by default. This
stream multiplexing protocol was available previously via the
`--enable-mplex-experiment` daemon flag, but has now graduated to being 'less
experimental' and no longer requires the flag to use it.

Aside from those, we have a good number of bugfixes, perf improvements and new
tests. Heres a list of highlights:

- Features
	- Implement basic filestore 'no-copy' functionality ([ipfs/go-ipfs#3629](https://github.com/ipfs/go-ipfs/pull/3629))
	- Add support for private ipfs networks ([ipfs/go-ipfs#3697](https://github.com/ipfs/go-ipfs/pull/3697))
	- Enable 'mplex' stream muxer by default ([ipfs/go-ipfs#3725](https://github.com/ipfs/go-ipfs/pull/3725))
	- Add `--quieter` option to `ipfs add` ([ipfs/go-ipfs#3770](https://github.com/ipfs/go-ipfs/pull/3770))
	- Report progress during `pin add` via `--progress` ([ipfs/go-ipfs#3671](https://github.com/ipfs/go-ipfs/pull/3671))
- Improvements
	- Allow `ipfs get` to handle content added with raw leaves option ([ipfs/go-ipfs#3757](https://github.com/ipfs/go-ipfs/pull/3757))
	- Fix accuracy of progress bar on `ipfs get` ([ipfs/go-ipfs#3758](https://github.com/ipfs/go-ipfs/pull/3758))
	- Limit number of objects in batches to prevent too many fds issue ([ipfs/go-ipfs#3756](https://github.com/ipfs/go-ipfs/pull/3756))
	- Add more info to bitswap stat ([ipfs/go-ipfs#3635](https://github.com/ipfs/go-ipfs/pull/3635))
	- Add multiple performance metrics ([ipfs/go-ipfs#3615](https://github.com/ipfs/go-ipfs/pull/3615))
	- Make `dist_get` fall back to other downloaders if one fails ([ipfs/go-ipfs#3692](https://github.com/ipfs/go-ipfs/pull/3692))
- Documentation
	- Add Arch Linux install instructions to readme ([ipfs/go-ipfs#3742](https://github.com/ipfs/go-ipfs/pull/3742))
	- Improve release checklist document ([ipfs/go-ipfs#3717](https://github.com/ipfs/go-ipfs/pull/3717))
- Bugfixes
	- Fix drive root parsing on windows ([ipfs/go-ipfs#3328](https://github.com/ipfs/go-ipfs/pull/3328))
	- Fix panic in ipfs get when passing no parameters to API ([ipfs/go-ipfs#3768](https://github.com/ipfs/go-ipfs/pull/3768))
	- Fix breakage of `ipfs pin add` api output ([ipfs/go-ipfs#3760](https://github.com/ipfs/go-ipfs/pull/3760))
	- Fix issue in DHT queries that was causing poor record replication ([ipfs/go-ipfs#3748](https://github.com/ipfs/go-ipfs/pull/3748))
	- Fix `ipfs mount` crashing if no name was published before ([ipfs/go-ipfs#3728](https://github.com/ipfs/go-ipfs/pull/3728))
	- Add `self` key to the `ipfs key list` listing ([ipfs/go-ipfs#3734](https://github.com/ipfs/go-ipfs/pull/3734))
	- Fix panic when shutting down `ipfs daemon` pre gateway setup ([ipfs/go-ipfs#3723](https://github.com/ipfs/go-ipfs/pull/3723))
- General Changes and Refactorings
	- Refactor `EnumerateChildren` to avoid need for bestEffort parameter ([ipfs/go-ipfs#3700](https://github.com/ipfs/go-ipfs/pull/3700))
	- Update fuse dependency, fixing several issues ([ipfs/go-ipfs#3727](https://github.com/ipfs/go-ipfs/pull/3727))
	- Add `install_unsupported` makefile target for 'exotic' systems ([ipfs/go-ipfs#3719](https://github.com/ipfs/go-ipfs/pull/3719))
	- Deprecate implicit daemon argument in Dockerfile ([ipfs/go-ipfs#3685](https://github.com/ipfs/go-ipfs/pull/3685))
- Testing
	- Add test to ensure helptext is under 80 columns wide ([ipfs/go-ipfs#3774](https://github.com/ipfs/go-ipfs/pull/3774))
	- Add unit tests for auto migration code ([ipfs/go-ipfs#3618](https://github.com/ipfs/go-ipfs/pull/3618))
	- Fix iptb stop issue in sharness tests  ([ipfs/go-ipfs#3714](https://github.com/ipfs/go-ipfs/pull/3714))


### 0.4.6 - 2017-02-21

Ipfs 0.4.6 contains several bugfixes related to migrations and also contains a
few other improvements to other parts of the codebase. Notably:

- The default config will now contain some ipv6 addresses for bootstrap nodes.
- `ipfs pin add` should be faster and consume less memory.
- Pinning thousands of files no longer causes superlinear usage of storage space.

- Improvements
	- Make pinset sharding deterministic ([ipfs/go-ipfs#3640](https://github.com/ipfs/go-ipfs/pull/3640))
	- Update to go-multihash with blake2 ([ipfs/go-ipfs#3649](https://github.com/ipfs/go-ipfs/pull/3649))
	- Pass cids instead of nodes around in EnumerateChildrenAsync ([ipfs/go-ipfs#3598](https://github.com/ipfs/go-ipfs/pull/3598))
	- Add /ip6 bootstrap nodes ([ipfs/go-ipfs#3523](https://github.com/ipfs/go-ipfs/pull/3523))
	- Add sub-object support to `dag get` command ([ipfs/go-ipfs#3687](https://github.com/ipfs/go-ipfs/pull/3687))
	- Add half-closed streams support to multiplex experiment ([ipfs/go-ipfs#3695](https://github.com/ipfs/go-ipfs/pull/3695))
- Documentation
	- Add the snap installation instructions ([ipfs/go-ipfs#3663](https://github.com/ipfs/go-ipfs/pull/3663))
	- Add closed PRs, Issues throughput ([ipfs/go-ipfs#3602](https://github.com/ipfs/go-ipfs/pull/3602))
- Bugfixes
	- Fix auto-migration on docker nodes ([ipfs/go-ipfs#3698](https://github.com/ipfs/go-ipfs/pull/3698))
	- Update flatfs to v1.1.2, fixing directory fd issue ([ipfs/go-ipfs#3711](https://github.com/ipfs/go-ipfs/pull/3711))
- General Changes and Refactorings
	- Remove `FindProviders` from routing mocks ([ipfs/go-ipfs#3617](https://github.com/ipfs/go-ipfs/pull/3617))
	- Use Marshalers instead of PostRun to process `block rm` output ([ipfs/go-ipfs#3708](https://github.com/ipfs/go-ipfs/pull/3708))
- Testing
	- Makefile rework and sharness test coverage ([ipfs/go-ipfs#3504](https://github.com/ipfs/go-ipfs/pull/3504))
	- Print out all daemon stderr files when iptb stop fails ([ipfs/go-ipfs#3701](https://github.com/ipfs/go-ipfs/pull/3701))
	- Add tests for recursively pinning a dag ([ipfs/go-ipfs#3691](https://github.com/ipfs/go-ipfs/pull/3691))
	- Fix lack of commit hash during build ([ipfs/go-ipfs#3705](https://github.com/ipfs/go-ipfs/pull/3705))

### 0.4.5 - 2017-02-11

#### Changes from rc3 to rc4
- Update to fixed webui. ([ipfs/go-ipfs#3669](https://github.com/ipfs/go-ipfs/pull/3669))

#### Changes from rc2 to rc3
- Fix handling of null arrays in cbor ipld objects.  ([ipfs/go-ipfs#3666](https://github.com/ipfs/go-ipfs/pull/3666))
- Add env var to enable yamux debug logging.  ([ipfs/go-ipfs#3668](https://github.com/ipfs/go-ipfs/pull/3668))
- Fix libc check during auto-migrations.  ([ipfs/go-ipfs#3665](https://github.com/ipfs/go-ipfs/pull/3665))

#### Changes from rc1 to rc2
- Fixed json output of ipld objects in `ipfs dag get` ([ipfs/go-ipfs#3655](https://github.com/ipfs/go-ipfs/pull/3655))

#### Changes since 0.4.4

- Notable changes
	- IPLD and CIDs
	  - Rework go-ipfs to use Content IDs  ([ipfs/go-ipfs#3187](https://github.com/ipfs/go-ipfs/pull/3187))  ([ipfs/go-ipfs#3290](https://github.com/ipfs/go-ipfs/pull/3290))
	  - Turn merkledag.Node into an interface ([ipfs/go-ipfs#3301](https://github.com/ipfs/go-ipfs/pull/3301))
	  - Implement cbor ipld nodes  ([ipfs/go-ipfs#3325](https://github.com/ipfs/go-ipfs/pull/3325))
	  - Allow cid format selection in block put command  ([ipfs/go-ipfs#3324](https://github.com/ipfs/go-ipfs/pull/3324))  ([ipfs/go-ipfs#3483](https://github.com/ipfs/go-ipfs/pull/3483))
	  - Bitswap protocol extension to handle cids  ([ipfs/go-ipfs#3297](https://github.com/ipfs/go-ipfs/pull/3297))
	  - Add dag get to read-only api  ([ipfs/go-ipfs#3499](https://github.com/ipfs/go-ipfs/pull/3499))
	- Raw Nodes
	  - Implement 'Raw Node' node type for addressing raw data  ([ipfs/go-ipfs#3307](https://github.com/ipfs/go-ipfs/pull/3307))
	  - Optimize DagService GetLinks for Raw Nodes.  ([ipfs/go-ipfs#3351](https://github.com/ipfs/go-ipfs/pull/3351))
	- Experimental PubSub
	  - Added a very basic pubsub implementation  ([ipfs/go-ipfs#3202](https://github.com/ipfs/go-ipfs/pull/3202))
	- Core API
	  - gateway: use core api for serving GET/HEAD/POST  ([ipfs/go-ipfs#3244](https://github.com/ipfs/go-ipfs/pull/3244))

- Improvements
	- Disable auto-gc check in 'ipfs cat'  ([ipfs/go-ipfs#3100](https://github.com/ipfs/go-ipfs/pull/3100))
	- Add `bitswap ledger` command  ([ipfs/go-ipfs#2852](https://github.com/ipfs/go-ipfs/pull/2852))
	- Add `ipfs block rm` command.  ([ipfs/go-ipfs#2962](https://github.com/ipfs/go-ipfs/pull/2962))
	- Add config option to disable bandwidth metrics   ([ipfs/go-ipfs#3381](https://github.com/ipfs/go-ipfs/pull/3381))
	- Add experimental dht 'client mode' flag  ([ipfs/go-ipfs#3269](https://github.com/ipfs/go-ipfs/pull/3269))
	- Add config option to set reprovider interval  ([ipfs/go-ipfs#3101](https://github.com/ipfs/go-ipfs/pull/3101))
	- Add `ipfs dht provide` command  ([ipfs/go-ipfs#3106](https://github.com/ipfs/go-ipfs/pull/3106))
	- Add stream info to `ipfs swarm peers -v`  ([ipfs/go-ipfs#3352](https://github.com/ipfs/go-ipfs/pull/3352))
	- Add option to enable go-multiplex experiment  ([ipfs/go-ipfs#3447](https://github.com/ipfs/go-ipfs/pull/3447))
	- Basic Keystore implementation  ([ipfs/go-ipfs#3472](https://github.com/ipfs/go-ipfs/pull/3472))
	- Make `ipfs add --local` not send providers messages  ([ipfs/go-ipfs#3102](https://github.com/ipfs/go-ipfs/pull/3102))
	- Fix bug in `ipfs tar add` that buffered input in memory  ([ipfs/go-ipfs#3334](https://github.com/ipfs/go-ipfs/pull/3334))
	- Make blockstore retry operations on temporary errors  ([ipfs/go-ipfs#3091](https://github.com/ipfs/go-ipfs/pull/3091))
	- Don't hold the PinLock in adder when not pinning.  ([ipfs/go-ipfs#3222](https://github.com/ipfs/go-ipfs/pull/3222))
	- Validate repo/api file and improve error message  ([ipfs/go-ipfs#3219](https://github.com/ipfs/go-ipfs/pull/3219))
	- no longer hard code gomaxprocs  ([ipfs/go-ipfs#3357](https://github.com/ipfs/go-ipfs/pull/3357))
	- Updated Bash complete script  ([ipfs/go-ipfs#3377](https://github.com/ipfs/go-ipfs/pull/3377))
	- Remove expensive debug statement in blockstore AllKeysChan  ([ipfs/go-ipfs#3384](https://github.com/ipfs/go-ipfs/pull/3384))
	- Remove GC timeout, fix GC tests  ([ipfs/go-ipfs#3494](https://github.com/ipfs/go-ipfs/pull/3494))
	- Fix `ipfs pin add` resource consumption  ([ipfs/go-ipfs#3495](https://github.com/ipfs/go-ipfs/pull/3495))  ([ipfs/go-ipfs#3571](https://github.com/ipfs/go-ipfs/pull/3571))
	- Add IPNS entry to DHT cache after publish  ([ipfs/go-ipfs#3501](https://github.com/ipfs/go-ipfs/pull/3501))
	- Add in `--routing=none` daemon option  ([ipfs/go-ipfs#3605](https://github.com/ipfs/go-ipfs/pull/3605))

- Bitswap
	- Don't re-provide blocks we've provided very recently  ([ipfs/go-ipfs#3105](https://github.com/ipfs/go-ipfs/pull/3105))
	- Add a deadline to sendmsg calls ([ipfs/go-ipfs#3445](https://github.com/ipfs/go-ipfs/pull/3445))
	- cleanup bitswap and handle message send failure slightly better  ([ipfs/go-ipfs#3408](https://github.com/ipfs/go-ipfs/pull/3408))
	- Increase wantlist resend delay to one minute  ([ipfs/go-ipfs#3448](https://github.com/ipfs/go-ipfs/pull/3448))
	- Fix issue where wantlist fullness wasn't included in messages  ([ipfs/go-ipfs#3461](https://github.com/ipfs/go-ipfs/pull/3461))
	- Only pass keys down newBlocks chan in bitswap   ([ipfs/go-ipfs#3271](https://github.com/ipfs/go-ipfs/pull/3271))

- Bugfixes
	- gateway: fix --writable flag  ([ipfs/go-ipfs#3206](https://github.com/ipfs/go-ipfs/pull/3206))
	- Fix relative seek in unixfs not expanding file properly   ([ipfs/go-ipfs#3095](https://github.com/ipfs/go-ipfs/pull/3095))
	- Update multicodec service names for ipfs services  ([ipfs/go-ipfs#3132](https://github.com/ipfs/go-ipfs/pull/3132))
	- dht: add missing protocol ID to newStream call  ([ipfs/go-ipfs#3203](https://github.com/ipfs/go-ipfs/pull/3203))
	- Return immediately on namesys error  ([ipfs/go-ipfs#3345](https://github.com/ipfs/go-ipfs/pull/3345))
	- Improve osxfuse handling  ([ipfs/go-ipfs#3098](https://github.com/ipfs/go-ipfs/pull/3098))  ([ipfs/go-ipfs#3413](https://github.com/ipfs/go-ipfs/pull/3413))
	- commands: fix opt.Description panic when desc was empty  ([ipfs/go-ipfs#3521](https://github.com/ipfs/go-ipfs/pull/3521))
	- Fixes #3133: Properly handle release candidates in version comparison  ([ipfs/go-ipfs#3136](https://github.com/ipfs/go-ipfs/pull/3136))
	- Don't drop error in readStreamedJson.  ([ipfs/go-ipfs#3276](https://github.com/ipfs/go-ipfs/pull/3276))
	- Error out on invalid `--routing` option  ([ipfs/go-ipfs#3482](https://github.com/ipfs/go-ipfs/pull/3482))
	- Respect contexts when returning diagnostics responses  ([ipfs/go-ipfs#3353](https://github.com/ipfs/go-ipfs/pull/3353))
	- Fix json marshalling of pbnode  ([ipfs/go-ipfs#3507](https://github.com/ipfs/go-ipfs/pull/3507))

- General changes and refactorings
	- Disable Suborigins the spec changed and our impl conflicts  ([ipfs/go-ipfs#3519](https://github.com/ipfs/go-ipfs/pull/3519))
	- Avoid sending provide messages for pinsets  ([ipfs/go-ipfs#3103](https://github.com/ipfs/go-ipfs/pull/3103))
	- Refactor cli handling to expose argument parsing functionality  ([ipfs/go-ipfs#3308](https://github.com/ipfs/go-ipfs/pull/3308))
	- Create a FilestoreNode object to carry PosInfo  ([ipfs/go-ipfs#3314](https://github.com/ipfs/go-ipfs/pull/3314))
	- Print 'n/a' instead of zero latency in `ipfs swarm peers`  ([ipfs/go-ipfs#3491](https://github.com/ipfs/go-ipfs/pull/3491))
	- Add DAGService.GetLinks() method to optimize traversals.  ([ipfs/go-ipfs#3255](https://github.com/ipfs/go-ipfs/pull/3255))
	- Make path resolver no longer require whole IpfsNode for construction  ([ipfs/go-ipfs#3321](https://github.com/ipfs/go-ipfs/pull/3321))
	- Distinguish between Offline and Local Modes of daemon operation.  ([ipfs/go-ipfs#3259](https://github.com/ipfs/go-ipfs/pull/3259))
	- Separate out the GC Locking from the Blockstore interface.  ([ipfs/go-ipfs#3348](https://github.com/ipfs/go-ipfs/pull/3348))
	- Avoid unnecessary allocs in datastore key handling  ([ipfs/go-ipfs#3407](https://github.com/ipfs/go-ipfs/pull/3407))
	- Use NextSync method for datastore queries ([ipfs/go-ipfs#3386](https://github.com/ipfs/go-ipfs/pull/3386))
	- Switch unixfs.Metadata.MimeType to optional ([ipfs/go-ipfs#3458](https://github.com/ipfs/go-ipfs/pull/3458))
	- Fix path parsing in `ipfs name publish`   ([ipfs/go-ipfs#3592](https://github.com/ipfs/go-ipfs/pull/3592))
	- Fix inconsistent `ipfs stats bw` formatting  ([ipfs/go-ipfs#3554](https://github.com/ipfs/go-ipfs/pull/3554))
	- Set the libp2p agent version based on version string  ([ipfs/go-ipfs#3569](https://github.com/ipfs/go-ipfs/pull/3569))

- Cross Platform Changes
	- Fix 'dist_get' script on BSDs.  ([ipfs/go-ipfs#3264](https://github.com/ipfs/go-ipfs/pull/3264))
	- ulimit: Tune resource limits on BSDs  ([ipfs/go-ipfs#3374](https://github.com/ipfs/go-ipfs/pull/3374))

- Metrics
	- Introduce go-metrics-interface  ([ipfs/go-ipfs#3189](https://github.com/ipfs/go-ipfs/pull/3189))
	- Fix metrics injection  ([ipfs/go-ipfs#3315](https://github.com/ipfs/go-ipfs/pull/3315))

- Misc
	- Bump Go requirement to 1.7  ([ipfs/go-ipfs#3111](https://github.com/ipfs/go-ipfs/pull/3111))
	- Merge 0.4.3 release candidate changes back into master  ([ipfs/go-ipfs#3248](https://github.com/ipfs/go-ipfs/pull/3248))
	- Add security@ipfs.io GPG key to assets  ([ipfs/go-ipfs#2997](https://github.com/ipfs/go-ipfs/pull/2997))
	- Improve makefiles  ([ipfs/go-ipfs#2999](https://github.com/ipfs/go-ipfs/pull/2999))  ([ipfs/go-ipfs#3265](https://github.com/ipfs/go-ipfs/pull/3265))
	- Refactor install.sh script  ([ipfs/go-ipfs#3194](https://github.com/ipfs/go-ipfs/pull/3194))
	- Add test check for go code formatting  ([ipfs/go-ipfs#3421](https://github.com/ipfs/go-ipfs/pull/3421))
	- bin: dist_get script: prevents get_go_vars() returns same values twice  ([ipfs/go-ipfs#3079](https://github.com/ipfs/go-ipfs/pull/3079))

- Dependencies
	- Update libp2p to have fixed spdystream dep  ([ipfs/go-ipfs#3210](https://github.com/ipfs/go-ipfs/pull/3210))
	- Update libp2p and dht packages  ([ipfs/go-ipfs#3263](https://github.com/ipfs/go-ipfs/pull/3263))
	- Update to libp2p 4.0.1 and propogate other changes  ([ipfs/go-ipfs#3284](https://github.com/ipfs/go-ipfs/pull/3284))
	- Update to libp2p 4.0.4  ([ipfs/go-ipfs#3361](https://github.com/ipfs/go-ipfs/pull/3361))
	- Update go-libp2p across codebase  ([ipfs/go-ipfs#3406](https://github.com/ipfs/go-ipfs/pull/3406))
	- Update to go-libp2p 4.1.0  ([ipfs/go-ipfs#3373](https://github.com/ipfs/go-ipfs/pull/3373))
	- Update deps for libp2p 3.4.0  ([ipfs/go-ipfs#3110](https://github.com/ipfs/go-ipfs/pull/3110))
	- Update go-libp2p-swarm with deadlock fixes  ([ipfs/go-ipfs#3339](https://github.com/ipfs/go-ipfs/pull/3339))
	- Update to new cid and ipld node packages  ([ipfs/go-ipfs#3326](https://github.com/ipfs/go-ipfs/pull/3326))
	- Update to newer ipld node interface with Copy and better Tree  ([ipfs/go-ipfs#3391](https://github.com/ipfs/go-ipfs/pull/3391))
	- Update experimental go-multiplex to 0.2.6  ([ipfs/go-ipfs#3475](https://github.com/ipfs/go-ipfs/pull/3475))
	- Rework routing interfaces to make separation easier  ([ipfs/go-ipfs#3107](https://github.com/ipfs/go-ipfs/pull/3107))
	- Update to dht code with fixed GetClosestPeers  ([ipfs/go-ipfs#3346](https://github.com/ipfs/go-ipfs/pull/3346))
	- Move go-is-domain to gx  ([ipfs/go-ipfs#3077](https://github.com/ipfs/go-ipfs/pull/3077))
	- Extract thirdparty/loggables and thirdparty/peerset  ([ipfs/go-ipfs#3204](https://github.com/ipfs/go-ipfs/pull/3204))
	- Completely remove go-key dep  ([ipfs/go-ipfs#3439](https://github.com/ipfs/go-ipfs/pull/3439))
	- Remove randbo dep, its no longer needed  ([ipfs/go-ipfs#3118](https://github.com/ipfs/go-ipfs/pull/3118))
	- Update libp2p for identify configuration updates  ([ipfs/go-ipfs#3539](https://github.com/ipfs/go-ipfs/pull/3539))
	- Use newer flatfs sharding scheme  ([ipfs/go-ipfs#3608](https://github.com/ipfs/go-ipfs/pull/3608))

- Testing
	- fix test_fsh arg quoting in ipfs-test-lib  ([ipfs/go-ipfs#3085](https://github.com/ipfs/go-ipfs/pull/3085))
	- 100% coverage for blocks/blocksutil  ([ipfs/go-ipfs#3090](https://github.com/ipfs/go-ipfs/pull/3090))
	- 100% coverage on blocks/set  ([ipfs/go-ipfs#3084](https://github.com/ipfs/go-ipfs/pull/3084))
	- 81% coverage on blockstore  ([ipfs/go-ipfs#3074](https://github.com/ipfs/go-ipfs/pull/3074))
	- 80% coverage of unixfs/mod  ([ipfs/go-ipfs#3096](https://github.com/ipfs/go-ipfs/pull/3096))
	- 82% coverage on blocks  ([ipfs/go-ipfs#3086](https://github.com/ipfs/go-ipfs/pull/3086))
	- 87% coverage on unixfs   ([ipfs/go-ipfs#3492](https://github.com/ipfs/go-ipfs/pull/3492)) 
	- Improve coverage on routing/offline  ([ipfs/go-ipfs#3516](https://github.com/ipfs/go-ipfs/pull/3516))
	- Add test for flags package   ([ipfs/go-ipfs#3449](https://github.com/ipfs/go-ipfs/pull/3449))
	- improve test coverage on merkledag package  ([ipfs/go-ipfs#3113](https://github.com/ipfs/go-ipfs/pull/3113))
	- 80% coverage of unixfs/io ([ipfs/go-ipfs#3097](https://github.com/ipfs/go-ipfs/pull/3097))
	- Accept more than one digit in repo version tests  ([ipfs/go-ipfs#3130](https://github.com/ipfs/go-ipfs/pull/3130))
	- Fix typo in hash in t0050  ([ipfs/go-ipfs#3170](https://github.com/ipfs/go-ipfs/pull/3170))
	- fix bug in pinsets and add a stress test for the scenario  ([ipfs/go-ipfs#3273](https://github.com/ipfs/go-ipfs/pull/3273))  ([ipfs/go-ipfs#3302](https://github.com/ipfs/go-ipfs/pull/3302))
	- Report coverage to codecov  ([ipfs/go-ipfs#3473](https://github.com/ipfs/go-ipfs/pull/3473))
	- Add test for 'ipfs config replace'  ([ipfs/go-ipfs#3073](https://github.com/ipfs/go-ipfs/pull/3073))
	- Fix netcat on macOS not closing socket when the stdin sends EOF  ([ipfs/go-ipfs#3515](https://github.com/ipfs/go-ipfs/pull/3515))

- Documentation
	- Update dns help with a correct domain name  ([ipfs/go-ipfs#3087](https://github.com/ipfs/go-ipfs/pull/3087))
	- Add period to `ipfs pin rm`  ([ipfs/go-ipfs#3088](https://github.com/ipfs/go-ipfs/pull/3088))
	- Make all Taglines use imperative mood  ([ipfs/go-ipfs#3041](https://github.com/ipfs/go-ipfs/pull/3041))
	- Document listing commands better  ([ipfs/go-ipfs#3083](https://github.com/ipfs/go-ipfs/pull/3083))
	- Add notes to readme on building for uncommon systems  ([ipfs/go-ipfs#3051](https://github.com/ipfs/go-ipfs/pull/3051))
	- Add branch naming conventions doc  ([ipfs/go-ipfs#3035](https://github.com/ipfs/go-ipfs/pull/3035))
	- Replace <default> keyword with <<default>>  ([ipfs/go-ipfs#3129](https://github.com/ipfs/go-ipfs/pull/3129))
	- Fix Add() docs regarding pinning  ([ipfs/go-ipfs#3513](https://github.com/ipfs/go-ipfs/pull/3513))
	- Add sudo to install commands.  ([ipfs/go-ipfs#3201](https://github.com/ipfs/go-ipfs/pull/3201))
	- Add docs for `"commands".Command.Run`  ([ipfs/go-ipfs#3382](https://github.com/ipfs/go-ipfs/pull/3382))
	- Put config keys in proper case  ([ipfs/go-ipfs#3365](https://github.com/ipfs/go-ipfs/pull/3365))
	- Fix link in `ipfs stats bw` help message  ([ipfs/go-ipfs#3620](https://github.com/ipfs/go-ipfs/pull/3620))


### 0.4.4 - 2016-10-11

This release contains an important hotfix for a bug we discovered in how pinning works.
If you had a large number of pins, new pins would overwrite existing pins.
Apart from the hotfix, this release is equal to the previous release 0.4.3.

- Fix bug in pinsets fanout, and add stress test. (@whyrusleeping, [ipfs/go-ipfs#3273](https://github.com/ipfs/go-ipfs/pull/3273))

We published a [detailed account of the bug and fix in a blog post](https://ipfs.io/blog/21-go-ipfs-0-4-4-released/).

### 0.4.3 - 2016-09-20

There have been no changes since the last release candidate 0.4.3-rc4. \o/

### 0.4.3-rc4 - 2016-09-09

This release candidate fixes issues in Bitswap and the `ipfs add` command, and improves testing.
We plan for this to be the last release candidate before the release of go-ipfs v0.4.3.

With this release candidate, we're also moving go-ipfs to Go 1.7, which we expect will yield improvements in runtime performance, memory usage, build time and size of the release binaries.

- Require Go 1.7. (@whyrusleeping, @Kubuxu, @lgierth, [ipfs/go-ipfs#3163](https://github.com/ipfs/go-ipfs/pull/3163))
  - For this purpose, switch Docker image from Alpine 3.4 to Alpine Edge.
- Fix cancellation of Bitswap `wantlist` entries. (@whyrusleeping, [ipfs/go-ipfs#3182](https://github.com/ipfs/go-ipfs/pull/3182))
- Fix clearing of `active` state of Bitswap provider queries. (@whyrusleeping, [ipfs/go-ipfs#3169](https://github.com/ipfs/go-ipfs/pull/3169))
- Fix a panic in the DHT code. (@Kubuxu, [ipfs/go-ipfs#3200](https://github.com/ipfs/go-ipfs/pull/3200))
- Improve handling of `Identity` field in `ipfs config` command. (@Kubuxu, @whyrusleeping, [ipfs/go-ipfs#3141](https://github.com/ipfs/go-ipfs/pull/3141))
- Fix explicit adding of symlinked files and directories. (@kevina, [ipfs/go-ipfs#3135](https://github.com/ipfs/go-ipfs/pull/3135))
- Fix bash auto-completion of `ipfs daemon --unrestricted-api` option. (@lgierth, [ipfs/go-ipfs#3159](https://github.com/ipfs/go-ipfs/pull/3159))
- Introduce a new timeout tool for tests to avoid licensing issues. (@Kubuxu, [ipfs/go-ipfs#3152](https://github.com/ipfs/go-ipfs/pull/3152))
- Improve output for migrations of fs-repo. (@lgierth, [ipfs/go-ipfs#3158](https://github.com/ipfs/go-ipfs/pull/3158))
- Fix info notice of commands taking input from stdin. (@Kubuxu, [ipfs/go-ipfs#3134](https://github.com/ipfs/go-ipfs/pull/3134))
- Bring back a few tests for stdin handling of `ipfs cat` and `ipfs add`. (@Kubuxu, [ipfs/go-ipfs#3144](https://github.com/ipfs/go-ipfs/pull/3144))
- Improve sharness tests for `ipfs repo verify` command. (@whyrusleeping, [ipfs/go-ipfs#3148](https://github.com/ipfs/go-ipfs/pull/3148))
- Improve sharness tests for CORS headers on the gateway. (@Kubuxu, [ipfs/go-ipfs#3142](https://github.com/ipfs/go-ipfs/pull/3142))
- Improve tests for pinning within `ipfs files`. (@kevina, [ipfs/go-ipfs#3151](https://github.com/ipfs/go-ipfs/pull/3151))
- Improve tests for the automatic raising of file descriptor limits. (@whyrusleeping, [ipfs/go-ipfs#3149](https://github.com/ipfs/go-ipfs/pull/3149))

### 0.4.3-rc3 - 2016-08-11

This release candidate fixes a panic that occurs when input from stdin was
expected, but none was given: [ipfs/go-ipfs#3050](https://github.com/ipfs/go-ipfs/pull/3050)

### 0.4.3-rc2 - 2016-08-04

This release includes bugfixes and fixes for regressions that were introduced
between 0.4.2 and 0.4.3-rc1.

- Regressions
  - Fix daemon panic when there is no multipart input provided over the HTTP API.
  (@whyrusleeping, [ipfs/go-ipfs#2989](https://github.com/ipfs/go-ipfs/pull/2989))
  - Fix `ipfs refs --edges` not printing edges.
  (@Kubuxu, [ipfs/go-ipfs#3007](https://github.com/ipfs/go-ipfs/pull/3007))
  - Fix progress option for `ipfs add` defaulting to true on the HTTP API.
  (@whyrusleeping, [ipfs/go-ipfs#3025](https://github.com/ipfs/go-ipfs/pull/3025))
  - Fix erroneous printing of stdin reading message.
  (@whyrusleeping, [ipfs/go-ipfs#3033](https://github.com/ipfs/go-ipfs/pull/3033))
  - Fix panic caused by passing `--mount` and `--offline` flags to `ipfs daemon`.
  (@Kubuxu, [ipfs/go-ipfs#3022](https://github.com/ipfs/go-ipfs/pull/3022))
  - Fix symlink path resolution on windows.
  (@Kubuxu, [ipfs/go-ipfs#3023](https://github.com/ipfs/go-ipfs/pull/3023))
  - Add in code to prevent issue 3032 from crashing the daemon.
  (@whyrusleeping, [ipfs/go-ipfs#3037](https://github.com/ipfs/go-ipfs/pull/3037))


### 0.4.3-rc1 - 2016-07-23

This is a maintenance release which comes with a couple of nice enhancements, and improves the performance of Storage, Bitswap, as well as Content and Peer Routing. It also introduces a handful of new commands and options, and fixes a good bunch of bugs.

This is the first Release Candidate. Unless there are vulnerabilities or regressions discovered, the final 0.4.3 release will happen about one week from now.

- Security Vulnerability

  - The `master` branch if go-ipfs suffered from a vulnerability for about 3 weeks. It allowed an attacker to use an iframe to request malicious HTML and JS from the API of a local go-ipfs node. The attacker could then gain unrestricted access to the node's API, and e.g. extract the private key. We fixed this issue by reintroducing restrictions on which particular objects can be loaded through the API (@lgierth, [ipfs/go-ipfs#2949](https://github.com/ipfs/go-ipfs/pull/2949)), and by completely excluding the private key from the API (@Kubuxu, [ipfs/go-ipfs#2957](https://github.com/ipfs/go-ipfs/pull/2957)). We will also work on more hardening of the API in the next release.
  - **The previous release 0.4.2 is not vulnerable. That means if you're using official binaries from [dist.ipfs.io](https://dist.ipfs.io) you're not affected.** If you're running go-ipfs built from the `master` branch between June 17th ([ipfs/go-ipfs@1afebc21](https://github.com/ipfs/go-ipfs/commit/1afebc21f324982141ca8a29710da0d6f83ca804)) and July 7th ([ipfs/go-ipfs@39bef0d5](https://github.com/ipfs/go-ipfs/commit/39bef0d5b01f70abf679fca2c4d078a2d55620e2)), please update to v0.4.3-rc1 immediately.
  - We are grateful to the group of independent researchers who made us aware of this vulnerability. We wanna use this opportunity to reiterate that we're very happy about any additional review of pull requests and releases. You can contact us any time at security@ipfs.io (GPG [4B9665FB 92636D17 7C7A86D3 50AAE8A9 59B13AF3](https://pgp.mit.edu/pks/lookup?op=get&search=0x50AAE8A959B13AF3)).

- Notable changes

  - Improve Bitswap performance. (@whyrusleeping, [ipfs/go-ipfs#2727](https://github.com/ipfs/go-ipfs/pull/2727), [ipfs/go-ipfs#2798](https://github.com/ipfs/go-ipfs/pull/2798))
  - Improve Content Routing and Peer Routing performance. (@whyrusleeping, [ipfs/go-ipfs#2817](https://github.com/ipfs/go-ipfs/pull/2817), [ipfs/go-ipfs#2841](https://github.com/ipfs/go-ipfs/pull/2841))
  - Improve datastore, blockstore, and dagstore performance. (@kevina, @Kubuxu, @whyrusleeping [ipfs/go-datastore#43](https://github.com/ipfs/go-datastore/pull/43), [ipfs/go-ipfs#2885](https://github.com/ipfs/go-ipfs/pull/2885), [ipfs/go-ipfs#2961](https://github.com/ipfs/go-ipfs/pull/2961), [ipfs/go-ipfs#2953](https://github.com/ipfs/go-ipfs/pull/2953), [ipfs/go-ipfs#2960](https://github.com/ipfs/go-ipfs/pull/2960))
  - Content Providers are now stored on disk to gain savings on process memory. (@whyrusleeping, [ipfs/go-ipfs#2804](https://github.com/ipfs/go-ipfs/pull/2804), [ipfs/go-ipfs#2860](https://github.com/ipfs/go-ipfs/pull/2860))
  - Migrations of the fs-repo (usually stored at `~/.ipfs`) now run automatically. If there's a TTY available, you'll get prompted when running `ipfs daemon`, and in addition you can use the `--migrate=true` or `--migrate=false` options to avoid the prompt. (@whyrusleeping, @lgierth, [ipfs/go-ipfs#2939](https://github.com/ipfs/go-ipfs/pull/2939))
  - The internal naming of blocks in the blockstore has changed, which requires a migration of the fs-repo, from version 3 to 4. (@whyrusleeping, [ipfs/go-ipfs#2903](https://github.com/ipfs/go-ipfs/pull/2903))
  - We now automatically raise the file descriptor limit to 1024 if neccessary. (@whyrusleeping, [ipfs/go-ipfs#2884](https://github.com/ipfs/go-ipfs/pull/2884), [ipfs/go-ipfs#2891](https://github.com/ipfs/go-ipfs/pull/2891))
  - After a long struggle with deadlocks and hanging connections, we've decided to disable the uTP transport by default for now. (@whyrusleeping, [ipfs/go-ipfs#2840](https://github.com/ipfs/go-ipfs/pull/2840), [ipfs/go-libp2p-transport@88244000](https://github.com/ipfs/go-libp2p-transport/commit/88244000f0ce8851ffcfbac746ebc0794b71d2a4))
  - There is now documentation for the configuration options in `docs/config.md`. (@whyrusleeping, [ipfs/go-ipfs#2974](https://github.com/ipfs/go-ipfs/pull/2974))
  - All commands now sanely handle the combination of stdin and optional flags in certain edge cases. (@lgierth, [ipfs/go-ipfs#2952](https://github.com/ipfs/go-ipfs/pull/2952))

- New Features

  - Add `--offline` option to `ipfs daemon` command, which disables all swarm networking. (@Kubuxu, [ipfs/go-ipfs#2696](https://github.com/ipfs/go-ipfs/pull/2696), [ipfs/go-ipfs#2867](https://github.com/ipfs/go-ipfs/pull/2867))
  - Add `Datastore.HashOnRead` option for verifying block hashes on read access. (@Kubuxu, [ipfs/go-ipfs#2904](https://github.com/ipfs/go-ipfs/pull/2904))
  - Add `Datastore.BloomFilterSize` option for tuning the blockstore's new lookup bloom filter. (@Kubuxu, [ipfs/go-ipfs#2973](https://github.com/ipfs/go-ipfs/pull/2973))

- Bugfixes

  - Fix publishing of local IPNS entries, and more. (@whyrusleeping, [ipfs/go-ipfs#2943](https://github.com/ipfs/go-ipfs/pull/2943))
  - Fix progress bars in `ipfs add` and `ipfs get`. (@whyrusleeping, [ipfs/go-ipfs#2893](https://github.com/ipfs/go-ipfs/pull/2893), [ipfs/go-ipfs#2948](https://github.com/ipfs/go-ipfs/pull/2948))
  - Make sure files added through `ipfs files` are pinned and don't get GC'd. (@kevina, [ipfs/go-ipfs#2872](https://github.com/ipfs/go-ipfs/pull/2872))
  - Fix copying into directory using `ipfs files cp`. (@whyrusleeping, [ipfs/go-ipfs#2977](https://github.com/ipfs/go-ipfs/pull/2977))
  - Fix `ipfs version --commit` with Docker containers. (@lgierth, [ipfs/go-ipfs#2734](https://github.com/ipfs/go-ipfs/pull/2734))
  - Run `ipfs diag` commands in the daemon instead of the CLI. (@Kubuxu, [ipfs/go-ipfs#2761](https://github.com/ipfs/go-ipfs/pull/2761))
  - Fix protobuf encoding on the API and in commands. (@stebalien, [ipfs/go-ipfs#2516](https://github.com/ipfs/go-ipfs/pull/2516))
  - Fix goroutine leak in `/ipfs/ping` protocol handler. (@whyrusleeping, [ipfs/go-libp2p#58](https://github.com/ipfs/go-libp2p/pull/58))
  - Fix `--flags` option on `ipfs commands`. (@Kubuxu, [ipfs/go-ipfs#2773](https://github.com/ipfs/go-ipfs/pull/2773))
  - Fix the error channels in `namesys`. (@whyrusleeping, [ipfs/go-ipfs#2788](https://github.com/ipfs/go-ipfs/pull/2788))
  - Fix consumptions of observed swarm addresses. (@whyrusleeping, [ipfs/go-libp2p#63](https://github.com/ipfs/go-libp2p/pull/63), [ipfs/go-ipfs#2771](https://github.com/ipfs/go-ipfs/issues/2771))
  - Fix a rare DHT panic. (@whyrusleeping, [ipfs/go-ipfs#2856](https://github.com/ipfs/go-ipfs/pull/2856))
  - Fix go-ipfs/js-ipfs interoperability issues in SPDY. (@whyrusleeping, [whyrusleeping/go-smux-spdystream@fae17783](https://github.com/whyrusleeping/go-smux-spdystream/commit/fae1778302a9e029bb308cf71cf33f857f2d89e8))
  - Fix a logging race condition during shutdown. (@Kubuxu, [ipfs/go-log#3](https://github.com/ipfs/go-log/pull/3))
  - Prevent DHT connection hangs. (@whyrusleeping, [ipfs/go-ipfs#2826](https://github.com/ipfs/go-ipfs/pull/2826), [ipfs/go-ipfs#2863](https://github.com/ipfs/go-ipfs/pull/2863))
  - Fix NDJSON output of `ipfs refs local`. (@Kubuxu, [ipfs/go-ipfs#2812](https://github.com/ipfs/go-ipfs/pull/2812))
  - Fix race condition in NAT detection. (@whyrusleeping, [ipfs/go-libp2p#69](https://github.com/ipfs/go-libp2p/pull/69))
  - Fix error messages. (@whyrusleeping, @Kubuxu, [ipfs/go-ipfs#2905](https://github.com/ipfs/go-ipfs/pull/2905), [ipfs/go-ipfs#2928](https://github.com/ipfs/go-ipfs/pull/2928))

- Enhancements

  - Increase maximum object size on `ipfs put` from 1 MiB to 2 MiB. The maximum object size on the wire including all framing is 4 MiB. (@kpcyrd, [ipfs/go-ipfs#2980](https://github.com/ipfs/go-ipfs/pull/2980))
  - Add CORS headers to the Gateway's default config. (@Kubuxu, [ipfs/go-ipfs#2778](https://github.com/ipfs/go-ipfs/pull/2778))
  - Clear the dial backoff for a peer when using `ipfs swarm connect`. (@whyrusleeping, [ipfs/go-ipfs#2941](https://github.com/ipfs/go-ipfs/pull/2941))
  - Allow passing options to daemon in Docker container. (@lgierth, [ipfs/go-ipfs#2955](https://github.com/ipfs/go-ipfs/pull/2955))
  - Add `-v/--verbose` to `ìpfs swarm peers` command. (@csasarak, [ipfs/go-ipfs#2713](https://github.com/ipfs/go-ipfs/pull/2713))
  - Add `--format`, `--hash`, and `--size` options to `ipfs files stat` command. (@Kubuxu, [ipfs/go-ipfs#2706](https://github.com/ipfs/go-ipfs/pull/2706))
  - Add `--all` option to `ipfs version` command. (@Kubuxu, [ipfs/go-ipfs#2790](https://github.com/ipfs/go-ipfs/pull/2790))
  - Add `ipfs repo version` command. (@pfista, [ipfs/go-ipfs#2598](https://github.com/ipfs/go-ipfs/pull/2598))
  - Add `ipfs repo verify` command. (@whyrusleeping, [ipfs/go-ipfs#2924](https://github.com/ipfs/go-ipfs/pull/2924), [ipfs/go-ipfs#2951](https://github.com/ipfs/go-ipfs/pull/2951))
  - Add `ipfs stats repo` and `ipfs stats bitswap` command aliases. (@pfista, [ipfs/go-ipfs#2810](https://github.com/ipfs/go-ipfs/pull/2810))
  - Add success indication to responses of `ipfs ping` command. (@Kubuxu, [ipfs/go-ipfs#2813](https://github.com/ipfs/go-ipfs/pull/2813))
  - Save changes made via `ipfs swarm filter` to the config file. (@yuvallanger, [ipfs/go-ipfs#2880](https://github.com/ipfs/go-ipfs/pull/2880))
  - Expand `ipfs_p2p_peers` metric to include libp2p transport. (@lgierth, [ipfs/go-ipfs#2728](https://github.com/ipfs/go-ipfs/pull/2728))
  - Rework `ipfs files add` internals to avoid caching and prevent memory leaks. (@whyrusleeping, [ipfs/go-ipfs#2795](https://github.com/ipfs/go-ipfs/pull/2795))
  - Support `GOPATH` with multiple path components. (@karalabe, @lgierth, @djdv, [ipfs/go-ipfs#2808](https://github.com/ipfs/go-ipfs/pull/2808), [ipfs/go-ipfs#2862](https://github.com/ipfs/go-ipfs/pull/2862), [ipfs/go-ipfs#2975](https://github.com/ipfs/go-ipfs/pull/2975))

- General Codebase

  - Take steps towards the `filestore` datastore. (@kevina, [ipfs/go-ipfs#2792](https://github.com/ipfs/go-ipfs/pull/2792), [ipfs/go-ipfs#2634](https://github.com/ipfs/go-ipfs/pull/2634))
  - Update recommended Golang version to 1.6.2 (@Kubuxu, [ipfs/go-ipfs#2724](https://github.com/ipfs/go-ipfs/pull/2724))
  - Update to Gx 0.8.0 and Gx-Go 1.2.1, which is faster and less noisy. (@whyrusleeping, [ipfs/go-ipfs#2979](https://github.com/ipfs/go-ipfs/pull/2979))
  - Use `go4.org/lock` instead of `camlistore/lock` for locking. (@whyrusleeping, [ipfs/go-ipfs#2887](https://github.com/ipfs/go-ipfs/pull/2887))
  - Manage `go.uuid`, `hamming`, `backoff`, `proquint`, `pb`, `go-context`, `cors`, `go-datastore` packages with Gx. (@Kubuxu, [ipfs/go-ipfs#2733](https://github.com/ipfs/go-ipfs/pull/2733), [ipfs/go-ipfs#2736](https://github.com/ipfs/go-ipfs/pull/2736), [ipfs/go-ipfs#2757](https://github.com/ipfs/go-ipfs/pull/2757), [ipfs/go-ipfs#2825](https://github.com/ipfs/go-ipfs/pull/2825), [ipfs/go-ipfs#2838](https://github.com/ipfs/go-ipfs/pull/2838))
  - Clean up the gateway's surface. (@lgierth, [ipfs/go-ipfs#2874](https://github.com/ipfs/go-ipfs/pull/2874))
  - Simplify the API gateway's access restrictions. (@lgierth, [ipfs/go-ipfs#2949](https://github.com/ipfs/go-ipfs/pull/2949), [ipfs/go-ipfs#2956](https://github.com/ipfs/go-ipfs/pull/2956))
  - Update docker image to Alpine Linux 3.4 and remove Go version constraint. (@lgierth, [ipfs/go-ipfs#2901](https://github.com/ipfs/go-ipfs/pull/2901), [ipfs/go-ipfs#2929](https://github.com/ipfs/go-ipfs/pull/2929))
  - Clarify `Dockerfile` and `Dockerfile.fast`. (@lgierth, [ipfs/go-ipfs#2796](https://github.com/ipfs/go-ipfs/pull/2796))
  - Simplify resolution of Git commit refs in Dockerfiles. (@lgierth, [ipfs/go-ipfs#2754](https://github.com/ipfs/go-ipfs/pull/2754))
  - Consolidate `--verbose` description across commands. (@Kubuxu, [ipfs/go-ipfs#2746](https://github.com/ipfs/go-ipfs/pull/2746))
  - Allow setting position of default values in command option descriptions. (@Kubuxu, [ipfs/go-ipfs#2744](https://github.com/ipfs/go-ipfs/pull/2744))
  - Set explicit default values for boolean command options. (@RichardLitt, [ipfs/go-ipfs#2657](https://github.com/ipfs/go-ipfs/pull/2657))
  - Autogenerate command synopsises. (@Kubuxu, [ipfs/go-ipfs#2785](https://github.com/ipfs/go-ipfs/pull/2785))
  - Fix and improve lots of documentation. (@RichardLitt, [ipfs/go-ipfs#2741](https://github.com/ipfs/go-ipfs/pull/2741), [ipfs/go-ipfs#2781](https://github.com/ipfs/go-ipfs/pull/2781))
  - Improve command descriptions to fit a width of 78 characters. (@RichardLitt, [ipfs/go-ipfs#2779](https://github.com/ipfs/go-ipfs/pull/2779), [ipfs/go-ipfs#2780](https://github.com/ipfs/go-ipfs/pull/2780), [ipfs/go-ipfs#2782](https://github.com/ipfs/go-ipfs/pull/2782))
  - Fix filename conflict in the debugging guide. (@Kubuxu, [ipfs/go-ipfs#2752](https://github.com/ipfs/go-ipfs/pull/2752))
  - Decapitalize log messages, according to Golang style guides. (@RichardLitt, [ipfs/go-ipfs#2853](https://github.com/ipfs/go-ipfs/pull/2853))
  - Add Github Issues HowTo guide. (@RichardLitt, @chriscool, [ipfs/go-ipfs#2889](https://github.com/ipfs/go-ipfs/pull/2889), [ipfs/go-ipfs#2895](https://github.com/ipfs/go-ipfs/pull/2895))
  - Add Github Issue template. (@chriscool, [ipfs/go-ipfs#2786](https://github.com/ipfs/go-ipfs/pull/2786))
  - Apply standard-readme to the README file. (@RichardLitt, [ipfs/go-ipfs#2883](https://github.com/ipfs/go-ipfs/pull/2883))
  - Fix issues pointed out by `govet`. (@Kubuxu, [ipfs/go-ipfs#2854](https://github.com/ipfs/go-ipfs/pull/2854))
  - Clarify `ipfs get` error message. (@whyrusleeping, [ipfs/go-ipfs#2886](https://github.com/ipfs/go-ipfs/pull/2886))
  - Remove dead code. (@whyrusleeping, [ipfs/go-ipfs#2819](https://github.com/ipfs/go-ipfs/pull/2819))
  - Add changelog for v0.4.3. (@lgierth, [ipfs/go-ipfs#2984](https://github.com/ipfs/go-ipfs/pull/2984))

- Tests & CI

  - Fix flaky `ipfs mount` sharness test by using the `iptb` tool. (@noffle, [ipfs/go-ipfs#2707](https://github.com/ipfs/go-ipfs/pull/2707))
  - Fix flaky IP port selection in tests. (@Kubuxu, [ipfs/go-ipfs#2855](https://github.com/ipfs/go-ipfs/pull/2855))
  - Fix CLI tests on OSX by resolving /tmp symlink. (@Kubuxu, [ipfs/go-ipfs#2926](https://github.com/ipfs/go-ipfs/pull/2926))
  - Fix flaky GC test by running the daemon in offline mode. (@Kubuxu, [ipfs/go-ipfs#2908](https://github.com/ipfs/go-ipfs/pull/2908))
  - Add tests for `ipfs add` with hidden files. (@Kubuxu, [ipfs/go-ipfs#2756](https://github.com/ipfs/go-ipfs/pull/2756))
  - Add test to make sure the body of HEAD responses is empty. (@Kubuxu, [ipfs/go-ipfs#2775](https://github.com/ipfs/go-ipfs/pull/2775))
  - Add test to catch misdials. (@Kubuxu, [ipfs/go-ipfs#2831](https://github.com/ipfs/go-ipfs/pull/2831))
  - Mark flaky tests for `ipfs dht query` as known failure. (@noffle, [ipfs/go-ipfs#2720](https://github.com/ipfs/go-ipfs/pull/2720))
  - Remove failing blockstore-without-context test. (@Kubuxu, [ipfs/go-ipfs#2857](https://github.com/ipfs/go-ipfs/pull/2857))
  - Fix `--version` tests for versions with a suffix like `-dev` or `-rc1`. (@lgierth, [ipfs/go-ipfs#2937](https://github.com/ipfs/go-ipfs/pull/2937))
  - Make sharness tests work in cases where go-ipfs is symlinked into GOPATH. (@lgierth, [ipfs/go-ipfs#2937](https://github.com/ipfs/go-ipfs/pull/2937))
  - Add variable delays to blockstore mocks. (@rikonor, [ipfs/go-ipfs#2871](https://github.com/ipfs/go-ipfs/pull/2871))
  - Disable Travis CI email notifications. (@Kubuxu, [ipfs/go-ipfs#2896](https://github.com/ipfs/go-ipfs/pull/2896))


### 0.4.2 - 2016-05-17

This is a patch release which fixes performance and networking bugs in go-libp2p,
You should see improvements in CPU and RAM usage, as well as speed of object lookups.
There are also a few other nice improvements.

* Notable Fixes
  * Set a deadline for dialing attempts. This prevents a node from accumulating
    failed connections. (@whyrusleeping)
  * Avoid unnecessary string/byte conversions in go-multihash. (@whyrusleeping)
  * Fix a deadlock around the yamux stream muxer. (@whyrusleeping)
  * Fix a bug that left channels open, causing hangs. (@whyrusleeping)
  * Fix a bug around yamux which caused connection hangs. (@whyrusleeping)
  * Fix a crash caused by nil multiaddrs. (@whyrusleeping)

* Enhancements
  * Add NetBSD support. (@erde74)
  * Set Cache-Control: immutable on /ipfs responses. (@kpcyrd)
  * Have `ipfs init` optionally accept a default configuration from stdin. (@sivachandran)
  * Add `ipfs log ls` command for listing logging subsystems. (@hsanjuan)
  * Allow bitswap to read multiple messages per stream. (@whyrusleeping)
  * Remove `make toolkit_upgrade` step. (@chriscool)

* Documentation
  * Add a debug-guidelines document. (@richardlitt)
  * Update the contribute document. (@richardlitt)
  * Fix documentation of many `ipfs` commands. (@richardlitt)
  * Fall back to ShortDesc if LongDesc is missing. (@Kubuxu)

* Removals
  * Remove -f option from `ipfs init` command. (@whyrusleeping)

* Bugfixes
  * Fix `ipfs object patch` argument handling and validation. (@jbenet)
  * Fix `ipfs config edit` command by running it client-side. (@Kubuxu)
  * Set default value for `ipfs refs` arguments. (@richardlitt)
  * Fix parsing of incorrect command and argument permutations. (@thomas-gardner)
  * Update Dockerfile to latest go1.5.4-r0. (@chriscool)
  * Allow passing IPFS_LOGGING to Docker image. (@lgierth)
  * Fix dot path parsing on Windows. (@djdv)
  * Fix formatting of `ipfs log ls` output. (@richardlitt)

* General Codebase
  * Refactor Makefile. (@kevina)
  * Wire context into bitswap requests more deeply. (@whyrusleeping)
  * Use gx for iptb. (@chriscool)
  * Update gx and gx-go. (@chriscool)
  * Make blocks.Block an interface. (@kevina)
  * Silence check for Docker existance. (@chriscool)
  * Add dist_get script for fetching tools from dist.ipfs.io. (@whyrusleeping)
  * Add proper defaults to all `ipfs` commands. (@richardlitt)
  * Remove dead `count` option from `ipfs pin ls`. (@richardlitt)
  * Initialize pin mode strings only once. (@chriscool)
  * Add changelog for v0.4.2. (@lgierth)
  * Specify a dist.ipfs.io hash for tool downloads instead of trusting DNS. (@lgierth)

* CI
  * Fix t0170-dht sharness test. (@chriscool)
  * Increase timeout in t0060-daemon sharness test. (@Kubuxu)
  * Have CircleCI use `make deps` instead of `gx` directly. (@whyrusleeping)


### 0.4.1 - 2016-04-25

This is a patch release that fixes a few bugs, and adds a few small (but not
insignificant) features. The primary reason for this release is the listener
hang bugfix that was shipped in the 0.4.0 release.

* Features
  * implemented ipfs object diff (@whyrusleeping)
  * allow promises (used in get, refs) to fail (@whyrusleeping)

* Tool changes
  * Adds 'toolkit_upgrade' to the makefile help target (@achin)

* General Codebase
  * Use extracted go-libp2p-crypto, -secio, -peer packages (@lgierth)
  * Update go-libp2p (@lgierth)
  * Fix package manifest fields (@lgierth)
  * remove incfusever dead-code (@whyrusleeping)
  * remove a ton of unused godeps (@whyrusleeping)
  * metrics: add prometheus back (@lgierth)
  * clean up dead code and config fields (@whyrusleeping)
  * Add log events when blocks are added/removed from the blockstore (@michealmure)
  * repo: don't create logs directory, not used any longer (@lgierth)

* Bugfixes
  * fixed ipfs name resolve --local multihash error (@pfista)
  * ipfs patch commands won't return null links field anymore (@whyrusleeping)
  * Make non recursive resolve print the result (@Kubuxu)
  * Output dirs on ipfs add -rn (@noffle)
  * update libp2p dep to fix hanging listeners problem (@whyrusleeping)
  * Fix Swarm.AddrFilters config setting with regard to `/ip6` addresses (@lgierth)
  * fix dht command key escaping (@whyrusleeping)

* Testing
  * Adds tests to make sure 'object patch' writes. (@noffle)
  * small sharness test for promise failure checking (@whyrusleeping)
  * sharness/Makefile: clean all BINS when cleaning (@chriscool)

* Documentation
  * Fix disconnect argument description (@richardlitt)
  * Added a note about swarm disconnect (@richardlitt)
  * Also fixed syntax for comment (@richardlitt)
  * Alphabetized swarm subcmds (@richardlitt)
  * Added note to ipfs stats bw interval option (@richardlitt)
  * Small syntax changes to repo stat man (@richardlitt)
  * update log command help text (@pfista)
  * Added a long description to add (@richardlitt)
  * Edited object patch set-data doc (@richardlitt)
  * add roadmap.md (@Jeromy)
  * Adds files api cmd to helptext (@noffle)


### 0.4.0 - 2016-04-05

This is a major release with plenty of new features and bugfixes.
It also includes breaking changes which make it incompatible with v0.3.x
on the networking layer.

* Major Changes
  * Multistream
    * The addition of multistream is a breaking change on the networking layer,
      but gives IPFS implementations the ability to mix and match different
      stream multiplexers, e.g. yamux, spdystream, or muxado.
      This adds a ton of flexibility on one of the lower layers of the protocol,
      and will help us avoid further breaking protocol changes in the future.
  * Files API
    * The new `files` command and API allow a program to interact with IPFS
      using familiar filesystem operations, namely: creating directories,
      reading, writing, and deleting files, listing out different directories,
      and so on. This feature enables any other application that uses a
      filesystem-like backend for storage, to use IPFS as its storage driver
      without having change the application logic at all.
  * Gx
    * go-ipfs now uses [gx](https://github.com/whyrusleeping/gx) to manage its
      dependencies. This means that under the hood, go-ipfs's dependencies are
      backed by IPFS itself! It also means that go-ipfs is no longer installed
      using `go get`. Use `make install` instead.
* New Features
  * Web UI
    * Update to new version which is compatible with 0.4.0. (@dignifiedquire)
  * Networking
    * Implement uTP transport. (@whyrusleeping)
    * Allow multiple addresses per configured bootstrap node. (@whyrusleeping)
  * IPNS
    * Improve IPNS resolution performance. (@whyrusleeping)
    * Have dnslink prefer `TXT _dnslink.example.com`, allows usage of CNAME records. (@Kubuxu)
    * Prevent `ipfs name publish` when `/ipns` is mounted. (@noffle)
  * Repo
    * Improve performance of `ipfs add`. (@whyrusleeping)
    * Add `Datastore.NoSync` config option for flatfs. (@rht)
    * Implement mark-and-sweep GC. (@whyrusleeping)
    * Allow for GC during `ipfs add`. (@whyrusleeping)
    * Add `ipfs repo stat` command. (@tmg, @diasdavid)
  * General
    * Add support for HTTP OPTIONS requests. (@lidel)
    * Add `ipfs diag cmds` to view active API requests (@whyrusleeping)
    * Add an `IPFS_LOW_MEM` environment variable which relaxes Bitswap's memory usage. (@whyrusleeping)
    * The Docker image now lives at `ipfs/go-ipfs` and has been completely reworked. (@lgierth)
* Security fixes
  * The gateway path prefix added in v0.3.10 was vulnerable to cross-site
    scripting attacks. This release introduces a configurable list of allowed
    path prefixes. It's called `Gateway.PathPrefixes` and takes a list of
    strings, e.g. `["/blog", "/foo/bar"]`. The v0.3.x line will not receive any
    further updates, so please update to v0.4.0 as soon as possible. (@lgierth)
* Incompatible Changes
  * Install using `make install` instead of `go get` (@whyrusleeping)
  * Rewrite pinning to store pins in IPFS objects. (@tv42)
  * Bump fs-repo version to 3. (@whyrusleeping)
  * Use multistream muxer (@whyrusleeping)
  * The default for `--type` in `ipfs pin ls` is now `all`. (@chriscool)
* Bug Fixes
  * Remove msgio double wrap. (@jbenet)
  * Buffer msgio. (@whyrusleeping)
  * Perform various fixes to the FUSE code. (@tv42)
  * Compute `ipfs add` size in background to not stall add operation. (@whyrusleeping)
  * Add option to have `ipfs add` include top-level hidden files. (@noffle)
  * Fix CORS checks on the API. (@rht)
  * Fix `ipfs update` error message. (@tomgg)
  * Resolve paths in `ipfs pin rm` without network lookup. (@noffle)
  * Detect FUSE unmounts and track mount state. (@noffle)
  * Fix go1.6rc2 panic caused by CloseNotify being called from wrong goroutine. (@rwcarlsen)
  * Bump DHT kvalue from 10 to 20. (@whyrusleeping)
  * Put public key and IPNS entry to DHT in parallel. (@whyrusleeping)
  * Fix panic in CLI argument parsing. (@whyrusleeping)
  * Fix range error by using larger-than-zero-length buffer. (@noffle)
  * Fix yamux hanging issue by increasing AcceptBacklog. (@whyrusleeping)
  * Fix double Transport-Encoding header bug. (@whyrusleeping)
  * Fix uTP panic and file descriptor leak. (@whyrusleeping)
* Tool Changes
  * Add `--pin` option to `ipfs add`, which defaults to `true` and allows `--pin=false`. (@eminence)
  * Add arguments to `ipfs pin ls`. (@chriscool)
  * Add `dns` and `resolve` commands to read-only API. (@Kubuxu)
  * Add option to display headers for `ipfs object links`. (@palkeo)
* General Codebase Changes
  * Check Golang version in Makefile. (@chriscool)
  * Improve Makefile. (@tomgg)
  * Remove dead Jenkins CI code. (@lgierth)
  * Add locking interface to blockstore. (@whyrusleeping)
  * Add Merkledag FetchGraph and EnumerateChildren. (@whyrusleeping)
  * Rename Lock/RLock to GCLock/PinLock. (@jbenet)
  * Implement pluggable datastore types. (@tv42)
  * Record datastore metrics for non-default datastores. (@tv42)
  * Allow multistream to have zero-rtt stream opening. (@whyrusleeping)
  * Refactor `ipnsfs` into a more generic and well tested `mfs`. (@whyrusleeping)
  * Grab more peers if bucket doesn't contain enough. (@whyrusleeping)
  * Use CloseNotify in gateway. (@whyrusleeping)
  * Flatten multipart file transfers. (@whyrusleeping)
  * Send updated DHT record fixes to peers who sent outdated records. (@whyrusleeping)
  * Replace go-psutil with go-sysinfo. (@whyrusleeping)
  * Use ServeContent for index.html. (@AtnNn)
  * Refactor `object patch` API to not store data in URL. (@whyrusleeping)
  * Use mfs for `ipfs add`. (@whyrusleeping)
  * Add `Server` header to API responses. (@Kubuxu)
  * Wire context directly into HTTP requests. (@rht)
  * Wire context directly into GetDAG operations within GC. (@rht)
  * Vendor libp2p using gx. (@whyrusleeping)
  * Use gx vendored packages instead of Godeps. (@whyrusleeping)
  * Simplify merkledag package interface to ease IPLD inclusion. (@mildred)
  * Add default option value support to commands lib. (@whyrusleeping)
  * Refactor merkledag fetching methods. (@whyrusleeping)
  * Use net/url to escape paths within Web UI. (@noffle)
  * Deprecated key.Pretty(). (@MichealMure)
* Documentation
  * Fix and update help text for **every** `ipfs` command. (@RichardLitt)
  * Change sample API origin settings from wildcard (`*`) to `example.com`. (@Kubuxu)
  * Improve documentation of installation process in README. (@whyrusleeping)
  * Improve windows.md. (@chriscool)
  * Clarify instructions for installing from source. (@noffle)
  * Make version checking more robust. (@jedahan)
  * Assert the source code is located within GOPATH. (@whyrusleeping)
  * Remove mentions of `/dns` from `ipfs dns` command docs. (@lgierth)
* Testing
  * Refactor iptb tests. (@chriscool)
  * Improve t0240 sharness test. (@chriscool)
  * Make bitswap tests less flaky. (@whyrusleeping)
  * Use TCP port zero for ipfs daemon in sharness tests. (@whyrusleeping)
  * Improve sharness tests on AppVeyor. (@chriscool)
  * Add a pause to fix timing on t0065. (@whyrusleeping)
  * Add support for arbitrary TCP ports to t0060-daemon.sh. (@noffle)
  * Make t0060 sharness test use TCP port zero. (@whyrusleeping)
  * Randomized ipfs stress testing via randor (@dignifiedquire)
  * Stress test pinning and migrations (@whyrusleeping)

### 0.3.11 - 2016-01-12

This is the final ipfs version before the transition to v0.4.0.
It introduces a few stability improvements, bugfixes, and increased
test coverage.

* Features
  * Add 'get' and 'patch' to the allowed gateway commands (@whyrusleeping)
  * Updated webui version (@dignifiedquire)

* BugFixes
  * Fix path parsing for add command (@djdv)
  * namesys: Make paths with multiple segments work. Fixes #2059 (@Kubuxu)
  * Fix up panic catching in http handler funcs (@whyrusleeping)
  * Add correct access control headers to the default api config (@dignifiedquire)
  * Fix closenotify by not sending empty file set (@whyrusleeping)

* Tool Changes
  * Have install.sh use the full path to ipfs binary if detected (@jedahan)
  * Install daemon system-wide if on El Capitan (@jedahan)
  * makefile: add -ldflags to install and nofuse tasks (@lgierth)

* General Codebase
  * Clean up http client code (@whyrusleeping)
  * Move api version check to header (@rht)

* Documentation
  * Improved release checklist (@jbenet)
  * Added quotes around command in long description (@RichardLitt)
  * Added a shutdown note to daemon description (@RichardLitt)

* Testing
  * t0080: improve last tests (@chriscool)
  * t0080: improve 'ipfs refs --unique' test (@chriscool)
  * Fix t.Fatal usage in goroutines (@chriscool)
  * Add docker testing support to sharness (@chriscool)
  * sharness: add t0300-docker-image.sh (@chriscool)
  * Included more namesys tests. (@Kubuxu)
  * Add sharness test to verify requests look good (@whyrusleeping)
  * Re-enable ipns sharness test now that iptb is fixed (@whyrusleeping)
  * Force use of ipv4 in test (@whyrusleeping)
  * Travis-CI: use go 1.5.2 (@jbenet)

### 0.3.10 - 2015-12-07

This patch update introduces the 'ipfs update' command which will be used for
future ipfs updates along with a few other bugfixes and documentation
improvements.


* Features
  * support for 'ipfs update' to call external binary (@whyrusleeping)
  * cache ipns entries to speed things up a little (@whyrusleeping)
  * add option to version command to print repo version (@whyrusleeping)
  * Add in some more notifications to help profile queries (@whyrusleeping)
  * gateway: add path prefix for directory listings (@lgierth)
  * gateway: add CurrentCommit to /version (@lgierth)

* BugFixes
  * set data and links nil if not present (@whyrusleeping)
  * fix log hanging issue, and implement close-notify for commands (@whyrusleeping)
  * fix dial backoff (@whyrusleeping)
  * proper ndjson implementation (@whyrusleeping)
  * seccat: fix secio context (@lgierth)
  * Add newline to end of the output for a few commands. (@nham)
  * Add fixed period repo GC + test (@rht)

* Tool Changes
  * Allow `ipfs cat` on ipns path (@rht)

* General Codebase
  * rewrite of backoff mechanism (@whyrusleeping)
  * refactor net code to use transports, in rough accordance with libp2p (@whyrusleeping)
  * disable building fuse stuff on windows (@whyrusleeping)
  * repo: remove Log config (@lgierth)
  * commands: fix description of --api (@lgierth)

* Documentation
  * --help: Add a note on using IPFS_PATH to the footer of the helptext.  (@sahib)
  * Moved email juan to ipfs/contribute (@richardlitt)
  * Added commit sign off section (@richardlitt)
  * Added a security section (@richardlitt)
  * Moved TODO doc to issue #1929 (@richardlitt)

* Testing
  * gateway: add tests for /version (@lgierth)
  * Add gc auto test (@rht)
  * t0020: cleanup dir with bad perms (@chriscool)

Note: this commit introduces fixed-period repo gc, which will trigger gc
after a fixed period of time. This feature is introduced now, disabled by
default, and can be enabled with `ipfs daemon --enable-gc`. If all goes well,
in the future, it will be enabled by default.

### 0.3.9 - 2015-10-30

This patch update includes a good number of bugfixes, notably, it fixes
builds on windows, and puts newlines between streaming json objects for a
proper ndjson format.

* Features
  * Writable gateway enabled again (@cryptix)

* Bugfixes
  * fix windows builds (@whyrusleeping)
  * content type on command responses default to text (@whyrusleeping)
  * add check to makefile to ensure windows builds don't fail silently (@whyrusleeping)
  * put newlines between streaming json output objects (@whyrusleeping)
  * fix streaming output to flush per write (@whyrusleeping)
  * purposely fail builds pre go1.5 (@whyrusleeping)
  * fix ipfs id <self> (@whyrusleeping)
  * fix a few race conditions in mocknet (@whyrusleeping)
  * fix makefile failing when not in a git repo (@whyrusleeping)
  * fix cli flag orders (long, short) (@rht)
  * fix races in http cors (@miolini)
  * small webui update (some bugfixes) (@jbenet)

* Tool Changes
  * make swarm connect return an error when it fails (@whyrusleeping)
  * Add short flag for `ipfs ls --headers` (v for verbose) (@rht)

* General Codebase
  * bitswap: clean log printf and humanize dup data count (@cryptix)
  * config: update pluto's peerID (@lgierth)
  * config: update bootstrap list hostname (@lgierth)

* Documentation
  * Pared down contribute to link to new go guidelines (@richardlitt)

* Testing
  * t0010: add tests for 'ipfs commands --flags' (@chriscool)
  * ipns_test: fix namesys.NewNameSystem() call (@chriscool)
  * t0060: fail if no nc (@chriscool)

### 0.3.8 - 2015-10-09

This patch update includes changes to make ipns more consistent and reliable,
symlink support in unixfs, mild performance improvements, new tooling features,
a plethora of bugfixes, and greatly improved tests.

NOTICE: Version 0.3.8 also requires golang version 1.5.1 or higher.

* Bugfixes
  * refactor ipns to be more consistent and reliable (@whyrusleeping)
  * fix 'ipfs refs' json output (@whyrusleeping)
  * fix setting null config maps (@rht)
  * fix output of dht commands (@whyrusleeping)
  * fix NAT spam dialing (@whyrusleeping)
  * fix random panics on 32 bit systems (@whyrusleeping)
  * limit total number of network fd's (@whyrusleeping)
  * fix http api content type (@WeMeetAgain)
  * fix writing of api file for port zero daemons (@whyrusleeping)
  * windows connection refused fixes (@mjanczyk)
  * use go1.5's built in trailers, no more failures (@whyrusleeping)
  * fix random bitswap hangs (@whyrusleeping)
  * rate limit fd usage (@whyrusleeping)
  * fix panic in bitswap ratelimiting (@whyrusleeping)

* Tool Changes
  * --empty-repo option for init (@prusnak)
  * implement symlinks (@whyrusleeping)
  * improve cmds lib files processing (@rht)
  * properly return errors through commands (@whyrusleeping)
  * bitswap unwant command (@whyrusleeping)
  * tar add/cat commands (@whyrusleeping)
  * fix gzip compression in get (@klauspost)
  * bitswap stat logs wasted bytes (@whyrusleeping)
  * resolve command now uses core.Resolve (@rht)
  * add `--local` flag to 'name resolve' (@whyrusleeping)
  * add `ipfs diag sys` command for debugging help (@whyrusleeping)

* General Codebase
  * improvements to dag editor (@whyrusleeping)
  * swarm IPv6 in default config (Baptiste Jonglez)
  * improve dir listing css (@rht)
  * removed elliptic.P224 usage (@prusnak)
  * improve bitswap providing speed (@jbenet)
  * print panics that occur in cmds lib (@whyrusleeping)
  * ipfs api check test fixes (@rht)
  * update peerstream and datastore (@whyrusleeping)
  * cleaned up tar-reader code (@jbenet)
  * write context into coreunix.Cat (@rht)
  * move assets to separate repo (@rht)
  * fix proc/ctx wiring in bitswap (@jbenet)
  * rabin fingerprinting chunker (@whyrusleeping)
  * better notification on daemon ready (@rht)
  * coreunix cat cleanup (@rht)
  * extract logging into go-log (@whyrusleeping)
  * blockservice.New no longer errors (@whyrusleeping)
  * refactor ipfs get (@rht)
  * readonly api on gateway (@rht)
  * cleanup context usage all over (@rht)
  * add xml decoding to 'object put' (@ForrestWeston)
  * replace nodebuilder with NewNode method (@whyrusleeping)
  * add metrics to http handlers (@lgierth)
  * rm blockservice workers (@whyrusleeping)
  * decompose maybeGzWriter (@rht)
  * makefile sets git commit sha on build (@CaioAlonso)

* Documentation
  * add contribute file (@RichardLitt)
  * add go devel guide to contribute.md (@whyrusleeping)

* Testing
  * fix mock notifs test (@whyrusleeping)
  * test utf8 with object cmd (@chriscool)
  * make mocknet conn close idempotent (@jbenet)
  * fix fuse tests (@pnelson)
  * improve sharness test quoting (@chriscool)
  * sharness tests for chunker and add-cat (@rht)
  * generalize peerid check in sharness (@chriscool)
  * test_cmp argument cleanup (@chriscool)

### 0.3.7 - 2015-08-02

This patch update fixes a problem we introduced in 0.3.6 and did not
catch: the webui failed to work with out-of-the-box CORS configs.
This has been fixed and now should work correctly. @jbenet

### 0.3.6 - 2015-07-30

This patch improves the resource consumption of go-ipfs,
introduces a few new options on the CLI, and also
fixes (yet again) windows builds.

* Resource consumption:
  * fixed goprocess memory leak @rht
  * implement batching on datastore @whyrusleeping
  * Fix bitswap memory leak @whyrusleeping
  * let bitswap ignore temporary write errors @whyrusleeping
  * remove logging to disk in favor of api endpoint @whyrusleeping
  * --only-hash option for add to skip writing to disk @whyrusleeping

* Tool changes
  * improved `ipfs daemon` output with all addresses @jbenet
  * improved `ipfs id -f` output, added `<addrs>` and  `\n \t` support @jbenet
  * `ipfs swarm addrs local` now shows the local node's addrs @jbenet
  * improved config json parsing @rht
  * improved Dockerfile to use alpine linux @Luzifer @lgierth
  * improved bash completion @MichaelMure
  * Improved 404 for gateway @cryptix
  * add unixfs ls to list correct filesizes @wking
  * ignore hidden files by default @gatesvp
  * global --timeout flag @whyrusleeping
  * fix random API failures by closing resp bodies @whyrusleeping
  * ipfs swarm filters @whyrusleeping
  * api returns errors in http trailers @whyrusleeping @jbenet
  * `ipfs patch` learned to create intermediate nodes @whyrusleeping
  * `ipfs object stat` now shows Hash @whyrusleeping
  * `ipfs cat` now clears progressbar on exit @rht
  * `ipfs add -w -r <dir>` now wraps directories @jbenet
  * `ipfs add -w <file1> <file2>` now wraps with one dir @jbenet
  * API + Gateway now support arbitrary HTTP Headers from config @jbenet
  * API now supports CORS properly from config @jbenet
  * **Deprecated:** `API_ORIGIN` env var (use config, see `ipfs daemon --help`) @jbenet

* General Codebase
  * `nofuse` tag for windows @Luzifer
  * improved `ipfs add` code @gatesvp
  * started requiring license trailers @chriscool @jbenet
  * removed CtxCloser for goprocess @rht
  * remove deadcode @lgierth @whyrusleeping
  * reduced number of logging libs to 2 (soon to be 1) @rht
  * dial address filtering @whyrusleeping
  * prometheus metrics @lgierth
  * new index page for gateway @krl @cryptix
  * move ping to separate protocol @whyrusleeping
  * add events to bitswap for a dashboard @whyrusleeping
  * add latency and bandwidth options to mocknet @heems
  * levenshtein distance cmd autosuggest @sbruce
  * refactor/cleanup of cmds http handler @whyrusleeping
  * cmds http stream reports errors in trailers @whyrusleeping

* Bugfixes
  * fixed path resolution and validation @rht
  * fixed `ipfs get -C` output and progress bar @rht
  * Fixed install pkg dist bug @jbenet @Luzifer
  * Fix `ipfs get` silent failure   @whyrusleeping
  * `ipfs get` tarx no longer times out @jbenet
  * `ipfs refs -r -u` is now correct @gatesvp
  * Fix `ipfs add -w -r <dir>` wrapping bugs @jbenet
  * Fixed FUSE unmount failures @jbenet
  * Fixed `ipfs log tail` command (api + cli) @whyrusleeping

* Testing
  * sharness updates @chriscool
  * ability to disable secio for testing @jbenet
  * fixed many random test failures, more reliable CI @whyrusleeping
  * Fixed racey notifier failures @whyrusleeping
  * `ipfs refs -r -u` test cases @jbenet
  * Fix failing pinning test @jbenet
  * Better CORS + Referer tests @jbenet
  * Added reversible gc test @rht
  * Fixed bugs in FUSE IPNS tests @whyrusleeping
  * Fixed bugs in FUSE IPFS tests @jbenet
  * Added `random-files` tool for easier sharness tests @jbenet

* Documentation
  * Add link to init system examples @slang800
  * Add CORS documentation to daemon init @carver  (Note: this will change soon)

### 0.3.5 - 2015-06-11

This patch improves overall stability and performance

* added 'object patch' and 'object new' commands @whyrusleeping
* improved symmetric NAT avoidance @jbenet
* move util.Key to blocks.Key @whyrusleeping
* fix memory leak in provider store @whyrusleeping
* updated webui to 0.2.0 @krl
* improved bitswap performance @whyrusleeping
* update fuse lib @cryptix
* fix path resolution @wking
* implement test_seq() in sharness @chriscool
* improve parsing of stdin for commands @chriscool
* fix 'ipfs refs' failing silently @whyrusleeping
* fix serial dialing bug @jbenet
* improved testing @chriscool @rht @jbenet
* fixed domain resolving @luzifer
* fix parsing of unwanted stdin @lgierth
* added CORS handlers to gateway @NodeGuy
* added `ipfs daemon --unrestricted-api` option @krl
* general cleanup of dependencies

### 0.3.4 - 2015-05-10

* fix ipns append bug @whyrusleeping
* fix out of memory panic @whyrusleeping
* add in expvar metrics @tv42
* bitswap improvements @whyrusleeping
* fix write-cache in blockstore @tv42
* vendoring cleanup @cryptix
* added `launchctl` plist for OSX @grncdr
* improved Dockerfile, changed root and mount paths @ehd
* improved `pin ls` output to show types @vitorbaptista

### 0.3.3 - 2015-04-28

This patch update fixes various issues, in particular:
- windows support (0.3.0 had broken it)
- commandline parses spaces correctly.

* much improved commandline parsing by @AtnNn
* improved dockerfile by @luzifer
* add cmd cleanup by @wking
* fix flatfs windows support by @tv42 and @gatesvp
* test case improvements by @chriscool
* ipns resolution timeout bug fix by @whyrusleeping
* new cluster tests with iptb by @whyrusleeping
* fix log callstack printing bug by @whyrusleeping
* document bash completion by @dylanPowers

### 0.3.2 - 2015-04-22

This patch update implements multicast dns as well as fxing a few test issues.

* implement mdns peer discovery @whyrusleeping
* fix mounting issues in sharness tests @chriscool

### 0.3.1 - 2015-04-21

This patch update fixes a few bugs:

* harden shutdown logic by @torarnv
* daemon locking fixes by @travisperson
* don't re-add entire dirs by @whyrusleeping
* tests now wait for graceful shutdown by @jbenet
* default key size is now 2048 by @jbenet

### 0.3.0 - 2015-04-20

We've just released version 0.3.0, which contains many
performance improvements, bugfixes, and new features.
Perhaps the most noticeable change is moving block storage
from leveldb to flat files in the filesystem.

What to expect:

* _much faster_ performance

* Repo format 2
  * moved default location from ~/.go-ipfs -> ~/.ipfs
  * renamed lock filename daemon.lock -> repo.lock
  * now using a flat-file datastore for local blocks

* Fixed lots of bugs
  * proper ipfs-path in various commands
  * fixed two pinning bugs (recursive pins)
  * increased yamux streams window (for speed)
  * increased bitswap workers (+ env var)
  * fixed memory leaks
  * ipfs add error returns
  * daemon exit bugfix
  * set proper UID and GID on fuse mounts

* Gateway
  * Added support for HEAD requests

* configuration
  * env var to turn off SO_REUSEPORT: IPFS_REUSEPORT=false
  * env var to increase bitswap workers: IPFS_BITSWAP_TASK_WORKERS=n

* other
  * bash completion is now available
  * ipfs stats bw -- bandwidth meetering

And many more things.

### 0.2.3 - 2015-03-01

* Alpha Release

### 2015-01-31:

* bootstrap addresses now have .../ipfs/... in format
  config file Bootstrap field changed accordingly. users
  can upgrade cleanly with:

      ipfs bootstrap >boostrap_peers
      ipfs bootstrap rm --all
      <install new ipfs>
      <manually add .../ipfs/... to addrs in bootstrap_peers>
      ipfs bootstrap add <bootstrap_peers
