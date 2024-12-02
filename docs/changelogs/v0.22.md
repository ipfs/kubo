# Kubo changelog v0.22

- [v0.22.0](#v0220)

## v0.22.0

- [Overview](#overview)
- [🔦 Highlights](#-highlights)
  - [Gateway: support for `order=` and `dups=` parameters (IPIP-412)](#gateway-support-for-order-and-dups-parameters-ipip-412)
  - [`ipfs name publish` now supports V2 only IPNS records](#ipfs-name-publish-now-supports-v2-only-ipns-records)
  - [IPNS name resolution has been fixed](#ipns-name-resolution-has-been-fixed)
  - [go-libp2p v0.29.0 update with smart dialing](#go-libp2p-v0290-update-with-smart-dialing)
- [📝 Changelog](#-changelog)
- [👨‍👩‍👧‍👦 Contributors](#-contributors)

### Overview

### 🔦 Highlights

#### Gateway: support for `order=` and `dups=` parameters (IPIP-412)

The updated [`boxo/gateway` library](https://github.com/ipfs/boxo/tree/main/gateway)
introduces support for ordered CAR responses through the inclusion of optional
CAR content type parameters: `order=dfs` and `dups=y|n` from
[IPIP-412](https://github.com/ipfs/specs/pull/412).

Previously, Kubo already provided CARs in DFS order without duplicate blocks.
With the implementation of IPIP-412, this behavior is now explicitly defined
rather than implied.

In the absence of `dups` or `order` in `Accept` request reader, the default CAR
response will have the `Content-Type: application/vnd.ipld.car; version=1; order=dfs; dups=n`
and the same blocks as Kubo 0.21.

Kubo 0.22 still only supports DFS block ordering (`order=dfs`). However, it is
now possible to request a DFS CAR stream with duplicate blocks by opting in via
`Accept: application/vnd.ipld.car; order=dfs; dups=y`. This opt-in feature can be
beneficial for memory-constrained clients and IoT devices, as it allows for
streaming large DAGs without the need to store all previously encountered
blocks in memory.

#### `ipfs name publish` now supports V2 only IPNS records

When publishing an IPNS record, you are now able to create v2 only records
by passing `--v1compat=false`. By default, we still create V1+V2 records, such
that there is the highest chance of backwards compatibility. The goal is to move
to V2 only in the future.

For more details, see [IPIP-428](https://specs.ipfs.tech/ipips/ipip-0428/)
and the updated [IPNS Record Verification](https://specs.ipfs.tech/ipns/ipns-record/#record-verification) logic.

#### IPNS name resolution has been fixed

IPNS name resolution had a regression where if IPNS over PubSub was enabled, but the name was not also available via IPNS over PubSub it would take 1 minute to for the lookup to complete (if the record was not yet cached).

This has been fixed and as before will give the best record from either the DHT subsystem or IPNS over PubSub, whichever comes back first.

For details see [#9927](https://github.com/ipfs/kubo/issues/9927) and [#10020](https://github.com/ipfs/kubo/pull/10020).

# go-libp2p v0.29.0 update with smart dialing

We updated from [go-libp2p](https://github.com/libp2p/go-libp2p) [v0.27.7](https://github.com/libp2p/go-libp2p/releases/tag/v0.27.7) to [v0.29.0](https://github.com/libp2p/go-libp2p/releases/tag/v0.29.0).  This release includes smart dialing, which is a prioritization algorithm that will try to rank addresses and protocols rather than attempting all options in parallel.  Anecdotally, we have observed [Kubo nodes make 30% less dials](https://github.com/libp2p/go-libp2p/issues/2326#issuecomment-1644332863) with no to low latency impact.

This includes a breaking change to `ipfs id` and some of the `ipfs swarm` commands.  We no longer report `ProtocolVersion`.  This used to be hardcoded as `ipfs/0.1.0` and sent to other peers but was not providing any distinguishing value.  See [libp2p/go-libp2p#2294](https://github.com/libp2p/go-libp2p/issues/2294) for more information.

### 📝 Changelog

<details><summary>Full Changelog</summary>

- github.com/ipfs/kubo:
  - chore: change version to v0.22.0
  - chore(misc/README.md): trim duplicated content
  - Merge branch 'release-v0.21' back into master
  - docs(readme): unofficial packages badge
  - chore: remove sharness tests ported to conformance testing (#9999) ([ipfs/kubo#9999](https://github.com/ipfs/kubo/pull/9999))
  - ci: switch from testing against js-ipfs to helia (#10042) ([ipfs/kubo#10042](https://github.com/ipfs/kubo/pull/10042))
  - chore: merge release back into master
  - chore: change orbitdb to haydenyoung EARLY_TESTERS
  - Fix usage numbers
  - chore: update early testers list (#9218) ([ipfs/kubo#9218](https://github.com/ipfs/kubo/pull/9218))
  - docs: changelog v0.21 fixes (#10037) ([ipfs/kubo#10037](https://github.com/ipfs/kubo/pull/10037))
  - refactor(ci): simplify Dockerfile and add docker image testing (#10021) ([ipfs/kubo#10021](https://github.com/ipfs/kubo/pull/10021))
  - chore: update version
  - fix(relay): apply user provider options
  - libp2p: stop reporting ProtocolVersion
  - chore: update go-libp2p to v0.29.0
  - chore: update go-libp2p to v0.28.1
  - fix: mark all routers DoNotWaitForSearchValue (#10020) ([ipfs/kubo#10020](https://github.com/ipfs/kubo/pull/10020))
  - feat(gateway): support for ipip-412 parameters
  - docs(commands): explain that swarm connect can reuse existing connections or known addresses (#10015) ([ipfs/kubo#10015](https://github.com/ipfs/kubo/pull/10015))
  - docs: add Brave to RELEASE_ISSUE_TEMPLATE.md (#10012) ([ipfs/kubo#10012](https://github.com/ipfs/kubo/pull/10012))
  - feat: webui@4.0.2
  -  ([ipfs/kubo#10008](https://github.com/ipfs/kubo/pull/10008))
  - docs: skip check before prepare branch in RELEASE_ISSUE_TEMPLATE.md
  - docs: update RELEASE_ISSUE_TEMPLATE.md with a warning about npm publish
  - docs: update refs to kuboreleaser in RELEASE_ISSUE_TEMPLATE.md
  - docs: Gateway.HTTPHeaders
  - refactor: replace boxo/ipld/car by ipld/go-car
  - chore: bump to boxo master
  - fix: correctly handle migration of configs
  - fix(gateway): include CORS on subdomain redirects (#9994) ([ipfs/kubo#9994](https://github.com/ipfs/kubo/pull/9994))
  - fix: docker repository initialization race condition
  - feat(ipns): records with V2-only signatures (#9932) ([ipfs/kubo#9932](https://github.com/ipfs/kubo/pull/9932))
  - cmds/dag/import: pin roots by default (#9966) ([ipfs/kubo#9966](https://github.com/ipfs/kubo/pull/9966))
  - docs: fix 0.21 changelog
  - feat!: dag import - don't pin roots by default (#9926) ([ipfs/kubo#9926](https://github.com/ipfs/kubo/pull/9926))
  - fix(cmd): useful errors in dag import (#9945) ([ipfs/kubo#9945](https://github.com/ipfs/kubo/pull/9945))
  - feat: webui@4.0.1 (#9940) ([ipfs/kubo#9940](https://github.com/ipfs/kubo/pull/9940))
  - chore(docs): typo http→https
  - fix: more stable prometheus test (#9944) ([ipfs/kubo#9944](https://github.com/ipfs/kubo/pull/9944))
  -  ([ipfs/kubo#9937](https://github.com/ipfs/kubo/pull/9937))
- github.com/ipfs/boxo (v0.10.3 -> v0.11.0):
  - Release v0.11.0 ([ipfs/boxo#417](https://github.com/ipfs/boxo/pull/417))
- github.com/ipfs/go-bitswap (null -> v0.11.0):
  - chore: release v0.11.0
  - chore: release v0.10.2
  - fix: create a copy of the protocol slice in network.processSettings
  - chore: release v0.10.1
  - fix: incorrect type in the WithTracer polyfill option
  - chore: fix incorrect log message when a bad option is passed
  - chore: release v0.10.0
  - chore: update go-libp2p v0.22.0
  - chore: release v0.9.0
  - feat: split client and server ([ipfs/go-bitswap#570](https://github.com/ipfs/go-bitswap/pull/570))
  - chore: remove goprocess from blockstoremanager
  - Don't add blocks to the datastore ([ipfs/go-bitswap#571](https://github.com/ipfs/go-bitswap/pull/571))
  - Remove dependency on travis package from go-libp2p-testing ([ipfs/go-bitswap#569](https://github.com/ipfs/go-bitswap/pull/569))
  - feat: add basic tracing (#562) ([ipfs/go-bitswap#562](https://github.com/ipfs/go-bitswap/pull/562))
  - chore: release v0.7.0 (#566) ([ipfs/go-bitswap#566](https://github.com/ipfs/go-bitswap/pull/566))
  - feat: coalesce and queue connection event handling (#565) ([ipfs/go-bitswap#565](https://github.com/ipfs/go-bitswap/pull/565))
- github.com/ipfs/go-merkledag (v0.10.0 -> v0.11.0):
  - chore: update v0.11.0 (#106) ([ipfs/go-merkledag#106](https://github.com/ipfs/go-merkledag/pull/106))
  - update merkeldag to use the explicit decoder registry (#104) ([ipfs/go-merkledag#104](https://github.com/ipfs/go-merkledag/pull/104))
  - Update status in README.md and added CODEOWNERS (#101) ([ipfs/go-merkledag#101](https://github.com/ipfs/go-merkledag/pull/101))
- github.com/ipld/go-car/v2 (v2.9.1-0.20230325062757-fff0e4397a3d -> v2.10.2-0.20230622090957-499d0c909d33):
  - feat: add inverse and version to filter cmd ([ipld/go-car#457](https://github.com/ipld/go-car/pull/457))
  - v0.6.1 bump
  - chore: update usage of merkledag by go-car (#437) ([ipld/go-car#437](https://github.com/ipld/go-car/pull/437))
  - feat(cmd/car): add '--no-wrap' option to 'create' command ([ipld/go-car#432](https://github.com/ipld/go-car/pull/432))
  - fix: remove github.com/ipfs/go-ipfs-blockstore dependency
  - feat: expose index for StorageCar
  - perf: reduce NewCarReader allocations
  - fix(deps): update deps for cmd (use master go-car and go-car/v2 for now)
  - fix: new error strings from go-cid
  - fix: tests should match stderr for verbose output
  - fix: reading from stdin should broadcast EOF to block loaders
  - refactor insertion index to be publicly accessible ([ipld/go-car#408](https://github.com/ipld/go-car/pull/408))
- github.com/libp2p/go-libp2p (v0.27.9 -> v0.29.2):
  - release v0.29.2
  - release v0.29.1
  - swarm: don't open new streams over transient connections (#2450) ([libp2p/go-libp2p#2450](https://github.com/libp2p/go-libp2p/pull/2450))
  - core/crypto: restrict RSA keys to <= 8192 bits (#2454) ([libp2p/go-libp2p#2454](https://github.com/libp2p/go-libp2p/pull/2454))
  - Release version v0.29.0 (#2431) ([libp2p/go-libp2p#2431](https://github.com/libp2p/go-libp2p/pull/2431))
  - webtransport: reject listening on a multiaddr with a certhash (#2426) ([libp2p/go-libp2p#2426](https://github.com/libp2p/go-libp2p/pull/2426))
  - swarm: deprecate libp2p.DialRanker option (#2430) ([libp2p/go-libp2p#2430](https://github.com/libp2p/go-libp2p/pull/2430))
  - quic: Update to quic-go v0.36.2 (#2424) ([libp2p/go-libp2p#2424](https://github.com/libp2p/go-libp2p/pull/2424))
  - autonat: fix typo in WithSchedule option comment (#2425) ([libp2p/go-libp2p#2425](https://github.com/libp2p/go-libp2p/pull/2425))
  - identify: filter nat64 well-known prefix ipv6 addresses (#2392) ([libp2p/go-libp2p#2392](https://github.com/libp2p/go-libp2p/pull/2392))
  - update go-multiaddr to v0.10.1, use Unique function from there (#2407) ([libp2p/go-libp2p#2407](https://github.com/libp2p/go-libp2p/pull/2407))
  - swarm: enable smart dialing by default (#2420) ([libp2p/go-libp2p#2420](https://github.com/libp2p/go-libp2p/pull/2420))
  - transport integration tests: make TestMoreStreamsThanOurLimits less flaky (#2410) ([libp2p/go-libp2p#2410](https://github.com/libp2p/go-libp2p/pull/2410))
  - holepunch: skip racy TestDirectDialWorks (#2419) ([libp2p/go-libp2p#2419](https://github.com/libp2p/go-libp2p/pull/2419))
  - swarm: change relay dial delay to 500ms (#2421) ([libp2p/go-libp2p#2421](https://github.com/libp2p/go-libp2p/pull/2421))
  - identify: disable racy TestLargeIdentifyMessage with race detector (#2401) ([libp2p/go-libp2p#2401](https://github.com/libp2p/go-libp2p/pull/2401))
  - swarm: make black hole detection configurable (#2403) ([libp2p/go-libp2p#2403](https://github.com/libp2p/go-libp2p/pull/2403))
  - net/mock: support ConnectionGater in MockNet (#2297) ([libp2p/go-libp2p#2297](https://github.com/libp2p/go-libp2p/pull/2297))
  - docs: Add a Github workflow for checking dead links (#2406) ([libp2p/go-libp2p#2406](https://github.com/libp2p/go-libp2p/pull/2406))
  - rcmgr: enable metrics by default (#2389) (#2409) ([libp2p/go-libp2p#2409](https://github.com/libp2p/go-libp2p/pull/2409))
  - chore: remove outdated info in README and link to libp2p-implementers slack (#2405) ([libp2p/go-libp2p#2405](https://github.com/libp2p/go-libp2p/pull/2405))
  - metrics: deduplicate code in examples (#2404) ([libp2p/go-libp2p#2404](https://github.com/libp2p/go-libp2p/pull/2404))
  - transport tests: remove mplex tests (#2402) ([libp2p/go-libp2p#2402](https://github.com/libp2p/go-libp2p/pull/2402))
  - swarm: implement Happy Eyeballs ranking (#2365) ([libp2p/go-libp2p#2365](https://github.com/libp2p/go-libp2p/pull/2365))
  - docs: fix some comments (#2391) ([libp2p/go-libp2p#2391](https://github.com/libp2p/go-libp2p/pull/2391))
  - metrics: provide separate docker-compose files for OSX and Linux (#2397) ([libp2p/go-libp2p#2397](https://github.com/libp2p/go-libp2p/pull/2397))
  - identify: use zero-alloc slice sorting function (#2396) ([libp2p/go-libp2p#2396](https://github.com/libp2p/go-libp2p/pull/2396))
  - rcmgr: move StatsTraceReporter to rcmgr package (#2388) ([libp2p/go-libp2p#2388](https://github.com/libp2p/go-libp2p/pull/2388))
  - swarm: implement blackhole detection (#2320) ([libp2p/go-libp2p#2320](https://github.com/libp2p/go-libp2p/pull/2320))
  - basichost / blankhost: wrap errors (#2331) ([libp2p/go-libp2p#2331](https://github.com/libp2p/go-libp2p/pull/2331))
  - network: don't allocate in DedupAddrs (#2395) ([libp2p/go-libp2p#2395](https://github.com/libp2p/go-libp2p/pull/2395))
  - rcmgr: test snapshot defaults and that we keep consistent defaults (#2315) ([libp2p/go-libp2p#2315](https://github.com/libp2p/go-libp2p/pull/2315))
  - rcmgr: register prometheus metrics with the libp2p registerer (#2370) ([libp2p/go-libp2p#2370](https://github.com/libp2p/go-libp2p/pull/2370))
  - metrics: make it possible to spin up Grafana using docker-compose (#2383) ([libp2p/go-libp2p#2383](https://github.com/libp2p/go-libp2p/pull/2383))
  - identify: set stream deadlines for Identify and Identify Push streams (#2382) ([libp2p/go-libp2p#2382](https://github.com/libp2p/go-libp2p/pull/2382))
  - fix: in the swarm move Connectedness emit after releasing conns (#2373) ([libp2p/go-libp2p#2373](https://github.com/libp2p/go-libp2p/pull/2373))
  - metrics: add example for metrics and dashboard (#2232) ([libp2p/go-libp2p#2232](https://github.com/libp2p/go-libp2p/pull/2232))
  - dashboards: finish metrics effort (#2362) ([libp2p/go-libp2p#2362](https://github.com/libp2p/go-libp2p/pull/2362))
  - transport tests: many streams and lots of data (#2296) ([libp2p/go-libp2p#2296](https://github.com/libp2p/go-libp2p/pull/2296))
  - webtransport: close the challenge stream after the Noise handshake (#2305) ([libp2p/go-libp2p#2305](https://github.com/libp2p/go-libp2p/pull/2305))
  - test: document why InstantTimer is required (#2351) ([libp2p/go-libp2p#2351](https://github.com/libp2p/go-libp2p/pull/2351))
  - rcmgr: fix link to dashboards in README (#2363) ([libp2p/go-libp2p#2363](https://github.com/libp2p/go-libp2p/pull/2363))
  - docs: fix some comments errors (#2356) ([libp2p/go-libp2p#2356](https://github.com/libp2p/go-libp2p/pull/2356))
  - release v0.28.0 (#2344) ([libp2p/go-libp2p#2344](https://github.com/libp2p/go-libp2p/pull/2344))
  - nat: add HasDiscoveredNAT method for checking NAT environments (#2358) ([libp2p/go-libp2p#2358](https://github.com/libp2p/go-libp2p/pull/2358))
  - swarm: fix stale DialBackoff comment (#2353) ([libp2p/go-libp2p#2353](https://github.com/libp2p/go-libp2p/pull/2353))
  - swarm: use RLock for DialBackoff reads (#2354) ([libp2p/go-libp2p#2354](https://github.com/libp2p/go-libp2p/pull/2354))
  - Clear stream scope if we error (#2345) ([libp2p/go-libp2p#2345](https://github.com/libp2p/go-libp2p/pull/2345))
  - changelog: improve description of smart dialing (#2342) ([libp2p/go-libp2p#2342](https://github.com/libp2p/go-libp2p/pull/2342))
  - swarm: make smart-dialing opt in (#2340) ([libp2p/go-libp2p#2340](https://github.com/libp2p/go-libp2p/pull/2340))
  - swarm: cleanup address filtering logic (#2333) ([libp2p/go-libp2p#2333](https://github.com/libp2p/go-libp2p/pull/2333))
  - chore: add 0.28.0 changelog (#2335) ([libp2p/go-libp2p#2335](https://github.com/libp2p/go-libp2p/pull/2335))
  - swarm: improve documentation for the DefaultDialRanker (#2336) ([libp2p/go-libp2p#2336](https://github.com/libp2p/go-libp2p/pull/2336))
  - holepunch: add metrics (#2246) ([libp2p/go-libp2p#2246](https://github.com/libp2p/go-libp2p/pull/2246))
  - swarm: implement smart dialing logic (#2260) ([libp2p/go-libp2p#2260](https://github.com/libp2p/go-libp2p/pull/2260))
  - revert "feat:add contexts to all peerstore methods (#2312)" (#2328) ([libp2p/go-libp2p#2328](https://github.com/libp2p/go-libp2p/pull/2328))
  - identify: don't save signed peer records (#2325) ([libp2p/go-libp2p#2325](https://github.com/libp2p/go-libp2p/pull/2325))
  - feat:add contexts to all peerstore methods (#2312) ([libp2p/go-libp2p#2312](https://github.com/libp2p/go-libp2p/pull/2312))
  - swarm: Dedup addresses to dial (#2322) ([libp2p/go-libp2p#2322](https://github.com/libp2p/go-libp2p/pull/2322))
  - identify: filter received addresses based on the node's remote address (#2300) ([libp2p/go-libp2p#2300](https://github.com/libp2p/go-libp2p/pull/2300))
  - update go-nat to v0.2.0, use context on AddMapping and RemoveMapping (#2319) ([libp2p/go-libp2p#2319](https://github.com/libp2p/go-libp2p/pull/2319))
  - transport integration tests: add tests for resource manager (#2285) ([libp2p/go-libp2p#2285](https://github.com/libp2p/go-libp2p/pull/2285))
  - identify: reject signed peer records on peer ID mismatch
  - identify: don't send default protocol version (#2303) ([libp2p/go-libp2p#2303](https://github.com/libp2p/go-libp2p/pull/2303))
  - metrics: add instance filter to all dashboards (#2301) ([libp2p/go-libp2p#2301](https://github.com/libp2p/go-libp2p/pull/2301))
  - identify: avoid spuriously triggering pushes (#2299) ([libp2p/go-libp2p#2299](https://github.com/libp2p/go-libp2p/pull/2299))
  - net/mock: mimic Swarm's event and notification behavior in MockNet (#2287) ([libp2p/go-libp2p#2287](https://github.com/libp2p/go-libp2p/pull/2287))
  - examples: fix flaky multipro TestMain (#2289) ([libp2p/go-libp2p#2289](https://github.com/libp2p/go-libp2p/pull/2289))
  - swarm: change maps with multiaddress keys to use strings (#2284) ([libp2p/go-libp2p#2284](https://github.com/libp2p/go-libp2p/pull/2284))
  - tests: add comprehensive end-to-end tests for connection gating (#2200) ([libp2p/go-libp2p#2200](https://github.com/libp2p/go-libp2p/pull/2200))
  - swarm: log unexpected listener errors (#2277) ([libp2p/go-libp2p#2277](https://github.com/libp2p/go-libp2p/pull/2277))
  - websocket: switch back to the gorilla library (#2280) ([libp2p/go-libp2p#2280](https://github.com/libp2p/go-libp2p/pull/2280))
  - quic: prioritise listen connections for reuse (#2262) ([libp2p/go-libp2p#2262](https://github.com/libp2p/go-libp2p/pull/2262))
  - quic virtual listener: don't panic when quic-go's accept call errors (#2276) ([libp2p/go-libp2p#2276](https://github.com/libp2p/go-libp2p/pull/2276))
  - tests: add docks for debugging flaky tests (#2216) ([libp2p/go-libp2p#2216](https://github.com/libp2p/go-libp2p/pull/2216))
  - webtransport: only add cert hashes if we already started listening (#2271) ([libp2p/go-libp2p#2271](https://github.com/libp2p/go-libp2p/pull/2271))
  - Revert "webtransport: initialize the certmanager when creating the transport (#2268)" (#2273) ([libp2p/go-libp2p#2273](https://github.com/libp2p/go-libp2p/pull/2273))
  - webtransport: initialize the certmanager when creating the transport (#2268) ([libp2p/go-libp2p#2268](https://github.com/libp2p/go-libp2p/pull/2268))
  - move NAT mapping logic out of the host, add tests for NAT handling ([libp2p/go-libp2p#2248](https://github.com/libp2p/go-libp2p/pull/2248))
  - githooks: add a githook to check that the test-plans go.mod is tidied (#2256) ([libp2p/go-libp2p#2256](https://github.com/libp2p/go-libp2p/pull/2256))
  - quic: fix race condition when generating random holepunch packet (#2263) ([libp2p/go-libp2p#2263](https://github.com/libp2p/go-libp2p/pull/2263))
  - swarm: remove unused variable in addrDial (#2257) ([libp2p/go-libp2p#2257](https://github.com/libp2p/go-libp2p/pull/2257))
- github.com/libp2p/go-libp2p-routing-helpers (v0.7.0 -> v0.7.1):
  - chore: release v0.7.1
  - fix: for comparallel never return nil channel for FindProvidersAsync
  - chore: rename DoNotWaitForStreamingResponses to DoNotWaitForSearchValue
  - feat: add DoNotWaitForStreamingResponses to ParallelRouter
  - chore: cleanup error handling in compparallel
  - fix: correctly handle errors in compparallel
  - fix: make the ProvideMany docs clearer
  - perf: remove goroutine that just waits before closing with a synchrous waitgroup
- github.com/libp2p/go-nat (v0.1.0 -> v0.2.0):
  - release v0.2.0 (#30) ([libp2p/go-nat#30](https://github.com/libp2p/go-nat/pull/30))
  - update deps, use contexts on UPnP functions (#29) ([libp2p/go-nat#29](https://github.com/libp2p/go-nat/pull/29))
  - sync: update CI config files (#28) ([libp2p/go-nat#28](https://github.com/libp2p/go-nat/pull/28))
  - sync: update CI config files (#24) ([libp2p/go-nat#24](https://github.com/libp2p/go-nat/pull/24))
- github.com/libp2p/go-yamux/v4 (v4.0.0 -> v4.0.1):
  - Release v4.0.1 ([libp2p/go-yamux#106](https://github.com/libp2p/go-yamux/pull/106))
  - fix: sendWindowUpdate respects deadlines (#105) ([libp2p/go-yamux#105](https://github.com/libp2p/go-yamux/pull/105))
- github.com/multiformats/go-multiaddr (v0.9.0 -> v0.10.1):
  - release v0.10.1 (#206) ([multiformats/go-multiaddr#206](https://github.com/multiformats/go-multiaddr/pull/206))
  - fix nat64 well-known prefix check (#205) ([multiformats/go-multiaddr#205](https://github.com/multiformats/go-multiaddr/pull/205))
  - release v0.10.0 (#204) ([multiformats/go-multiaddr#204](https://github.com/multiformats/go-multiaddr/pull/204))
  - add a Unique function (#203) ([multiformats/go-multiaddr#203](https://github.com/multiformats/go-multiaddr/pull/203))
  - manet: add function to test if address is NAT64 IPv4 converted IPv6 address (#202) ([multiformats/go-multiaddr#202](https://github.com/multiformats/go-multiaddr/pull/202))
  - sync: update CI config files (#190) ([multiformats/go-multiaddr#190](https://github.com/multiformats/go-multiaddr/pull/190))

</details>

### 👨‍👩‍👧‍👦 Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Henrique Dias | 14 | +3735/-17889 | 185 |
| Sukun | 28 | +5910/-957 | 100 |
| Jorropo | 40 | +2913/-2112 | 205 |
| Marten Seemann | 41 | +2926/-1833 | 163 |
| Marco Munizaga | 20 | +1559/-586 | 81 |
| Prem Chaitanya Prathi | 1 | +757/-740 | 61 |
| Laurent Senta | 2 | +69/-1094 | 32 |
| Marcin Rataj | 11 | +339/-198 | 22 |
| Steven Allen | 2 | +313/-161 | 9 |
| Will | 2 | +118/-211 | 9 |
| Adin Schmahmann | 4 | +275/-41 | 8 |
| Michael Muré | 1 | +113/-164 | 6 |
| Rod Vagg | 8 | +228/-46 | 28 |
| Gus Eggert | 5 | +156/-93 | 21 |
| Adrian Sutton | 1 | +190/-17 | 4 |
| Hlib Kanunnikov | 3 | +139/-40 | 9 |
| VM | 2 | +80/-79 | 49 |
| UnkwUsr | 1 | +0/-124 | 1 |
| Piotr Galar | 4 | +51/-59 | 5 |
| web3-bot | 3 | +22/-46 | 4 |
| Will Scott | 2 | +29/-28 | 6 |
| Prithvi Shahi | 2 | +40/-7 | 2 |
| Brad Fitzpatrick | 1 | +42/-2 | 2 |
| Steve Loeppky | 1 | +6/-23 | 2 |
| Sahib Yar | 1 | +4/-4 | 3 |
| Russell Dempsey | 2 | +4/-2 | 2 |
| Mohamed MHAMDI | 1 | +3/-3 | 1 |
| Bryan White | 1 | +2/-2 | 1 |
| Dennis Trautwein | 1 | +1/-1 | 1 |
| Antonio Navarro Perez | 1 | +0/-1 | 1 |

