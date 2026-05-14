# Kubo changelog v0.17

## v0.17.0

### Overview

Below is an outline of all that is in this release, so you get a sense of all that's included.

- [Kubo changelog v0.17](#kubo-changelog-v017)
  - [v0.17.0](#v0170)
    - [Overview](#overview)
    - [🔦 Highlights](#-highlights)
      - [libp2p resource management enabled by default](#libp2p-resource-management-enabled-by-default)
      - [Implicit connection manager limits](#implicit-connection-manager-limits)
      - [TAR Response Format on Gateways](#tar-response-format-on-gateways)
      - [Dialling `/wss` peer behind a reverse proxy](#dialling-wss-peer-behind-a-reverse-proxy)
    - [Changelog](#changelog)
    - [Contributors](#contributors)

### 🔦 Highlights

<!-- TODO -->

#### libp2p resource management enabled by default

To help protect nodes from DoS (resource exhaustion) and eclipse attacks, 
go-libp2p released a [Network Resource Manager](https://github.com/libp2p/go-libp2p/tree/master/p2p/host/resource-manager) with a host of improvements throughout 2022.  

Kubo first [exposed this functionality in Kubo 0.13](https://github.com/ipfs/kubo/blob/master/docs/changelogs/v0.13.md#-libp2p-network-resource-manager-swarmresourcemgr), 
but it was disabled by default.

The resource manager is now enabled by default to protect nodes.  
The defaults balance providing protection from various attacks while still enabling normal usecases to work as expected.

If you want to adjust the defaults, then you can:
1. bound the amount of memory and file descriptors that libp2p will use with [Swarm.ResourceMgr.MaxMemory](https://github.com/ipfs/go-ipfs/blob/master/docs/config.md#swarmresourcemgrmaxmemory) 
and [Swarm.ResourceMgr.MaxFileDescriptors](https://github.com/ipfs/go-ipfs/blob/master/docs/config.md#swarmresourcemgrmaxfiledescriptors) and/or
2. override any specific resource scopes/limits with [Swarm.ResourceMgr.Limits](https://github.com/ipfs/go-ipfs/blob/master/docs/config.md#swarmresourcemgrlimits)

See [Swarm.ResourceMgr](https://github.com/ipfs/go-ipfs/blob/master/docs/config.md#swarmresourcemgr) for
1. what limits are set by default,
2. example override configuration, 
3. how to access Prometheus metrics and view Grafana dashboards of resource usage, and 
4. how to set explicit "allow lists" to protect against eclipse attacks. 

#### Implicit connection manager limits

Starting with this release, `ipfs init` will no longer store the default
[Connection Manager](https://github.com/ipfs/kubo/blob/master/docs/config.md#swarmconnmgr)
limits in the user config under `Swarm.ConnMgr`.

Users are still free to use this setting to set custom values, but for most use
cases, the defaults provided with the latest Kubo release should be sufficient.

To remove any custom limits and switch to the implicit defaults managed by Kubo:

```console
$ ipfs config --json Swarm.ConnMgr '{}'
```

We will be adjusting defaults in the future releases.

#### TAR Response Format on Gateways

Implemented [IPIP-288](https://github.com/ipfs/specs/pull/288) which adds
support for requesting deserialized UnixFS directory as a TAR stream.

HTTP clients can request TAR response by passing the `?format=tar` URL
parameter, or setting `Accept: application/x-tar` HTTP header:

```console
$ export DIR_CID=bafybeigccimv3zqm5g4jt363faybagywkvqbrismoquogimy7kvz2sj7sq
$ curl -H "Accept: application/x-tar" "http://127.0.0.1:8080/ipfs/$DIR_CID" > dir.tar
$ curl "http://127.0.0.1:8080/ipfs/$DIR_CID?format=tar" | tar xv
bafybeigccimv3zqm5g4jt363faybagywkvqbrismoquogimy7kvz2sj7sq
bafybeigccimv3zqm5g4jt363faybagywkvqbrismoquogimy7kvz2sj7sq/1 - Barrel - Part 1 - alt.txt
bafybeigccimv3zqm5g4jt363faybagywkvqbrismoquogimy7kvz2sj7sq/1 - Barrel - Part 1 - transcript.txt
bafybeigccimv3zqm5g4jt363faybagywkvqbrismoquogimy7kvz2sj7sq/1 - Barrel - Part 1.png
```

#### Dialling `/wss` peer behind a reverse proxy

This release resolves a regression introduced in Kubo 0.16, making it possible
again to connect to a peer over a WebSockets endpoint (`/wss`) that is
deployed behind a reverse proxy.

More details in [go-libp2p release notes](https://github.com/libp2p/go-libp2p/releases/tag/v0.23.3).

### Changelog

<details><summary>Full Changelog</summary>

- github.com/ipfs/kubo:
  - chore: bump version to v0.17.0 ([ipfs/kubo#9427](https://github.com/ipfs/kubo/pull/9427))
  - chore: bump version to v0.17.0-rc2 ([ipfs/kubo#9414](https://github.com/ipfs/kubo/pull/9414))
  - Doc improvements and changelog for resource manager (#9413) ([ipfs/kubo#9413](https://github.com/ipfs/kubo/pull/9413))
  - fix(docs): typo
  - docs: document /wss fixes in 0.17
  - refactor(config): remove Swarm.ConnMgr defaults
  - fix(config): skip nulls in ResourceMgr
  - Apply go fmt
  - Update core/node/libp2p/rcmgr_defaults.go
  - Remove limitation by HighWater param.
  - Fix RM errors when acceleratedDHT is active
  - docs: Deprecate Reframe on docs. (#9401) ([ipfs/kubo#9401](https://github.com/ipfs/kubo/pull/9401))
  - chore: bump version to v0.17.0-rc1 ([ipfs/kubo#9394](https://github.com/ipfs/kubo/pull/9394))
  - feat: Improve ResourceManager UX (#9338) ([ipfs/kubo#9338](https://github.com/ipfs/kubo/pull/9338))
  - feat: ipfs-webui 2.20.0
  - docs: note log tail is broken (#9383) ([ipfs/kubo#9383](https://github.com/ipfs/kubo/pull/9383))
  - feat(gateway): TAR response format (#9029) ([ipfs/kubo#9029](https://github.com/ipfs/kubo/pull/9029))
  - fix: error when using huge json limit file
  - chore: go-multicodec v0.7.0
  - fix: remove old unused buggy coredag code
  - feat: Add command line completion for fish
  - chore: delete snap configuration ([ipfs/kubo#9352](https://github.com/ipfs/kubo/pull/9352))
  - docs: update scoop package
  - docs: init release issue template improvement process v0.16.0 ([ipfs/kubo#9283](https://github.com/ipfs/kubo/pull/9283))
  - feat: add delegated routing metrics (#9354) ([ipfs/kubo#9354](https://github.com/ipfs/kubo/pull/9354))
  - chore: create v0.17.md changelog ([ipfs/kubo#9353](https://github.com/ipfs/kubo/pull/9353))
  - docs: pin remote arg
  - feat: webui@v2.19.0
  - test(car): export/import of (dag-)cbor/json codecs
  - add refs local alias repo ls (#9320) ([ipfs/kubo#9320](https://github.com/ipfs/kubo/pull/9320))
  - docs(cmds): Clarify block fetching of refs endpoint.
  - chore(cmds): dag import: use ipld legacy decode ([ipfs/kubo#9219](https://github.com/ipfs/kubo/pull/9219))
  - fix ipfs swarm peering crash in offline mode (#9261) ([ipfs/kubo#9261](https://github.com/ipfs/kubo/pull/9261))
  - feat: remove provider delay interval in bitswap (#9053) ([ipfs/kubo#9053](https://github.com/ipfs/kubo/pull/9053))
  - feat: --reset flag on swarm limit command (#9310) ([ipfs/kubo#9310](https://github.com/ipfs/kubo/pull/9310))
  - fix: add InlineDNSLink flag to PublicGateways config (#9328) ([ipfs/kubo#9328](https://github.com/ipfs/kubo/pull/9328))
  - docs: Fix typo and grammar in README
  - ci: add stylecheck to golangci-lint (#9334) ([ipfs/kubo#9334](https://github.com/ipfs/kubo/pull/9334))
  - Fix: `swarm stats all` command
  - Merge release v0.16.0 back into master ([ipfs/kubo#9324](https://github.com/ipfs/kubo/pull/9324))
  - fix: Set default Methods value to nil
  - docs: add WebTransport docs ([ipfs/kubo#9314](https://github.com/ipfs/kubo/pull/9314))
  - chore: bump version to 0.17.0-dev
- github.com/ipfs/go-delegated-routing (v0.6.0 -> v0.7.0):
  - Release v0.7.0
  - feat: add latency & count metrics for content routing client (#59) ([ipfs/go-delegated-routing#59](https://github.com/ipfs/go-delegated-routing/pull/59))
  - docs: add basic readme ([ipfs/go-delegated-routing#57](https://github.com/ipfs/go-delegated-routing/pull/57))
  - sync: update CI config files ([ipfs/go-delegated-routing#40](https://github.com/ipfs/go-delegated-routing/pull/40))
  - added link to reframe blog post (#54) ([ipfs/go-delegated-routing#54](https://github.com/ipfs/go-delegated-routing/pull/54))
- github.com/ipfs/go-ipfs-files (v0.1.1 -> v0.2.0):
  - Release v0.2.0
  - fix: error when TAR has files outside of root (#56) ([ipfs/go-ipfs-files#56](https://github.com/ipfs/go-ipfs-files/pull/56))
  - sync: update CI config files ([ipfs/go-ipfs-files#55](https://github.com/ipfs/go-ipfs-files/pull/55))
  - chore(Directory): add DirIterator API restriction: iterate only once
- github.com/ipfs/go-unixfs (v0.4.0 -> v0.4.1):
  - Update version.json
  - Fix: panic when childer is nil (#127) ([ipfs/go-unixfs#127](https://github.com/ipfs/go-unixfs/pull/127))
  - sync: update CI config files (#125) ([ipfs/go-unixfs#125](https://github.com/ipfs/go-unixfs/pull/125))
- github.com/ipld/go-ipld-prime (v0.18.0 -> v0.19.0):
  - Prepare v0.19.0
  - fix: correct json codec links & bytes handling
  - test(basicnode): increase test coverage for int and map types (#454) ([ipld/go-ipld-prime#454](https://github.com/ipld/go-ipld-prime/pull/454))
  - fix: remove reliance on ioutil
  - run gofmt -s
  - bump go.mod to Go 1.18 and run go fix
  - feat: add kinded union to gendemo
- github.com/libp2p/go-libp2p (v0.23.2 -> v0.23.4):
  - Release v0.23.4 (#1864) ([libp2p/go-libp2p#1864](https://github.com/libp2p/go-libp2p/pull/1864))
  - release v0.23.3
  - websocket: set the HTTP host header in WSS
- github.com/libp2p/go-netroute (v0.2.0 -> v0.2.1):
  - v0.2.1 ([libp2p/go-netroute#27](https://github.com/libp2p/go-netroute/pull/27))
  - fix(phys-addr-length): fix physical address length mismatch ([libp2p/go-netroute#29](https://github.com/libp2p/go-netroute/pull/29))
  - compare priority if route rule's dst mask is same size
  - compare priority if route rule's dst mask is same size
  - sync: update CI config files (#24) ([libp2p/go-netroute#24](https://github.com/libp2p/go-netroute/pull/24))
- github.com/marten-seemann/qpack (v0.2.1 -> v0.3.0):
  - update to Ginkgo v2 (#30) ([marten-seemann/qpack#30](https://github.com/marten-seemann/qpack/pull/30))
  - return write error when encoding header fields (#28) ([marten-seemann/qpack#28](https://github.com/marten-seemann/qpack/pull/28))
  - update Go versions (#29) ([marten-seemann/qpack#29](https://github.com/marten-seemann/qpack/pull/29))
  - remove CircleCI build status from README
  - add link to QPACK RFC to README
  - remove build constraint from fuzzer ([marten-seemann/qpack#24](https://github.com/marten-seemann/qpack/pull/24))
- github.com/multiformats/go-multicodec (v0.6.0 -> v0.7.0):
  - feat: update ./multicodec/table.csv ([multiformats/go-multicodec#71](https://github.com/multiformats/go-multicodec/pull/71))

</details>

### Contributors

| Contributor | Commits | Lines ± | Files Changed |
|-------------|---------|---------|---------------|
| Antonio Navarro Perez | 11 | +780/-987 | 31 |
| Marcin Rataj | 14 | +791/-543 | 26 |
| web3-bot | 7 | +393/-427 | 71 |
| galargh | 20 | +309/-277 | 21 |
| Gus Eggert | 5 | +358/-222 | 58 |
| Henrique Dias | 3 | +409/-30 | 13 |
| Dustin Long | 1 | +314/-0 | 2 |
| Marco Munizaga | 2 | +211/-46 | 11 |
| Rod Vagg | 4 | +188/-62 | 13 |
| Jorropo | 2 | +4/-219 | 5 |
| Steve Loeppky | 1 | +115/-72 | 4 |
| Andreas Källberg | 1 | +145/-5 | 5 |
| Lucas Molas | 3 | +76/-53 | 9 |
| snyh | 2 | +36/-18 | 2 |
| Piotr Galar | 2 | +31/-4 | 2 |
| Ondrej Kokes | 1 | +25/-4 | 2 |
| Marten Seemann | 6 | +14/-14 | 14 |
| Yann Autissier | 1 | +14/-4 | 1 |
| maxos | 1 | +8/-1 | 2 |
| reidlw | 1 | +1/-4 | 1 |
| Russell Dempsey | 2 | +4/-1 | 2 |
| Ian Davis | 1 | +4/-0 | 2 |
| Daniel Norman | 1 | +3/-1 | 1 |
| Will Scott | 1 | +1/-1 | 1 |
| Nikhilesh Susarla | 1 | +2/-0 | 2 |
| Jamie Wilkinson | 1 | +1/-1 | 1 |
| Will | 1 | +0/-1 | 1 |
