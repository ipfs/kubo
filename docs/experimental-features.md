# Experimental features of go-ipfs

This document contains a list of experimental features in go-ipfs.
These features, commands, and APIs aren't mature, and you shouldn't rely on them.
Once they reach maturity, there's going to be mention in the changelog and
release posts. If they don't reach maturity, the same applies, and their code is
removed.

Subscribe to https://github.com/ipfs/go-ipfs/issues/3397 to get updates.

When you add a new experimental feature to go-ipfs or change an experimental
feature, you MUST please make a PR updating this document, and link the PR in
the above issue.

- [ipfs pubsub](#ipfs-pubsub)
- [Raw leaves for unixfs files](#raw-leaves-for-unixfs-files)
- [ipfs filestore](#ipfs-filestore)
- [ipfs urlstore](#ipfs-urlstore)
- [Private Networks](#private-networks)
- [ipfs p2p](#ipfs-p2p)
- [p2p http proxy](#p2p-http-proxy)
- [Plugins](#plugins)
- [Directory Sharding / HAMT](#directory-sharding--hamt)
- [IPNS PubSub](#ipns-pubsub)
- [AutoRelay](#autorelay)
- [Strategic Providing](#strategic-providing)
- [Graphsync](#graphsync)
- [Noise](#noise)

---

## ipfs pubsub

### State

Candidate, disabled by default but will be enabled by default in 0.6.0.

### In Version

0.4.5

### How to enable

run your daemon with the `--enable-pubsub-experiment` flag. Then use the
`ipfs pubsub` commands.

Configuration documentation can be found in [./config.md]()

### Road to being a real feature

- [ ] Needs to not impact peers who don't use pubsub:
      https://github.com/libp2p/go-libp2p-pubsub/issues/332

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

### In Version

0.4.7

### How to enable

Generate a pre-shared-key using [ipfs-swarm-key-gen](https://github.com/Kubuxu/go-ipfs-swarm-key-gen)):
```
go get github.com/Kubuxu/go-ipfs-swarm-key-gen/ipfs-swarm-key-gen
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
- [ ] Needs better tooling/UX.

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
0.4.8

### State
Experimental

Allows creating directories with an unlimited number of entries.

**Caveats:**
1. right now it is a GLOBAL FLAG which will impact the final CID of all directories produced by `ipfs.add` (even the small ones)
2. currently size of unixfs directories is limited by the maximum block size

### Basic Usage:

```
ipfs config --json Experimental.ShardingEnabled true
```

### Road to being a real feature

- [ ] Make sure that objects that don't have to be sharded aren't
- [ ] Generalize sharding and define a new layer between IPLD and IPFS

## IPNS pubsub

### In Version

0.4.14 :
  - Introduced

0.5.0 : 
   - No longer needs to use the DHT for the first resolution
   - When discovering PubSub peers via the DHT, the DHT key is different from previous versions
      - This leads to 0.5 IPNS pubsub peers and 0.4 IPNS pubsub peers not being able to find each other in the DHT
   - Robustness improvements

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

run your daemon with the `--enable-namesys-pubsub` flag; enables pubsub.

### Road to being a real feature

- [ ] Needs more people to use and report on how well it works
- [ ] Pubsub enabled as a real feature

## AutoRelay

### In Version

0.4.19

### State

Experimental, disabled by default.

Automatically discovers relays and advertises relay addresses when the node is behind an impenetrable NAT.

### How to enable

Modify your ipfs config:

```
ipfs config --json Swarm.EnableRelayHop false
ipfs config --json Swarm.EnableAutoRelay true
```

NOTE: Ensuring `Swarm.EnableRelayHop` is _false_ is extremely important here. If you set it to true, you will _act_ as a public relay for the rest of the network instead of _using_ the public relays.

### Road to being a real feature

- [ ] needs testing

## Strategic Providing

### State

Experimental, disabled by default.

Replaces the existing provide mechanism with a robust, strategic provider system.

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

Experimental, disabled by default.

[GraphSync](https://github.com/ipfs/go-graphsync) is the next-gen graph exchange
protocol for IPFS.

When this feature is enabled, IPFS will make files available over the graphsync
protocol. However, IPFS will not currently use this protocol to _fetch_ files.

### How to enable

Modify your ipfs config:

```
ipfs config --json Experimental.GraphsyncEnabled true
```

### Road to being a real feature

- [ ] We need to confirm that it can't be used to DoS a node. The server-side logic for GraphSync is quite complex and, if we're not careful, the server might end up performing unbounded work when responding to a malicious request.

## Noise

### State

Experimental, enabled by default

[Noise](https://github.com/libp2p/specs/tree/master/noise) libp2p transport based on the [Noise Protocol Framework](https://noiseprotocol.org/noise.html). While TLS remains the default transport in go-ipfs, Noise is easier to implement and will thus serve as the "interop" transport between IPFS and libp2p implementations, eventually replacing SECIO.

### How to enable

While the Noise transport is now shipped and enabled by default in go-ipfs, it won't be used by default for most connections because TLS and SECIO are currently preferred. If you'd like to test out the Noise transport, you can increase the priority of the noise transport:

```
ipfs config --json Swarm.Transports.Security.Noise 1
```

Or even disable TLS and/or SECIO (not recommended for the moment):

```
ipfs config --json Swarm.Transports.Security.TLS false
ipfs config --json Swarm.Transports.Security.SECIO false
```

### Road to being a real feature

- [ ] Needs real-world testing.
- [ ] Ideally a js-ipfs and a rust-ipfs release would include support for Noise.
