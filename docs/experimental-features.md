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
- [Client mode DHT routing](#client-mode-dht-routing)
- [go-multiplex stream muxer](#go-multiplex-stream-muxer)
- [Raw leaves for unixfs files](#raw-leaves-for-unixfs-files)
- [ipfs filestore](#ipfs-filestore)
- [ipfs urlstore](#ipfs-urlstore)
- [BadgerDB datastore](#badger-datastore)
- [Private Networks](#private-networks)
- [ipfs p2p](#ipfs-p2p)
- [p2p http proxy](#p2p-http-proxy)
- [Circuit Relay](#circuit-relay)
- [Plugins](#plugins)
- [Directory Sharding / HAMT](#directory-sharding--hamt)
- [IPNS PubSub](#ipns-pubsub)
- [QUIC](#quic)
- [AutoRelay](#autorelay)
- [TLS 1.3 Handshake](#tls-13-as-default-handshake-protocol)
- [Strategic Providing](#strategic-providing)
- [Graphsync](graphsync)

---

## ipfs pubsub

### State

experimental, default-disabled.

### In Version

0.4.5

### How to enable

run your daemon with the `--enable-pubsub-experiment` flag. Then use the
`ipfs pubsub` commands.

### gossipsub

Gossipsub is a new, experimental routing protocol for pubsub that
should waste less bandwidth than floodsub, the current pubsub
protocol. It's backward compatible with floodsub so enabling this
feature shouldn't break compatibility with existing IPFS nodes.

You can enable gossipsub via configuration:
`ipfs config Pubsub.Router gossipsub`

### Message Signing

As of 0.4.18, go-ipfs signs all pubsub messages by default. For now, it doesn't
*reject* unsigned messages but it will in the future.

You can turn off message signing (not recommended unless you're using a private
network) by running:
`ipfs config Pubsub.DisableSigning true`

You can turn on strict signature verification (require that all messages be
signed) by running:
`ipfs config Pubsub.StrictSignatureVerification true`

(this last option will be set to true by default and eventually removed entirely)

### Road to being a real feature
- [ ] Needs more people to use and report on how well it works
- [ ] Needs authenticating modes to be implemented
- [ ] needs performance analyses to be done

---

## Client mode DHT routing

Allows the dht to be run in a mode that doesn't serve requests to the network,
saving bandwidth.

### State
stable

### In Version

0.5.0

### How to enable

run your daemon with the `--routing=dhtclient` flag.

---

## go-multiplex stream muxer
Adds support for using the go-multiplex stream muxer alongside (or instead of)
yamux and spdy. This multiplexer is far simpler, and uses less memory and
bandwidth than the others, but is lacking on congestion control and backpressure
logic. It is available to try out and experiment with.

### State
Stable

### In Version
0.4.5

### How to enable

To make it the default stream muxer, set the environment variable
`LIBP2P_MUX_PREFS` as follows:
```
export LIBP2P_MUX_PREFS="/mplex/6.7.0 /yamux/1.0.0 /spdy/3.1.0"
```

---

## Raw Leaves for unixfs files
Allows files to be added with no formatting in the leaf nodes of the graph.

### State
experimental.

### In Version
master, 0.4.5

### How to enable
Use `--raw-leaves` flag when calling `ipfs add`.

### Road to being a real feature
- [ ] Needs more people to use and report on how well it works.

---

## ipfs filestore
Allows files to be added without duplicating the space they take up on disk.

### State
experimental.

### In Version
master, 0.4.7

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

---

## ipfs urlstore
Allows ipfs to retrieve blocks contents via a URL instead of storing it in the datastore

### State
experimental.

### In Version
master, v0.4.17

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

---

## Private Networks

It allows ipfs to only connect to other peers who have a shared secret key.

### State
Experimental

### In Version
master, 0.4.7

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
- [ ] Needs more people to use and report on how well it works
- [ ] More documentation

---

## ipfs p2p

Allows tunneling of TCP connections through Libp2p streams. If you've ever used
port forwarding with SSH (the `-L` option in OpenSSH), this feature is quite
similar.

### State

Experimental

### In Version

master, 0.4.10

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
- [ ] Needs more people to use and report on how well it works / fits use cases
- [ ] More documentation
- [ ] Support other protocols (e.g, Unix domain sockets, WebSockets, etc.)

---

## p2p http proxy

Allows proxying of HTTP requests over p2p streams. This allows serving any standard HTTP app over p2p streams.

### State

Experimental

### In Version

master, 0.4.19

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

---

## Circuit Relay

Allows peers to connect through an intermediate relay node when there
is no direct connectivity.

### State
Experimental

### In Version
master, 0.4.11

### How to enable

The relay transport is enabled by default, which allows peers to dial through
a relay and listens for incoming relay connections. The transport can be disabled
by setting `Swarm.DisableRelay = true` in the configuration.

By default, peers don't act as intermediate nodes (relays). This can be enabled
by setting `Swarm.EnableRelayHop = true` in the configuration. Note that the
option needs to be set before online services are started to have an effect; an
already online node would have to be restarted.

### Basic Usage:

To connect peers QmA and QmB through a relay node QmRelay:

- Both peers should connect to the relay:
`ipfs swarm connect /transport/address/p2p/QmRelay`
- Peer QmA can then connect to peer QmB using the relay:
`ipfs swarm connect /p2p/QmRelay/p2p-circuit/p2p/QmB`

Peers can also connect with an unspecific relay address, which will
try to dial through known relays:
`ipfs swarm connect /p2p-circuit/p2p/QmB`

Peers can see their (unspecific) relay address in the output of
`ipfs swarm addrs listen`

### Road to being a real feature

- [ ] Needs more people to use it and report on how well it works.
- [ ] Advertise relay addresses to the DHT for NATed or otherwise unreachable
      peers.
- [ ] Active relay discovery for specific relay address advertisement. We would
      like advertised relay addresses to designate specific relays for efficient dialing.
- [ ] Dialing priorities for relay addresses; arguably, relay addresses should
      have lower priority than direct dials.

## Plugins

### In Version
0.4.11

### State
Experimental

Plugins allow adding functionality without the need to recompile the daemon.

### Basic Usage:

See [Plugin docs](./plugins.md)

### Road to being a real feature

- [ ] Better support for platforms other than Linux
- [ ] More plugins and plugin types
- [ ] Feedback on stability

---

## Badger datastore

### In Version

0.4.11

Badger-ds is new datastore implementation based on
https://github.com/dgraph-io/badger.
 

### Basic Usage

```
$ ipfs init --profile=badgerds
```
or install https://github.com/ipfs/ipfs-ds-convert/ and
```
[BACKUP ~/.ipfs]
$ ipfs config profile apply badgerds
$ ipfs-ds-convert convert
```

You can read more in the [datastore](./datastores.md#badgerds) documentation.

### Road to being a real feature

- [ ] Needs more testing
- [ ] Make sure there are no unknown major problems

## Directory Sharding / HAMT

### In Version
0.4.8

### State
Experimental

Allows creating directories with an unlimited number of entries - currently
size of unixfs directories is limited by the maximum block size

### Basic Usage:

```
ipfs config --json Experimental.ShardingEnabled true
```

### Road to being a real feature

- [ ] Make sure that objects that don't have to be sharded aren't
- [ ] Generalize sharding and define a new layer between IPLD and IPFS

---

## IPNS pubsub

### In Version

0.4.14

### State

Experimental, default-disabled.

Utilizes pubsub for publishing ipns records in real time.

When it is enabled:
- IPNS publishers push records to a name-specific pubsub topic,
  in addition to publishing to the DHT.
- IPNS resolvers subscribe to the name-specific topic on first
  resolution and receive subsequently published records through pubsub in real time. This makes subsequent resolutions instant, as they are resolved through the local cache. Note that the initial resolution still goes through the DHT, as there is no message history in pubsub.

Both the publisher and the resolver nodes need to have the feature enabled for it to work effectively.

### How to enable

run your daemon with the `--enable-namesys-pubsub` flag; enables pubsub.

### Road to being a real feature

- [ ] Needs more people to use and report on how well it works
- [ ] Add a mechanism for last record distribution on subscription,
  so that we don't have to hit the DHT for the initial resolution.
  Alternatively, we could republish the last record periodically.

---

## QUIC

### In Version

0.4.18

### State

Experiment, disabled by default

### How to enable

Modify your ipfs config:

```
ipfs config --json Experimental.QUIC true
```

For listening on a QUIC address, add it to the swarm addresses, e.g. `/ip4/0.0.0.0/udp/4001/quic`.


### Road to being a real feature

- [ ] The IETF QUIC specification needs to be finalized.
- [ ] Make sure QUIC connections work reliably
- [ ] Make sure QUIC connection offer equal or better performance than TCP connections on real-world networks
- [ ] Finalize libp2p-TLS handshake spec.


## AutoRelay

### In Version

0.4.19

### State

Experimental, disabled by default.

Automatically discovers relays and advertises relay addresses when the node is behind an impenetrable NAT.

### How to enable

Modify your ipfs config:

```
ipfs config --json Swarm.EnableAutoRelay true
```

Bootstrappers (and other public nodes) need to also enable the AutoNATService:
```
ipfs config --json Swarm.EnableAutoNATService true
```

### Road to being a real feature

- [ ] needs testing


## TLS 1.3 as default handshake protocol

### In Version

0.5.0

### State

Stable

---

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
    
---

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
