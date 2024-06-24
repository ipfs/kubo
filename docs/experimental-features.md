# Experimental features of Kubo

This document contains a list of experimental features in Kubo.
These features, commands, and APIs aren't mature, and you shouldn't rely on them.
Once they reach maturity, there's going to be mention in the changelog and
release posts. If they don't reach maturity, the same applies, and their code is
removed.

Subscribe to https://github.com/ipfs/kubo/issues/3397 to get updates.

When you add a new experimental feature to kubo or change an experimental
feature, you MUST please make a PR updating this document, and link the PR in
the above issue.

- [Raw leaves for unixfs files](#raw-leaves-for-unixfs-files)
- [ipfs filestore](#ipfs-filestore)
- [ipfs urlstore](#ipfs-urlstore)
- [Private Networks](#private-networks)
- [ipfs p2p](#ipfs-p2p)
- [p2p http proxy](#p2p-http-proxy)
- [FUSE](#fuse)
- [Plugins](#plugins)
- [Directory Sharding / HAMT](#directory-sharding--hamt)
- [IPNS PubSub](#ipns-pubsub)
- [AutoRelay](#autorelay)
- [Strategic Providing](#strategic-providing)
- [Graphsync](#graphsync)
- [Noise](#noise)
- [Optimistic Provide](#optimistic-provide)
- [HTTP Gateway over Libp2p](#http-gateway-over-libp2p)

---

## Raw Leaves for unixfs files

Allows files to be added with no formatting in the leaf nodes of the graph.

### State

Stable but not used by default.

### In Version

0.4.5

### How to enable

Use `--raw-leaves` flag when calling `ipfs add`. This will save some space when adding files.

### Road to being a real feature

Enabling this feature _by default_ will change the CIDs (hashes) of all newly imported files and will prevent newly imported files from deduplicating against previously imported files. While we do intend on enabling this by default, we plan on doing so once we have a large batch of "hash-changing" features we can enable all at once.

## ipfs filestore

Allows files to be added without duplicating the space they take up on disk.

### State

Experimental.

### In Version

0.4.7

### How to enable

Modify your ipfs config:
```
ipfs config --json Experimental.FilestoreEnabled true
```

Then restart your IPFS node to reload your config.

Finally, when adding files with ipfs add, pass the --nocopy flag to use the
filestore instead of copying the files into your local IPFS repo.

### Road to being a real feature

- [ ] Needs more people to use and report on how well it works.
- [ ] Need to address error states and failure conditions
- [ ] Need to write docs on usage, advantages, disadvantages
- [ ] Need to merge utility commands to aid in maintenance and repair of filestore

## ipfs urlstore

Allows ipfs to retrieve blocks contents via a URL instead of storing it in the datastore

### State

Experimental.

### In Version

v0.4.17

### How to enable

Modify your ipfs config:
```
ipfs config --json Experimental.UrlstoreEnabled true
```

And then add a file at a specific URL using `ipfs urlstore add <url>`

### Road to being a real feature
- [ ] Needs more people to use and report on how well it works.
- [ ] Need to address error states and failure conditions
- [ ] Need to write docs on usage, advantages, disadvantages
- [ ] Need to implement caching
- [ ] Need to add metrics to monitor performance

## Private Networks

It allows ipfs to only connect to other peers who have a shared secret key.

### State

Stable but not quite ready for prime-time.

> [!WARNING]
> Limited to TCP transport, comes with overhead of double-encryption. See details below.

### In Version

0.4.7

### How to enable

Generate a pre-shared-key using [ipfs-swarm-key-gen](https://github.com/Kubuxu/go-ipfs-swarm-key-gen)):
```
go install github.com/Kubuxu/go-ipfs-swarm-key-gen/ipfs-swarm-key-gen@latest
ipfs-swarm-key-gen > ~/.ipfs/swarm.key
```

To join a given private network, get the key file from someone in the network
and save it to `~/.ipfs/swarm.key` (If you are using a custom `$IPFS_PATH`, put
it in there instead).

When using this feature, you will not be able to connect to the default bootstrap
nodes (Since we aren't part of your private network) so you will need to set up
your own bootstrap nodes.

First, to prevent your node from even trying to connect to the default bootstrap nodes, run:
```bash
ipfs bootstrap rm --all
```

Then add your own bootstrap peers with:
```bash
ipfs bootstrap add <multiaddr>
```

For example:
```
ipfs bootstrap add /ip4/104.236.76.40/tcp/4001/p2p/QmSoLV4Bbm51jM9C4gDYZQ9Cy3U6aXMJDAbzgu2fzaDs64
```

Bootstrap nodes are no different from all other nodes in the network apart from
the function they serve.

To be extra cautious, You can also set the `LIBP2P_FORCE_PNET` environment
variable to `1` to force the usage of private networks. If no private network is
configured, the daemon will fail to start.

### Road to being a real feature

- [x] Needs more people to use and report on how well it works
- [ ] More documentation
- [ ] Improve / future proof libp2p support (see [libp2p/specs#489](https://github.com/libp2p/specs/issues/489))
  - [ ] Currently limited to TCP-only, and double-encrypts all data sent on TCP. This is slow.
  - [ ] Does not work with QUIC: [go-libp2p#1432](https://github.com/libp2p/go-libp2p/issues/1432)
- [ ] Needs better tooling/UX
  - [ ] Detect lack of peers when swarm key is present and prompt user to set up bootstrappers/peering
  - [ ] ipfs-webui will not load  unless blocks are present in private swarm. Detect it and prompt user to import CAR with webui.

## ipfs p2p

Allows tunneling of TCP connections through Libp2p streams. If you've ever used
port forwarding with SSH (the `-L` option in OpenSSH), this feature is quite
similar.

### State

Experimental, will be stabilized in 0.6.0

### In Version

0.4.10

### How to enable

The `p2p` command needs to be enabled in the config:

```sh
> ipfs config --json Experimental.Libp2pStreamMounting true
```

### How to use

**Netcat example:**

First, pick a protocol name for your application. Think of the protocol name as
a port number, just significantly more user-friendly. In this example, we're
going to use `/x/kickass/1.0`.

***Setup:***

1. A "server" node with peer ID `$SERVER_ID`
2. A "client" node.

***On the "server" node:***

First, start your application and have it listen for TCP connections on
port `$APP_PORT`.

Then, configure the p2p listener by running:

```sh
> ipfs p2p listen /x/kickass/1.0 /ip4/127.0.0.1/tcp/$APP_PORT
```

This will configure IPFS to forward all incoming `/x/kickass/1.0` streams to
`127.0.0.1:$APP_PORT` (opening a new connection to `127.0.0.1:$APP_PORT` per
incoming stream.

***On the "client" node:***

First, configure the client p2p dialer, so that it forwards all inbound
connections on `127.0.0.1:SOME_PORT` to the server node listening
on `/x/kickass/1.0`.

```sh
> ipfs p2p forward /x/kickass/1.0 /ip4/127.0.0.1/tcp/$SOME_PORT /p2p/$SERVER_ID
```

Next, have your application open a connection to `127.0.0.1:$SOME_PORT`. This
connection will be forwarded to the service running on `127.0.0.1:$APP_PORT` on
the remote machine. You can test it with netcat:

***On "server" node:***
```sh
> nc -v -l -p $APP_PORT
```

***On "client" node:***
```sh
> nc -v 127.0.0.1 $SOME_PORT
```

You should now see that a connection has been established and be able to
exchange messages between netcat instances.

(note that depending on your netcat version you may need to drop the `-v` flag)

**SSH example**

**Setup:**

1. A "server" node with peer ID `$SERVER_ID` and running ssh server on the
   default port.
2. A "client" node.

_you can get `$SERVER_ID` by running `ipfs id -f "<id>\n"`_

***First, on the "server" node:***

```sh
ipfs p2p listen /x/ssh /ip4/127.0.0.1/tcp/22
```

***Then, on "client" node:***

```sh
ipfs p2p forward /x/ssh /ip4/127.0.0.1/tcp/2222 /p2p/$SERVER_ID
```

You should now be able to connect to your ssh server through a libp2p connection
with `ssh [user]@127.0.0.1 -p 2222`.


### Road to being a real feature

- [ ] More documentation

## p2p http proxy

Allows proxying of HTTP requests over p2p streams. This allows serving any standard HTTP app over p2p streams.

### State

Experimental

### In Version

0.4.19

### How to enable

The `p2p` command needs to be enabled in the config:

```sh
> ipfs config --json Experimental.Libp2pStreamMounting true
```

On the client, the p2p HTTP proxy needs to be enabled in the config:

```sh
> ipfs config --json Experimental.P2pHttpProxy true
```

### How to use

**Netcat example:**

First, pick a protocol name for your application. Think of the protocol name as
a port number, just significantly more user-friendly. In this example, we're
going to use `/http`.

***Setup:***

1. A "server" node with peer ID `$SERVER_ID`
2. A "client" node.

***On the "server" node:***

First, start your application and have it listen for TCP connections on
port `$APP_PORT`.

Then, configure the p2p listener by running:

```sh
> ipfs p2p listen --allow-custom-protocol /http /ip4/127.0.0.1/tcp/$APP_PORT
```

This will configure IPFS to forward all incoming `/http` streams to
`127.0.0.1:$APP_PORT` (opening a new connection to `127.0.0.1:$APP_PORT` per incoming stream.

***On the "client" node:***

Next, have your application make a http request to `127.0.0.1:8080/p2p/$SERVER_ID/http/$FORWARDED_PATH`. This
connection will be forwarded to the service running on `127.0.0.1:$APP_PORT` on
the remote machine (which needs to be a http server!) with path `$FORWARDED_PATH`. You can test it with netcat:

***On "server" node:***
```sh
> echo -e "HTTP/1.1 200\nContent-length: 11\n\nIPFS rocks!" | nc -l -p $APP_PORT
```

***On "client" node:***
```sh
> curl http://localhost:8080/p2p/$SERVER_ID/http/
```

You should now see the resulting HTTP response: IPFS rocks!

### Custom protocol names

We also support the use of protocol names of the form /x/$NAME/http where $NAME doesn't contain any "/"'s

### Road to being a real feature

- [ ] Needs p2p streams to graduate from experiments
- [ ] Needs more people to use and report on how well it works / fits use cases
- [ ] More documentation
- [ ] Need better integration with the subdomain gateway feature.

## FUSE

FUSE makes it possible to mount `/ipfs` and `/ipns` namespaces in your OS,
allowing arbitrary apps access to IPFS using a subset of filesystem abstractions.

It is considered  EXPERIMENTAL due to limited (and buggy) support on some platforms.

See [fuse.md](./fuse.md) for more details.

## Plugins

### In Version
0.4.11

### State
Experimental

Plugins allow adding functionality without the need to recompile the daemon.

### Basic Usage:

See [Plugin docs](./plugins.md)

### Road to being a real feature

- [x] More plugins and plugin types
- [ ] A way to reliably build and distribute plugins.
- [ ] Better support for platforms other than Linux & MacOS
- [ ] Feedback on stability

## Directory Sharding / HAMT

### In Version

- 0.4.8:
  - Introduced `Experimental.ShardingEnabled` which enabled sharding globally.
  - All-or-nothing, unnecessary sharding of small directories.

- 0.11.0 :
  - Removed support for `Experimental.ShardingEnabled`
  - Replaced with automatic sharding based on the block size

### State

Replaced by autosharding.

The `Experimental.ShardingEnabled` config field is no longer used, please remove it from your configs.

kubo now automatically shards when directory block is bigger than 256KB, ensuring every block is small enough to be exchanged with other peers

## IPNS pubsub

### In Version

0.4.14 :
  - Introduced

0.5.0 :
   - No longer needs to use the DHT for the first resolution
   - When discovering PubSub peers via the DHT, the DHT key is different from previous versions
      - This leads to 0.5 IPNS pubsub peers and 0.4 IPNS pubsub peers not being able to find each other in the DHT
   - Robustness improvements

0.11.0 :
  - Can be enabled via `Ipns.UsePubsub` flag in config

### State

Experimental, default-disabled.

Utilizes pubsub for publishing ipns records in real time.

When it is enabled:
- IPNS publishers push records to a name-specific pubsub topic,
  in addition to publishing to the DHT.
- IPNS resolvers subscribe to the name-specific topic on first
  resolution and receive subsequently published records through pubsub in real time.
  This makes subsequent resolutions instant, as they are resolved through the local cache.

Both the publisher and the resolver nodes need to have the feature enabled for it to work effectively.

Note: While IPNS pubsub has been available since 0.4.14, it received major changes in 0.5.0.
Users interested in this feature should upgrade to at least 0.5.0

### How to enable

Run your daemon with the `--enable-namesys-pubsub` flag
or modify your ipfs config and restart the daemon:
```
ipfs config --json Ipns.UsePubsub true
```

NOTE:
- This feature implicitly enables [ipfs pubsub](#ipfs-pubsub).
- Passing `--enable-namesys-pubsub` CLI flag overrides `Ipns.UsePubsub` config.

### Road to being a real feature

- [ ] Needs more people to use and report on how well it works
- [ ] Pubsub enabled as a real feature

## AutoRelay

### In Version

- 0.4.19 :
  - Introduced Circuit Relay v1
- 0.11.0 :
  - Deprecated v1
  - Introduced [Circuit Relay v2](https://github.com/libp2p/specs/blob/master/relay/circuit-v2.md)

### State

Experimental, disabled by default.

Automatically discovers relays and advertises relay addresses when the node is behind an impenetrable NAT.

### How to enable

Modify your ipfs config:

```
ipfs config --json Swarm.RelayClient.Enabled true
```

### Road to being a real feature

- [ ] needs testing
- [ ] needs to be automatically enabled when AutoNAT detects node is behind an impenetrable NAT.


## Strategic Providing

### State

Experimental, disabled by default.

Replaces the existing provide mechanism with a robust, strategic provider system. Currently enabling this option will provide nothing.

### How to enable

Modify your ipfs config:

```
ipfs config --json Experimental.StrategicProviding true
```

### Road to being a real feature

- [ ] needs real-world testing
- [ ] needs adoption
- [ ] needs to support all provider subsystem features
    - [X] provide nothing
    - [ ] provide roots
    - [ ] provide all
    - [ ] provide strategic

## GraphSync

### State

Removed, no plans to reintegrate either as experimental or stable feature.

[Trustless Gateway over Libp2p](#http-gateway-over-libp2p) should be easier to use for unixfs usecases and support basic wildcard car streams for non unixfs.

See https://github.com/ipfs/kubo/pull/9747 for more information.

## Noise

### State

Stable, enabled by default

[Noise](https://github.com/libp2p/specs/tree/master/noise) libp2p transport based on the [Noise Protocol Framework](https://noiseprotocol.org/noise.html). While TLS remains the default transport in Kubo, Noise is easier to implement and is thus the "interop" transport between IPFS and libp2p implementations.

## Optimistic Provide

### In Version

0.20.0

### State

Experimental, disabled by default.

When the Amino DHT client tries to store a provider in the DHT, it typically searches for the 20 peers that are closest to the
target key. However, this process can be time-consuming, as the search terminates only after no closer peers are found
among the three currently (during the query) known closest ones. In cases where these closest peers are slow to respond
(which often happens if they are located at the edge of the DHT network), the query gets blocked by the slowest peer.

To address this issue, the `OptimisticProvide` feature can be enabled. This feature allows the client to estimate the
network size and determine how close a peer _likely_ needs to be to the target key to be within the 20 closest peers.
While searching for the closest peers in the DHT, the client will _optimistically_ store the provider record with peers
and abort the query completely when the set of currently known 20 closest peers are also _likely_ the actual 20 closest
ones. This heuristic approach can significantly speed up the process, resulting in a speed improvement of 2x to >10x.

When it is enabled:

- Amino DHT provide operations should complete much faster than with it disabled
- This can be tested with commands such as `ipfs routing provide`

**Tradeoffs**

There are now the classic client, the accelerated DHT client, and optimistic provide that improve the provider process.
There are different trade-offs with all of them. The accelerated DHT client is still faster to provide large amounts
of provider records at the cost of high resource requirements. Optimistic provide doesn't have the high resource
requirements but might not choose optimal peers and is not as fast as the accelerated client, but still much faster
than the classic client.

**Caveats:**

1. Providing optimistically requires a current network size estimation. This estimation is calculated through routing
   table refresh queries and is only available after the daemon has been running for some time. If there is no network
   size estimation available the client will transparently fall back to the classic approach.
2. The chosen peers to store the provider records might not be the actual closest ones. Measurements showed that this
   is not a problem.
3. The optimistic provide process returns already after 15 out of the 20 provider records were stored with peers. The
   reasoning here is that one out of the remaining 5 peers are very likely to time out and delay the whole process. To
   limit the number of in-flight async requests there is the second `OptimisticProvideJobsPoolSize` setting. Currently,
   this is set to 60. This means that at most 60 parallel background requests are allowed to be in-flight. If this
   limit is exceeded optimistic provide will block until all 20 provider records are written. This is still 2x faster
   than the classic approach but not as fast as returning early which yields >10x speed-ups.
4. Since the in-flight background requests are likely to time out, they are not consuming many resources and the job
   pool size could probably be much higher.

For more information, see:

- Project doc: https://protocollabs.notion.site/Optimistic-Provide-2c79745820fa45649d48de038516b814
- go-libp2p-kad-dht: https://github.com/libp2p/go-libp2p-kad-dht/pull/783

### Configuring
To enable:

```
ipfs config --json Experimental.OptimisticProvide true
```

If you want to change the `OptimisticProvideJobsPoolSize` setting from its default of 60:

```
ipfs config --json Experimental.OptimisticProvideJobsPoolSize 120
```

### Road to being a real feature

- [ ] Needs more people to use and report on how well it works
- [ ] Should prove at least equivalent availability of provider records as the classic approach

## HTTP Gateway over Libp2p

### In Version

0.23.0

### State

Experimental, disabled by default.

Enables serving a subset of the [IPFS HTTP Gateway](https://specs.ipfs.tech/http-gateways/) semantics over libp2p `/http/1.1` protocol.

Notes:
- This feature only about serving verifiable gateway requests over libp2p:
  - Deserialized responses are not supported.
  - Only operate on `/ipfs` resources (no `/ipns` atm)
  - Only support requests for `application/vnd.ipld.raw` and
    `application/vnd.ipld.car` (from [Trustless Gateway Specification](https://specs.ipfs.tech/http-gateways/trustless-gateway/),
    where data integrity can be verified).
  - Only serve data that is already local to the node (i.e. similar to a
    [`Gateway.NoFetch`](https://github.com/ipfs/kubo/blob/master/docs/config.md#gatewaynofetch))
- While Kubo currently mounts the gateway API at the root (i.e. `/`) of the
  libp2p `/http/1.1` protocol, that is subject to change.
  - The way to reliably discover where a given HTTP protocol is mounted on a
    libp2p endpoint is via the `.well-known/libp2p` resource specified in the
    [http+libp2p specification](https://github.com/libp2p/specs/pull/508)
    - The identifier of the protocol mount point under `/http/1.1` listener is
      `/ipfs/gateway`, as noted in
      [ipfs/specs#434](https://github.com/ipfs/specs/pull/434).

### How to enable

Modify your ipfs config:

```
ipfs config --json Experimental.GatewayOverLibp2p true
```

### Road to being a real feature

- [ ] Needs more people to use and report on how well it works
- [ ] Needs UX work for exposing non-recursive "HTTP transport" (NoFetch) over both libp2p and plain TCP (and sharing the configuration)
- [ ] Needs a mechanism for HTTP handler to signal supported features ([IPIP-425](https://github.com/ipfs/specs/pull/425))
- [ ] Needs an option for Kubo to detect peers that have it enabled and prefer HTTP transport before falling back to bitswap (and use CAR if peer supports dag-scope=entity from [IPIP-402](https://github.com/ipfs/specs/pull/402))

## Accelerated DHT Client

This feature now lives at [`Routing.AcceleratedDHTClient`](https://github.com/ipfs/kubo/blob/master/docs/config.md#routingaccelerateddhtclient).
