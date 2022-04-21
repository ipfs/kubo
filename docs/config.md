# The Kubo config file

The Kubo (go-ipfs) config file is a JSON document located at `$IPFS_PATH/config`. It
is read once at node instantiation, either for an offline command, or when
starting the daemon. Commands that execute on a running daemon do not read the
config file at runtime.

# Table of Contents

- [The Kubo config file](#the-kubo-config-file)
- [Table of Contents](#table-of-contents)
  - [Profiles](#profiles)
  - [Types](#types)
    - [`flag`](#flag)
    - [`priority`](#priority)
    - [`strings`](#strings)
    - [`duration`](#duration)
    - [`optionalInteger`](#optionalinteger)
    - [`optionalBytes`](#optionalbytes)
    - [`optionalString`](#optionalstring)
    - [`optionalDuration`](#optionalduration)
  - [`Addresses`](#addresses)
    - [`Addresses.API`](#addressesapi)
    - [`Addresses.Gateway`](#addressesgateway)
    - [`Addresses.Swarm`](#addressesswarm)
    - [`Addresses.Announce`](#addressesannounce)
    - [`Addresses.AppendAnnounce`](#addressesappendannounce)
    - [`Addresses.NoAnnounce`](#addressesnoannounce)
  - [`API`](#api)
    - [`API.HTTPHeaders`](#apihttpheaders)
  - [`AutoNAT`](#autonat)
    - [`AutoNAT.ServiceMode`](#autonatservicemode)
    - [`AutoNAT.Throttle`](#autonatthrottle)
    - [`AutoNAT.Throttle.GlobalLimit`](#autonatthrottlegloballimit)
    - [`AutoNAT.Throttle.PeerLimit`](#autonatthrottlepeerlimit)
    - [`AutoNAT.Throttle.Interval`](#autonatthrottleinterval)
  - [`Bootstrap`](#bootstrap)
  - [`Datastore`](#datastore)
    - [`Datastore.StorageMax`](#datastorestoragemax)
    - [`Datastore.StorageGCWatermark`](#datastorestoragegcwatermark)
    - [`Datastore.GCPeriod`](#datastoregcperiod)
    - [`Datastore.HashOnRead`](#datastorehashonread)
    - [`Datastore.BloomFilterSize`](#datastorebloomfiltersize)
    - [`Datastore.Spec`](#datastorespec)
  - [`Discovery`](#discovery)
    - [`Discovery.MDNS`](#discoverymdns)
      - [`Discovery.MDNS.Enabled`](#discoverymdnsenabled)
      - [`Discovery.MDNS.Interval`](#discoverymdnsinterval)
  - [`Experimental`](#experimental)
  - [`Gateway`](#gateway)
    - [`Gateway.NoFetch`](#gatewaynofetch)
    - [`Gateway.NoDNSLink`](#gatewaynodnslink)
    - [`Gateway.HTTPHeaders`](#gatewayhttpheaders)
    - [`Gateway.RootRedirect`](#gatewayrootredirect)
    - [`Gateway.FastDirIndexThreshold`](#gatewayfastdirindexthreshold)
    - [`Gateway.Writable`](#gatewaywritable)
    - [`Gateway.PathPrefixes`](#gatewaypathprefixes)
    - [`Gateway.PublicGateways`](#gatewaypublicgateways)
      - [`Gateway.PublicGateways: Paths`](#gatewaypublicgateways-paths)
      - [`Gateway.PublicGateways: UseSubdomains`](#gatewaypublicgateways-usesubdomains)
      - [`Gateway.PublicGateways: NoDNSLink`](#gatewaypublicgateways-nodnslink)
      - [Implicit defaults of `Gateway.PublicGateways`](#implicit-defaults-of-gatewaypublicgateways)
    - [`Gateway` recipes](#gateway-recipes)
  - [`Identity`](#identity)
    - [`Identity.PeerID`](#identitypeerid)
    - [`Identity.PrivKey`](#identityprivkey)
  - [`Internal`](#internal)
    - [`Internal.Bitswap`](#internalbitswap)
      - [`Internal.Bitswap.TaskWorkerCount`](#internalbitswaptaskworkercount)
      - [`Internal.Bitswap.EngineBlockstoreWorkerCount`](#internalbitswapengineblockstoreworkercount)
      - [`Internal.Bitswap.EngineTaskWorkerCount`](#internalbitswapenginetaskworkercount)
      - [`Internal.Bitswap.MaxOutstandingBytesPerPeer`](#internalbitswapmaxoutstandingbytesperpeer)
    - [`Internal.UnixFSShardingSizeThreshold`](#internalunixfsshardingsizethreshold)
  - [`Ipns`](#ipns)
    - [`Ipns.RepublishPeriod`](#ipnsrepublishperiod)
    - [`Ipns.RecordLifetime`](#ipnsrecordlifetime)
    - [`Ipns.ResolveCacheSize`](#ipnsresolvecachesize)
    - [`Ipns.UsePubsub`](#ipnsusepubsub)
  - [`Migration`](#migration)
    - [`Migration.DownloadSources`](#migrationdownloadsources)
    - [`Migration.Keep`](#migrationkeep)
  - [`Mounts`](#mounts)
    - [`Mounts.IPFS`](#mountsipfs)
    - [`Mounts.IPNS`](#mountsipns)
    - [`Mounts.FuseAllowOther`](#mountsfuseallowother)
  - [`Pinning`](#pinning)
    - [`Pinning.RemoteServices`](#pinningremoteservices)
      - [`Pinning.RemoteServices: API`](#pinningremoteservices-api)
        - [`Pinning.RemoteServices: API.Endpoint`](#pinningremoteservices-apiendpoint)
        - [`Pinning.RemoteServices: API.Key`](#pinningremoteservices-apikey)
      - [`Pinning.RemoteServices: Policies`](#pinningremoteservices-policies)
        - [`Pinning.RemoteServices: Policies.MFS`](#pinningremoteservices-policiesmfs)
          - [`Pinning.RemoteServices: Policies.MFS.Enabled`](#pinningremoteservices-policiesmfsenabled)
          - [`Pinning.RemoteServices: Policies.MFS.PinName`](#pinningremoteservices-policiesmfspinname)
          - [`Pinning.RemoteServices: Policies.MFS.RepinInterval`](#pinningremoteservices-policiesmfsrepininterval)
  - [`Pubsub`](#pubsub)
    - [`Pubsub.Enabled`](#pubsubenabled)
    - [`Pubsub.Router`](#pubsubrouter)
    - [`Pubsub.DisableSigning`](#pubsubdisablesigning)
  - [`Peering`](#peering)
    - [`Peering.Peers`](#peeringpeers)
  - [`Reprovider`](#reprovider)
    - [`Reprovider.Interval`](#reproviderinterval)
    - [`Reprovider.Strategy`](#reproviderstrategy)
  - [`Routing`](#routing)
    - [`Routing.Routers`](#routingrouters)
      - [`Routing.Routers: Type`](#routingrouters-type)
      - [`Routing.Routers: Enabled`](#routingrouters-enabled)
      - [`Routing.Routers: Parameters`](#routingrouters-parameters)
    - [`Routing.Type`](#routingtype)
  - [`Swarm`](#swarm)
    - [`Swarm.AddrFilters`](#swarmaddrfilters)
    - [`Swarm.DisableBandwidthMetrics`](#swarmdisablebandwidthmetrics)
    - [`Swarm.DisableNatPortMap`](#swarmdisablenatportmap)
    - [`Swarm.EnableHolePunching`](#swarmenableholepunching)
    - [`Swarm.EnableAutoRelay`](#swarmenableautorelay)
    - [`Swarm.RelayClient`](#swarmrelayclient)
      - [`Swarm.RelayClient.Enabled`](#swarmrelayclientenabled)
      - [`Swarm.RelayClient.StaticRelays`](#swarmrelayclientstaticrelays)
    - [`Swarm.RelayService`](#swarmrelayservice)
      - [`Swarm.RelayService.Enabled`](#swarmrelayserviceenabled)
      - [`Swarm.RelayService.Limit`](#swarmrelayservicelimit)
        - [`Swarm.RelayService.ConnectionDurationLimit`](#swarmrelayserviceconnectiondurationlimit)
        - [`Swarm.RelayService.ConnectionDataLimit`](#swarmrelayserviceconnectiondatalimit)
      - [`Swarm.RelayService.ReservationTTL`](#swarmrelayservicereservationttl)
      - [`Swarm.RelayService.MaxReservations`](#swarmrelayservicemaxreservations)
      - [`Swarm.RelayService.MaxCircuits`](#swarmrelayservicemaxcircuits)
      - [`Swarm.RelayService.BufferSize`](#swarmrelayservicebuffersize)
      - [`Swarm.RelayService.MaxReservationsPerPeer`](#swarmrelayservicemaxreservationsperpeer)
      - [`Swarm.RelayService.MaxReservationsPerIP`](#swarmrelayservicemaxreservationsperip)
      - [`Swarm.RelayService.MaxReservationsPerASN`](#swarmrelayservicemaxreservationsperasn)
    - [`Swarm.EnableRelayHop`](#swarmenablerelayhop)
    - [`Swarm.DisableRelay`](#swarmdisablerelay)
    - [`Swarm.EnableAutoNATService`](#swarmenableautonatservice)
    - [`Swarm.ConnMgr`](#swarmconnmgr)
      - [`Swarm.ConnMgr.Type`](#swarmconnmgrtype)
      - [Basic Connection Manager](#basic-connection-manager)
        - [`Swarm.ConnMgr.LowWater`](#swarmconnmgrlowwater)
        - [`Swarm.ConnMgr.HighWater`](#swarmconnmgrhighwater)
        - [`Swarm.ConnMgr.GracePeriod`](#swarmconnmgrgraceperiod)
    - [`Swarm.ResourceMgr`](#swarmresourcemgr)
      - [`Swarm.ResourceMgr.Enabled`](#swarmresourcemgrenabled)
      - [`Swarm.ResourceMgr.Limits`](#swarmresourcemgrlimits)
      - [`Swarm.ResourceMgr.Allowlist`](#swarmresourcemgrallowlist)
    - [`Swarm.Transports`](#swarmtransports)
    - [`Swarm.Transports.Network`](#swarmtransportsnetwork)
      - [`Swarm.Transports.Network.TCP`](#swarmtransportsnetworktcp)
      - [`Swarm.Transports.Network.Websocket`](#swarmtransportsnetworkwebsocket)
      - [`Swarm.Transports.Network.QUIC`](#swarmtransportsnetworkquic)
      - [`Swarm.Transports.Network.Relay`](#swarmtransportsnetworkrelay)
    - [`Swarm.Transports.Security`](#swarmtransportssecurity)
      - [`Swarm.Transports.Security.TLS`](#swarmtransportssecuritytls)
      - [`Swarm.Transports.Security.SECIO`](#swarmtransportssecuritysecio)
      - [`Swarm.Transports.Security.Noise`](#swarmtransportssecuritynoise)
    - [`Swarm.Transports.Multiplexers`](#swarmtransportsmultiplexers)
    - [`Swarm.Transports.Multiplexers.Yamux`](#swarmtransportsmultiplexersyamux)
    - [`Swarm.Transports.Multiplexers.Mplex`](#swarmtransportsmultiplexersmplex)
  - [`DNS`](#dns)
    - [`DNS.Resolvers`](#dnsresolvers)
    - [`DNS.MaxCacheTTL`](#dnsmaxcachettl)

## Profiles

Configuration profiles allow to tweak configuration quickly. Profiles can be
applied with the `--profile` flag to `ipfs init` or with the `ipfs config profile
apply` command. When a profile is applied a backup of the configuration file
will be created in `$IPFS_PATH`.

The available configuration profiles are listed below. You can also find them
documented in `ipfs config profile --help`.

- `server`

  Disables local host discovery, recommended when
  running IPFS on machines with public IPv4 addresses.

- `randomports`

  Use a random port number for the incoming swarm connections.

- `default-datastore`

  Configures the node to use the default datastore (flatfs).

  Read the "flatfs" profile description for more information on this datastore.

  This profile may only be applied when first initializing the node.

- `local-discovery`

  Enables local discovery (enabled by default). Useful to re-enable local discovery after it's
  disabled by another profile (e.g., the server profile).

- `test`

  Reduces external interference of IPFS daemon, this
  is useful when using the daemon in test environments.

- `default-networking`

  Restores default network settings.
  Inverse profile of the test profile.

- `flatfs`

  Configures the node to use the flatfs datastore. Flatfs is the default datastore.

  This is the most battle-tested and reliable datastore. 
  You should use this datastore if:

  - You need a very simple and very reliable datastore, and you trust your
    filesystem. This datastore stores each block as a separate file in the
    underlying filesystem so it's unlikely to lose data unless there's an issue
    with the underlying file system.
  - You need to run garbage collection in a way that reclaims free space as soon as possible.
  - You want to minimize memory usage.
  - You are ok with the default speed of data import, or prefer to use `--nocopy`.

  This profile may only be applied when first initializing the node.


- `badgerds`

  Configures the node to use the experimental badger datastore. Keep in mind that this **uses an outdated badger 1.x**.

  Use this datastore if some aspects of performance, 
  especially the speed of adding many gigabytes of files, are critical. However, be aware that:
  
  - This datastore will not properly reclaim space when your datastore is
    smaller than several gigabytes. If you run IPFS with `--enable-gc`, you plan on storing very little data in
    your IPFS node, and disk usage is more critical than performance, consider using
    `flatfs`.
  - This datastore uses up to several gigabytes of memory.  
  - Good for medium-size datastores, but may run into performance issues if your dataset is bigger than a terabyte.
  - The current implementation is based on old badger 1.x which is no longer supported by the upstream team.

  This profile may only be applied when first initializing the node.

- `lowpower`

  Reduces daemon overhead on the system. May affect node
  functionality - performance of content discovery and data
  fetching may be degraded.

## Types

This document refers to the standard JSON types (e.g., `null`, `string`,
`number`, etc.), as well as a few custom types, described below.

### `flag`

Flags allow enabling and disabling features. However, unlike simple booleans,
they can also be `null` (or omitted) to indicate that the default value should
be chosen. This makes it easier for Kubo to change the defaults in the
future unless the user _explicitly_ sets the flag to either `true` (enabled) or
`false` (disabled). Flags have three possible states:

- `null` or missing (apply the default value).
- `true` (enabled)
- `false` (disabled)

### `priority`

Priorities allow specifying the priority of a feature/protocol and disabling the
feature/protocol. Priorities can take one of the following values:

- `null`/missing (apply the default priority, same as with flags)
- `false` (disabled)
- `1 - 2^63` (priority, lower is preferred)

### `strings`

Strings is a special type for conveniently specifying a single string, an array
of strings, or null:

- `null`
- `"a single string"`
- `["an", "array", "of", "strings"]`

### `duration`

Duration is a type for describing lengths of time, using the same format go
does (e.g, `"1d2h4m40.01s"`).

### `optionalInteger`

Optional integers allow specifying some numerical value which has
an implicit default when missing from the config file:

- `null`/missing will apply the default value defined in Kubo sources (`.WithDefault(value)`)
- an integer between `-2^63` and `2^63-1` (i.e. `-9223372036854775808` to `9223372036854775807`)

### `optionalBytes`

Optional Bytes allow specifying some number of bytes which has
an implicit default when missing from the config file:

- `null`/missing (apply the default value defined in Kubo sources)
- a string value indicating the number of bytes, including human readable representations:
  - [SI sizes](https://en.wikipedia.org/wiki/Metric_prefix#List_of_SI_prefixes) (metric units, powers of 1000), e.g. `1B`, `2kB`, `3MB`, `4GB`, `5TB`, …)
  - [IEC sizes](https://en.wikipedia.org/wiki/Binary_prefix#IEC_prefixes) (binary units, powers of 1024), e.g. `1B`, `2KiB`, `3MiB`, `4GiB`, `5TiB`, …)

### `optionalString`

Optional strings allow specifying some string value which has
an implicit default when missing from the config file:

- `null`/missing will apply the default value defined in Kubo sources (`.WithDefault("value")`)
- a string

### `optionalDuration`

Optional durations allow specifying some duration value which has
an implicit default when missing from the config file:

- `null`/missing will apply the default value defined in Kubo sources (`.WithDefault("1h2m3s")`)
- a string with a valid [go duration](#duration)  (e.g, `"1d2h4m40.01s"`).

## `Addresses`

Contains information about various listener addresses to be used by this node.

### `Addresses.API`

Multiaddr or array of multiaddrs describing the address to serve the local HTTP
API on.

Supported Transports:

* tcp/ip{4,6} - `/ipN/.../tcp/...`
* unix - `/unix/path/to/socket`

Default: `/ip4/127.0.0.1/tcp/5001`

Type: `strings` (multiaddrs)

### `Addresses.Gateway`

Multiaddr or array of multiaddrs describing the address to serve the local
gateway on.

Supported Transports:

* tcp/ip{4,6} - `/ipN/.../tcp/...`
* unix - `/unix/path/to/socket`

Default: `/ip4/127.0.0.1/tcp/8080`

Type: `strings` (multiaddrs)

### `Addresses.Swarm`

An array of multiaddrs describing which addresses to listen on for p2p swarm
connections.

Supported Transports:

* tcp/ip{4,6} - `/ipN/.../tcp/...`
* websocket - `/ipN/.../tcp/.../ws`
* quic - `/ipN/.../udp/.../quic`

Default:
```json
[
  "/ip4/0.0.0.0/tcp/4001",
  "/ip6/::/tcp/4001",
  "/ip4/0.0.0.0/udp/4001/quic",
  "/ip6/::/udp/4001/quic"
]
```

Type: `array[string]` (multiaddrs)

### `Addresses.Announce`

If non-empty, this array specifies the swarm addresses to announce to the
network. If empty, the daemon will announce inferred swarm addresses.

Default: `[]`

Type: `array[string]` (multiaddrs)

### `Addresses.AppendAnnounce`

Similar to [`Addresses.Announce`](#addressesannounce) except this doesn't
override inferred swarm addresses if non-empty.

Default: `[]`

Type: `array[string]` (multiaddrs)

### `Addresses.NoAnnounce`

An array of swarm addresses not to announce to the network.
Takes precedence over `Addresses.Announce` and `Addresses.AppendAnnounce`.

Default: `[]`

Type: `array[string]` (multiaddrs)

## `API`
Contains information used by the API gateway.

### `API.HTTPHeaders`
Map of HTTP headers to set on responses from the API HTTP server.

Example:
```json
{
	"Foo": ["bar"]
}
```

Default: `null`

Type: `object[string -> array[string]]` (header names -> array of header values)

## `AutoNAT`

Contains the configuration options for the AutoNAT service. The AutoNAT service
helps other nodes on the network determine if they're publicly reachable from
the rest of the internet.

### `AutoNAT.ServiceMode`

When unset (default), the AutoNAT service defaults to _enabled_. Otherwise, this
field can take one of two values:

* "enabled" - Enable the service (unless the node determines that it, itself,
  isn't reachable by the public internet).
* "disabled" - Disable the service.

Additional modes may be added in the future.

Type: `string` (one of `"enabled"` or `"disabled"`)

### `AutoNAT.Throttle`

When set, this option configure's the AutoNAT services throttling behavior. By
default, Kubo will rate-limit the number of NAT checks performed for other
nodes to 30 per minute, and 3 per peer.

### `AutoNAT.Throttle.GlobalLimit`

Configures how many AutoNAT requests to service per `AutoNAT.Throttle.Interval`.

Default: 30

Type: `integer` (non-negative, `0` means unlimited)

### `AutoNAT.Throttle.PeerLimit`

Configures how many AutoNAT requests per-peer to service per `AutoNAT.Throttle.Interval`.

Default: 3

Type: `integer` (non-negative, `0` means unlimited)

### `AutoNAT.Throttle.Interval`

Configures the interval for the above limits.

Default: 1 Minute

Type: `duration` (when `0`/unset, the default value is used)

## `Bootstrap`

Bootstrap is an array of multiaddrs of trusted nodes that your node connects to, to fetch other nodes of the network on startup.

Default: The ipfs.io bootstrap nodes

Type: `array[string]` (multiaddrs)

## `Datastore`

Contains information related to the construction and operation of the on-disk
storage system.

### `Datastore.StorageMax`

A soft upper limit for the size of the ipfs repository's datastore. With `StorageGCWatermark`,
is used to calculate whether to trigger a gc run (only if `--enable-gc` flag is set).

Default: `"10GB"`

Type: `string` (size)

### `Datastore.StorageGCWatermark`

The percentage of the `StorageMax` value at which a garbage collection will be
triggered automatically if the daemon was run with automatic gc enabled (that
option defaults to false currently).

Default: `90`

Type: `integer` (0-100%)

### `Datastore.GCPeriod`

A time duration specifying how frequently to run a garbage collection. Only used
if automatic gc is enabled.

Default: `1h`

Type: `duration` (an empty string means the default value)

### `Datastore.HashOnRead`

A boolean value. If set to true, all block reads from the disk will be hashed and
verified. This will cause increased CPU utilization.

Default: `false`

Type: `bool`

### `Datastore.BloomFilterSize`

A number representing the size in bytes of the blockstore's [bloom
filter](https://en.wikipedia.org/wiki/Bloom_filter). A value of zero represents
the feature is disabled.

This site generates useful graphs for various bloom filter values:
<https://hur.st/bloomfilter/?n=1e6&p=0.01&m=&k=7> You may use it to find a
preferred optimal value, where `m` is `BloomFilterSize` in bits. Remember to
convert the value `m` from bits, into bytes for use as `BloomFilterSize` in the
config file. For example, for 1,000,000 blocks, expecting a 1% false-positive
rate, you'd end up with a filter size of 9592955 bits, so for `BloomFilterSize`
we'd want to use 1199120 bytes. As of writing, [7 hash
functions](https://github.com/ipfs/go-ipfs-blockstore/blob/547442836ade055cc114b562a3cc193d4e57c884/caching.go#L22)
are used, so the constant `k` is 7 in the formula.

Default: `0` (disabled)

Type: `integer` (non-negative, bytes)

### `Datastore.Spec`

Spec defines the structure of the ipfs datastore. It is a composable structure,
where each datastore is represented by a json object. Datastores can wrap other
datastores to provide extra functionality (eg metrics, logging, or caching).

This can be changed manually, however, if you make any changes that require a
different on-disk structure, you will need to run the [ipfs-ds-convert
tool](https://github.com/ipfs/ipfs-ds-convert) to migrate data into the new
structures.

For more information on possible values for this configuration option, see
[docs/datastores.md](datastores.md)

Default:
```
{
  "mounts": [
	{
	  "child": {
		"path": "blocks",
		"shardFunc": "/repo/flatfs/shard/v1/next-to-last/2",
		"sync": true,
		"type": "flatfs"
	  },
	  "mountpoint": "/blocks",
	  "prefix": "flatfs.datastore",
	  "type": "measure"
	},
	{
	  "child": {
		"compression": "none",
		"path": "datastore",
		"type": "levelds"
	  },
	  "mountpoint": "/",
	  "prefix": "leveldb.datastore",
	  "type": "measure"
	}
  ],
  "type": "mount"
}
```

Type: `object`

## `Discovery`

Contains options for configuring IPFS node discovery mechanisms.

### `Discovery.MDNS`

Options for [ZeroConf](https://github.com/libp2p/zeroconf#readme) Multicast DNS-SD peer discovery.

#### `Discovery.MDNS.Enabled`

A boolean value for whether or not Multicast DNS-SD should be active.

Default: `true`

Type: `bool`

#### `Discovery.MDNS.Interval`

**REMOVED:**  this is not configurable any more
in the [new mDNS implementation](https://github.com/libp2p/zeroconf#readme).

## `Experimental`

Toggle and configure experimental features of Kubo. Experimental features are listed [here](./experimental-features.md).

## `Gateway`

Options for the HTTP gateway.

### `Gateway.NoFetch`

When set to true, the gateway will only serve content already in the local repo
and will not fetch files from the network.

Default: `false`

Type: `bool`

### `Gateway.NoDNSLink`

A boolean to configure whether DNSLink lookup for value in `Host` HTTP header
should be performed.  If DNSLink is present, the content path stored in the DNS TXT
record becomes the `/` and the respective payload is returned to the client.

Default: `false`

Type: `bool`

### `Gateway.HTTPHeaders`

Headers to set on gateway responses.

Default:
```json
{
	"Access-Control-Allow-Headers": [
		"X-Requested-With"
	],
	"Access-Control-Allow-Methods": [
		"GET"
	],
	"Access-Control-Allow-Origin": [
		"*"
	]
}
```

Type: `object[string -> array[string]]`

### `Gateway.RootRedirect`

A url to redirect requests for `/` to.

Default: `""`

Type: `string` (url)

### `Gateway.FastDirIndexThreshold`

The maximum number of items in a directory before the Gateway switches
to a shallow, faster listing which only requires the root node.

This allows for fast listings of big directories, without the linear slowdown caused
by reading size metadata from child nodes.

Setting to 0 will enable fast listings for all directories.

Default: `100`

Type: `optionalInteger`

### `Gateway.Writable`

A boolean to configure whether the gateway is writeable or not.

Default: `false`

Type: `bool`

### `Gateway.PathPrefixes`

**REMOVED:** see [go-ipfs#7702](https://github.com/ipfs/go-ipfs/issues/7702)

### `Gateway.PublicGateways`

`PublicGateways` is a dictionary for defining gateway behavior on specified hostnames.

Hostnames can optionally be defined with one or more wildcards.

Examples:
- `*.example.com` will match requests to `http://foo.example.com/ipfs/*` or `http://{cid}.ipfs.bar.example.com/*`.
- `foo-*.example.com` will match requests to `http://foo-bar.example.com/ipfs/*` or `http://{cid}.ipfs.foo-xyz.example.com/*`.

#### `Gateway.PublicGateways: Paths`

An array of paths that should be exposed on the hostname.

Example:
```json
{
  "Gateway": {
    "PublicGateways": {
      "example.com": {
        "Paths": ["/ipfs", "/ipns"],
      }
    }
  }
}
```

Above enables `http://example.com/ipfs/*` and `http://example.com/ipns/*` but not `http://example.com/api/*`

Default: `[]`

Type: `array[string]`

#### `Gateway.PublicGateways: UseSubdomains`

A boolean to configure whether the gateway at the hostname provides [Origin isolation](https://developer.mozilla.org/en-US/docs/Web/Security/Same-origin_policy)
between content roots.

- `true` - enables [subdomain gateway](https://docs.ipfs.tech/how-to/address-ipfs-on-web/#subdomain-gateway) at `http://*.{hostname}/`
    - **Requires whitelist:** make sure respective `Paths` are set.
      For example, `Paths: ["/ipfs", "/ipns"]` are required for `http://{cid}.ipfs.{hostname}` and `http://{foo}.ipns.{hostname}` to work:
        ```json
        "Gateway": {
            "PublicGateways": {
                "dweb.link": {
                    "UseSubdomains": true,
                    "Paths": ["/ipfs", "/ipns"],
                }
            }
        }
        ```
    - **Backward-compatible:** requests for content paths such as `http://{hostname}/ipfs/{cid}` produce redirect to `http://{cid}.ipfs.{hostname}`
    - **API:** if `/api` is on the `Paths` whitelist, `http://{hostname}/api/{cmd}` produces redirect to `http://api.{hostname}/api/{cmd}`

- `false` - enables [path gateway](https://docs.ipfs.tech/how-to/address-ipfs-on-web/#path-gateway) at `http://{hostname}/*`
  - Example:
    ```json
    "Gateway": {
        "PublicGateways": {
            "ipfs.io": {
                "UseSubdomains": false,
                "Paths": ["/ipfs", "/ipns", "/api"],
            }
        }
    }
    ```

Default: `false`

Type: `bool`

#### `Gateway.PublicGateways: NoDNSLink`

A boolean to configure whether DNSLink for hostname present in `Host`
HTTP header should be resolved. Overrides global setting.
If `Paths` are defined, they take priority over DNSLink.

Default: `false` (DNSLink lookup enabled by default for every defined hostname)

Type: `bool`

#### Implicit defaults of `Gateway.PublicGateways`

Default entries for `localhost` hostname and loopback IPs are always present.
If additional config is provided for those hostnames, it will be merged on top of implicit values:
```json
{
  "Gateway": {
    "PublicGateways": {
      "localhost": {
        "Paths": ["/ipfs", "/ipns"],
        "UseSubdomains": true
      }
    }
  }
}
```

It is also possible to remove a default by setting it to `null`.

For example, to disable subdomain gateway on `localhost`
and make that hostname act the same as `127.0.0.1`:

```console
$ ipfs config --json Gateway.PublicGateways '{"localhost": null }'
```

### `Gateway` recipes

Below is a list of the most common public gateway setups.

* Public [subdomain gateway](https://docs.ipfs.tech/how-to/address-ipfs-on-web/#subdomain-gateway) at `http://{cid}.ipfs.dweb.link` (each content root gets its own Origin)
   ```console
   $ ipfs config --json Gateway.PublicGateways '{
       "dweb.link": {
         "UseSubdomains": true,
         "Paths": ["/ipfs", "/ipns"]
       }
     }'
   ```
   - **Backward-compatible:** this feature enables automatic redirects from content paths to subdomains:
   
     `http://dweb.link/ipfs/{cid}` → `http://{cid}.ipfs.dweb.link`
     
   - **X-Forwarded-Proto:** if you run Kubo behind a reverse proxy that provides TLS, make it add a `X-Forwarded-Proto: https` HTTP header to ensure users are redirected to `https://`, not `http://`. It will also ensure DNSLink names are inlined to fit in a single DNS label, so they work fine with a wildcart TLS cert ([details](https://github.com/ipfs/in-web-browsers/issues/169)). The NGINX directive is `proxy_set_header X-Forwarded-Proto "https";`.:
    
     `http://dweb.link/ipfs/{cid}` → `https://{cid}.ipfs.dweb.link`
     
     `http://dweb.link/ipns/your-dnslink.site.example.com` → `https://your--dnslink-site-example-com.ipfs.dweb.link`
     
   - **X-Forwarded-Host:** we also support `X-Forwarded-Host: example.com` if you want to override subdomain gateway host from the original request:
   
     `http://dweb.link/ipfs/{cid}` → `http://{cid}.ipfs.example.com`


* Public [path gateway](https://docs.ipfs.tech/how-to/address-ipfs-on-web/#path-gateway) at `http://ipfs.io/ipfs/{cid}` (no Origin separation)
   ```console
   $ ipfs config --json Gateway.PublicGateways '{
       "ipfs.io": {
         "UseSubdomains": false,
         "Paths": ["/ipfs", "/ipns", "/api"]
       }
     }'
   ```

* Public [DNSLink](https://dnslink.io/) gateway resolving every hostname passed in `Host` header.
  ```console
  $ ipfs config --json Gateway.NoDNSLink false
  ```
  * Note that `NoDNSLink: false` is the default (it works out of the box unless set to `true` manually)

* Hardened, site-specific [DNSLink gateway](https://docs.ipfs.tech/how-to/address-ipfs-on-web/#dnslink-gateway).

  Disable fetching of remote data (`NoFetch: true`) and resolving DNSLink at unknown hostnames (`NoDNSLink: true`).
  Then, enable DNSLink gateway only for the specific hostname (for which data
  is already present on the node), without exposing any content-addressing `Paths`:
  
   ```console
   $ ipfs config --json Gateway.NoFetch true
   $ ipfs config --json Gateway.NoDNSLink true
   $ ipfs config --json Gateway.PublicGateways '{
       "en.wikipedia-on-ipfs.org": {
         "NoDNSLink": false,
         "Paths": []
       }
     }'
   ```

## `Identity`

### `Identity.PeerID`

The unique PKI identity label for this configs peer. Set on init and never read,
it's merely here for convenience. Ipfs will always generate the peerID from its
keypair at runtime.

Type: `string` (peer ID)

### `Identity.PrivKey`

The base64 encoded protobuf describing (and containing) the node's private key.

Type: `string` (base64 encoded)

## `Internal`

This section includes internal knobs for various subsystems to allow advanced users with big or private infrastructures to fine-tune some behaviors without the need to recompile Kubo.  

**Be aware that making informed change here requires in-depth knowledge and most users should leave these untouched. All knobs listed here are subject to breaking changes between versions.** 

### `Internal.Bitswap`

`Internal.Bitswap` contains knobs for tuning bitswap resource utilization.
The knobs (below) document how their value should related to each other.
Whether their values should be raised or lowered should be determined
based on the metrics `ipfs_bitswap_active_tasks`, `ipfs_bitswap_pending_tasks`,
`ipfs_bitswap_pending_block_tasks` and `ipfs_bitswap_active_block_tasks`
reported by bitswap.

These metrics can be accessed as the prometheus endpoint at `{Addresses.API}/debug/metrics/prometheus` (default: `http://127.0.0.1:5001/debug/metrics/prometheus`)

The value of `ipfs_bitswap_active_tasks` is capped by `EngineTaskWorkerCount`.

The value of `ipfs_bitswap_pending_tasks` is generally capped by the knobs below,
however its exact maximum value is hard to predict as it depends on task sizes
as well as number of requesting peers. However, as a rule of thumb,
during healthy operation this value should oscillate around a "typical" low value
(without hitting a plateau continuously).

If `ipfs_bitswap_pending_tasks` is growing while `ipfs_bitswap_active_tasks` is at its maximum then
the node has reached its resource limits and new requests are unable to be processed as quickly as they are coming in.
Raising resource limits (using the knobs below) could help, assuming the hardware can support the new limits.

The value of `ipfs_bitswap_active_block_tasks` is capped by `EngineBlockstoreWorkerCount`.

The value of `ipfs_bitswap_pending_block_tasks` is indirectly capped by `ipfs_bitswap_active_tasks`, but can be hard to
predict as it depends on the number of blocks involved in a peer task which can vary.

If the value of `ipfs_bitswap_pending_block_tasks` is observed to grow,
while `ipfs_bitswap_active_block_tasks` is at its maximum, there is indication that the number of
available block tasks is creating a bottleneck (either due to high-latency block operations,
or due to high number of block operations per bitswap peer task).
In such cases, try increasing the `EngineBlockstoreWorkerCount`.
If this adjustment still does not increase the throuput of the node, there might
be hardware limitations like I/O or CPU.

#### `Internal.Bitswap.TaskWorkerCount`

Number of threads (goroutines) sending outgoing messages.
Throttles the number of concurrent send operations.

Type: `optionalInteger` (thread count, `null` means default which is 8)

#### `Internal.Bitswap.EngineBlockstoreWorkerCount`

Number of threads for blockstore operations.
Used to throttle the number of concurrent requests to the block store.
The optimal value can be informed by the metrics `ipfs_bitswap_pending_block_tasks` and `ipfs_bitswap_active_block_tasks`.
This would be a number that depends on your hardware (I/O and CPU).

Type: `optionalInteger` (thread count, `null` means default which is 128)

#### `Internal.Bitswap.EngineTaskWorkerCount`

Number of worker threads used for preparing and packaging responses before they are sent out.
This number should generally be equal to `TaskWorkerCount`.

Type: `optionalInteger` (thread count, `null` means default which is 8)

#### `Internal.Bitswap.MaxOutstandingBytesPerPeer`

Maximum number of bytes (across all tasks) pending to be processed and sent to any individual peer.
This number controls fairness and can very from 250Kb (very fair) to 10Mb (less fair, with more work
dedicated to peers who ask for more). Values below 250Kb could cause thrashing.
Values above 10Mb open the potential for aggressively-wanting peers to consume all resources and
deteriorate the quality provided to less aggressively-wanting peers.

Type: `optionalInteger` (byte count, `null` means default which is 1MB)

### `Internal.UnixFSShardingSizeThreshold`

The sharding threshold used internally to decide whether a UnixFS directory should be sharded or not.
This value is not strictly related to the size of the UnixFS directory block and any increases in
the threshold should come with being careful that block sizes stay under 2MiB in order for them to be
reliably transferable through the networking stack (IPFS peers on the public swarm tend to ignore requests for blocks bigger than 2MiB).

Decreasing this value to 1B is functionally equivalent to the previous experimental sharding option to
shard all directories.

Type: `optionalBytes` (`null` means default which is 256KiB)

## `Ipns`

### `Ipns.RepublishPeriod`

A time duration specifying how frequently to republish ipns records to ensure
they stay fresh on the network.

Default: 4 hours.

Type: `interval` or an empty string for the default.

### `Ipns.RecordLifetime`

A time duration specifying the value to set on ipns records for their validity
lifetime.

Default: 24 hours.

Type: `interval` or an empty string for the default.

### `Ipns.ResolveCacheSize`

The number of entries to store in an LRU cache of resolved ipns entries. Entries
will be kept cached until their lifetime is expired.

Default: `128`

Type: `integer` (non-negative, 0 means the default)

### `Ipns.UsePubsub`

Enables IPFS over pubsub experiment for publishing IPNS records in real time.

**EXPERIMENTAL:**  read about current limitations at [experimental-features.md#ipns-pubsub](./experimental-features.md#ipns-pubsub).

Default: `disabled`

Type: `flag`

## `Migration`

Migration configures how migrations are downloaded and if the downloads are added to IPFS locally.

### `Migration.DownloadSources`

Sources in order of preference, where "IPFS" means use IPFS and "HTTPS" means use default gateways. Any other values are interpreted as hostnames for custom gateways. An empty list means "use default sources".

Default: `["HTTPS", "IPFS"]`

### `Migration.Keep`

Specifies whether or not to keep the migration after downloading it. Options are "discard", "cache", "pin". Empty string for default.

Default: `cache`

## `Mounts`

**EXPERIMENTAL:** read about current limitations at [fuse.md](./fuse.md).

FUSE mount point configuration options.

### `Mounts.IPFS`

Mountpoint for `/ipfs/`.

Default: `/ipfs`

Type: `string` (filesystem path)

### `Mounts.IPNS`

Mountpoint for `/ipns/`.

Default: `/ipns`

Type: `string` (filesystem path)

### `Mounts.FuseAllowOther`

Sets the 'FUSE allow other'-option on the mount point.

## `Pinning`

Pinning configures the options available for pinning content
(i.e. keeping content longer-term instead of as temporarily cached storage).

### `Pinning.RemoteServices`

`RemoteServices` maps a name for a remote pinning service to its configuration.

A remote pinning service is a remote service that exposes an API for managing
that service's interest in long-term data storage.

The exposed API conforms to the specification defined at
https://ipfs.github.io/pinning-services-api-spec/

#### `Pinning.RemoteServices: API`

Contains information relevant to utilizing the remote pinning service

Example:
```json
{
  "Pinning": {
    "RemoteServices": {
      "myPinningService": {
        "API" : {
          "Endpoint" : "https://pinningservice.tld:1234/my/api/path",
          "Key" : "someOpaqueKey"
				}
      }
    }
  }
}
```

##### `Pinning.RemoteServices: API.Endpoint`

The HTTP(S) endpoint through which to access the pinning service

Example: "https://pinningservice.tld:1234/my/api/path"

Type: `string`

##### `Pinning.RemoteServices: API.Key`

The key through which access to the pinning service is granted

Type: `string`

#### `Pinning.RemoteServices: Policies`

Contains additional opt-in policies for the remote pinning service.

##### `Pinning.RemoteServices: Policies.MFS`

When this policy is enabled, it follows changes to MFS
and updates the pin for MFS root on the configured remote service.

A pin request to the remote service is sent only when MFS root CID has changed
and enough time has passed since the previous request (determined by `RepinInterval`).

One can observe MFS pinning details by enabling debug via `ipfs log level remotepinning/mfs debug` and switching back to `error` when done.

###### `Pinning.RemoteServices: Policies.MFS.Enabled`

Controls if this policy is active.

Default: `false`

Type: `bool`

###### `Pinning.RemoteServices: Policies.MFS.PinName`

Optional name to use for a remote pin that represents the MFS root CID.
When left empty, a default name will be generated.

Default: `"policy/{PeerID}/mfs"`, e.g. `"policy/12.../mfs"`

Type: `string`

###### `Pinning.RemoteServices: Policies.MFS.RepinInterval`

Defines how often (at most) the pin request should be sent to the remote service.
If left empty, the default interval will be used. Values lower than `1m` will be ignored.

Default: `"5m"`

Type: `duration`

## `Pubsub`

Pubsub configures the `ipfs pubsub` subsystem. To use, it must be enabled by
passing the `--enable-pubsub-experiment` flag to the daemon
or via the `Pubsub.Enabled` flag below.

### `Pubsub.Enabled`

**EXPERIMENTAL:** read about current limitations at [experimental-features.md#ipfs-pubsub](./experimental-features.md#ipfs-pubsub).

Enables the pubsub system.

Default: `false`

Type: `flag`

### `Pubsub.Router`

Sets the default router used by pubsub to route messages to peers. This can be one of:

* `"floodsub"` - floodsub is a basic router that simply _floods_ messages to all
  connected peers. This router is extremely inefficient but _very_ reliable.
* `"gossipsub"` - [gossipsub][] is a more advanced routing algorithm that will
  build an overlay mesh from a subset of the links in the network.

Default: `"gossipsub"`

Type: `string` (one of `"floodsub"`, `"gossipsub"`, or `""` (apply default))

[gossipsub]: https://github.com/libp2p/specs/tree/master/pubsub/gossipsub

### `Pubsub.DisableSigning`

Disables message signing and signature verification. Enable this option if
you're operating in a completely trusted network.

It is _not_ safe to disable signing even if you don't care _who_ sent the
message because spoofed messages can be used to silence real messages by
intentionally re-using the real message's message ID.

Default: `false`

Type: `bool`

## `Peering`

Configures the peering subsystem. The peering subsystem configures Kubo to
connect to, remain connected to, and reconnect to a set of nodes. Nodes should
use this subsystem to create "sticky" links between frequently useful peers to
improve reliability.

Use-cases:

* An IPFS gateway connected to an IPFS cluster should peer to ensure that the
  gateway can always fetch content from the cluster.
* A dapp may peer embedded Kubo nodes with a set of pinning services or
  textile cafes/hubs.
* A set of friends may peer to ensure that they can always fetch each other's
  content.

When a node is added to the set of peered nodes, Kubo will:

1. Protect connections to this node from the connection manager. That is,
   Kubo will never automatically close the connection to this node and
   connections to this node will not count towards the connection limit.
2. Connect to this node on startup.
3. Repeatedly try to reconnect to this node if the last connection dies or the
   node goes offline. This repeated re-connect logic is governed by a randomized
   exponential backoff delay ranging from ~5 seconds to ~10 minutes to avoid
   repeatedly reconnect to a node that's offline.

Peering can be asymmetric or symmetric:

* When symmetric, the connection will be protected by both nodes and will likely
  be very stable.
* When asymmetric, only one node (the node that configured peering) will protect
  the connection and attempt to re-connect to the peered node on disconnect. If
  the peered node is under heavy load and/or has a low connection limit, the
  connection may flap repeatedly. Be careful when asymmetrically peering to not
  overload peers.

### `Peering.Peers`

The set of peers with which to peer.

```json
{
  "Peering": {
    "Peers": [
      {
        "ID": "QmPeerID1",
        "Addrs": ["/ip4/18.1.1.1/tcp/4001"]
      },
      {
        "ID": "QmPeerID2",
        "Addrs": ["/ip4/18.1.1.2/tcp/4001", "/ip4/18.1.1.2/udp/4001/quic"]
      }
    ]
  }
  ...
}
```

Where `ID` is the peer ID and `Addrs` is a set of known addresses for the peer. If no addresses are specified, the DHT will be queried.

Additional fields may be added in the future.

Default: empty.

Type: `array[peering]`

## `Reprovider`

### `Reprovider.Interval`

Sets the time between rounds of reproviding local content to the routing
system. If unset, it defaults to 12 hours. If set to the value `"0"` it will
disable content reproviding.

Note: disabling content reproviding will result in other nodes on the network
not being able to discover that you have the objects that you have. If you want
to have this disabled and keep the network aware of what you have, you must
manually announce your content periodically.

Type: `duration`

### `Reprovider.Strategy`

Tells reprovider what should be announced. Valid strategies are:

- `"all"` - announce all CIDs of stored blocks
- `"pinned"` - only announce pinned CIDs recursively (both roots and child blocks)
- `"roots"` - only announce the root block of explicitly pinned CIDs

Default: `"all"`

Type: `string` (or unset for the default, which is "all")

## `Routing`

Contains options for content, peer, and IPNS routing mechanisms.

### `Routing.Routers`

**EXPERIMENTAL: `Routing.Routers` configuration may change in future release**

Map of additional Routers.

Allows for extending the default routing (DHT) with alternative Router
implementations, such as custom DHTs and delegated routing based
on the [reframe protocol](https://github.com/ipfs/specs/tree/main/reframe#readme).

The map key is a name of a Router, and the value is its configuration.

Default: `{}`

Type: `object[string->object]`

#### `Routing.Routers: Type`

**EXPERIMENTAL: `Routing.Routers` configuration may change in future release**

It specifies the routing type that will be created.

Currently supported types:

- `reframe` (delegated routing based on the [reframe protocol](https://github.com/ipfs/specs/tree/main/reframe#readme))
- <del>`dht`</del> (WIP, custom DHT will be added in a future release)

Type: `string`

#### `Routing.Routers: Enabled`

**EXPERIMENTAL: `Routing.Routers` configuration may change in future release**

Optional flag to disable the specified router without removing it from the configuration file.

Default: `true`

Type: `flag` (`null`/missing will apply the default)

#### `Routing.Routers: Parameters`

**EXPERIMENTAL: `Routing.Routers` configuration may change in future release**

Parameters needed to create the specified router. Supported params per router type:

Reframe:
  - `Endpoint` (mandatory): URL that will be used to connect to a specified router.
  - `Priority` (optional): Priority is used when making a routing request. Small numbers represent more important routers. The default priority is 100000.

**Examples:**

To add router provided by _Store the Index_ team at [cid.contact](https://cid.contact):

```console
$ ipfs config Routing.Routers.CidContact --json '{
  "Type": "reframe",
  "Parameters": {
    "Endpoint": "https://cid.contact/reframe"
  }
}'
```

Anyone can create and run their own Reframe endpoint, and experiment with custom routing logic. See [`someguy`](https://github.com/aschmahmann/someguy) example, which proxies requests to BOTH the IPFS Public DHT AND an Indexer node. Protocol Labs provides a public instance at `https://routing.delegate.ipfs.io/reframe`.

Default: `{}` (use the safe implicit defaults)

Type: `object[string->string]`

### `Routing.Type`

There are two core routing options: "none" and "dht" (default).

* If set to "none", your node will use _no_ routing system. You'll have to
  explicitly connect to peers that have the content you're looking for.
* If set to "dht" (or "dhtclient"/"dhtserver"), your node will use the IPFS DHT.

When the DHT is enabled, it can operate in two modes: client and server.

* In server mode, your node will query other peers for DHT records, and will
  respond to requests from other peers (both requests to store records and
  requests to retrieve records).
* In client mode, your node will query the DHT as a client but will not respond
  to requests from other peers. This mode is less resource-intensive than server
  mode.

When `Routing.Type` is set to `dht`, your node will start as a DHT client, and
switch to a DHT server when and if it determines that it's reachable from the
public internet (e.g., it's not behind a firewall).

To force a specific DHT mode, client or server, set `Routing.Type` to
`dhtclient` or `dhtserver` respectively. Please do not set this to `dhtserver`
unless you're sure your node is reachable from the public network.

**Example:**

```json
{
  "Routing": {
    "Type": "dhtclient"
  }
}
```

Default: `dht`

Type: `optionalString` (`null`/missing means the default)

## `Swarm`

Options for configuring the swarm.

### `Swarm.AddrFilters`

An array of addresses (multiaddr netmasks) to not dial. By default, IPFS nodes
advertise _all_ addresses, even internal ones. This makes it easier for nodes on
the same network to reach each other. Unfortunately, this means that an IPFS
node will try to connect to one or more private IP addresses whenever dialing
another node, even if this other node is on a different network. This may
trigger netscan alerts on some hosting providers or cause strain in some setups.

The `server` configuration profile fills up this list with sensible defaults,
preventing dials to all non-routable IP addresses (e.g., `192.168.0.0/16`) but
you should always check settings against your own network and/or hosting
provider.

Default: `[]`

Type: `array[string]`

### `Swarm.DisableBandwidthMetrics`

A boolean value that when set to true, will cause ipfs to not keep track of
bandwidth metrics. Disabling bandwidth metrics can lead to a slight performance
improvement, as well as a reduction in memory usage.

Default: `false`

Type: `bool`

### `Swarm.DisableNatPortMap`

Disable automatic NAT port forwarding.

When not disabled (default), Kubo asks NAT devices (e.g., routers), to open
up an external port and forward it to the port Kubo is running on. When this
works (i.e., when your router supports NAT port forwarding), it makes the local
Kubo node accessible from the public internet.

Default: `false`

Type: `bool`

### `Swarm.EnableHolePunching`

Enable hole punching for NAT traversal
when port forwarding is not possible.

When enabled, Kubo will coordinate with the counterparty using
a [relayed connection](https://github.com/libp2p/specs/blob/master/relay/circuit-v2.md),
to [upgrade to a direct connection](https://github.com/libp2p/specs/blob/master/relay/DCUtR.md)
through a NAT/firewall whenever possible.
This feature requires `Swarm.RelayClient.Enabled` to be set to `true`.

Default: `true`

Type: `flag`

### `Swarm.EnableAutoRelay`

**REMOVED**

See `Swarm.RelayClient` instead.

### `Swarm.RelayClient`

Configuration options for the relay client to use relay services.

Default: `{}`

Type: `object`

#### `Swarm.RelayClient.Enabled`

Enables "automatic relay user" mode for this node.

Your node will automatically _use_ public relays from the network if it detects
that it cannot be reached from the public internet (e.g., it's behind a
firewall) and get a `/p2p-circuit` address from a public relay.

Default: `true`

Type: `flag`

#### `Swarm.RelayClient.StaticRelays`

Your node will use these statically configured relay servers (V1 or V2)
instead of discovering public relays V2 from the network.

Default: `[]`

Type: `array[string]`

### `Swarm.RelayService`

Configuration options for the relay service that can be provided to _other_ peers
on the network ([Circuit Relay v2](https://github.com/libp2p/specs/blob/master/relay/circuit-v2.md)).

Default: `{}`

Type: `object`

#### `Swarm.RelayService.Enabled`

Enables providing `/p2p-circuit` v2 relay service to other peers on the network.

NOTE: This is the service/server part of the relay system.
Disabling this will prevent this node from running as a relay server.
Use [`Swarm.RelayClient.Enabled`](#swarmrelayclientenabled) for turning your node into a relay user.

Default: `true`

Type: `flag`

#### `Swarm.RelayService.Limit`

Limits applied to every relayed connection.

Default: `{}`

Type: `object[string -> string]`

##### `Swarm.RelayService.ConnectionDurationLimit`

Time limit before a relayed connection is reset.

Default: `"2m"`

Type: `duration`

##### `Swarm.RelayService.ConnectionDataLimit`

Limit of data relayed (in each direction) before a relayed connection is reset.

Default: `131072` (128 kb)

Type: `optionalInteger`


#### `Swarm.RelayService.ReservationTTL`

Duration of a new or refreshed reservation. 

Default: `"1h"`

Type: `duration`


#### `Swarm.RelayService.MaxReservations`

Maximum number of active relay slots.

Default: `128`

Type: `optionalInteger`


#### `Swarm.RelayService.MaxCircuits`

Maximum number of open relay connections for each peer.

Default: `16`

Type: `optionalInteger`


#### `Swarm.RelayService.BufferSize`

Size of the relayed connection buffers.

Default: `2048`

Type: `optionalInteger`


#### `Swarm.RelayService.MaxReservationsPerPeer`

Maximum number of reservations originating from the same peer.

Default: `4`

Type: `optionalInteger`


#### `Swarm.RelayService.MaxReservationsPerIP`

Maximum number of reservations originating from the same IP.

Default: `8`

Type: `optionalInteger`

#### `Swarm.RelayService.MaxReservationsPerASN`

Maximum number of reservations originating from the same ASN.

Default: `32`

Type: `optionalInteger`

### `Swarm.EnableRelayHop`

**REMOVED**

Replaced with [`Swarm.RelayService.Enabled`](#swarmrelayserviceenabled).

### `Swarm.DisableRelay`

**REMOVED**

Set `Swarm.Transports.Network.Relay` to `false` instead.

### `Swarm.EnableAutoNATService`

**REMOVED**

Please use [`AutoNAT.ServiceMode`](#autonatservicemode).

### `Swarm.ConnMgr`

The connection manager determines which and how many connections to keep and can
be configured to keep. Kubo currently supports two connection managers:

* none: never close idle connections.
* basic: the default connection manager.

Default: basic

#### `Swarm.ConnMgr.Type`

Sets the type of connection manager to use, options are: `"none"` (no connection
management) and `"basic"`.

Default: "basic".

Type: `string` (when unset or `""`, the default connection manager is applied
and all `ConnMgr` fields are ignored).

#### Basic Connection Manager

The basic connection manager uses a "high water", a "low water", and internal
scoring to periodically close connections to free up resources. When a node
using the basic connection manager reaches `HighWater` idle connections, it will
close the least useful ones until it reaches `LowWater` idle connections.

The connection manager considers a connection idle if:

* It has not been explicitly _protected_ by some subsystem. For example, Bitswap
  will protect connections to peers from which it is actively downloading data,
  the DHT will protect some peers for routing, and the peering subsystem will
  protect all "peered" nodes.
* It has existed for longer than the `GracePeriod`.

**Example:**

```json
{
  "Swarm": {
    "ConnMgr": {
      "Type": "basic",
      "LowWater": 100,
      "HighWater": 200,
      "GracePeriod": "30s"
    }
  }
}
```

##### `Swarm.ConnMgr.LowWater`

LowWater is the number of connections that the basic connection manager will
trim down to.

Default: `600`

Type: `integer`

##### `Swarm.ConnMgr.HighWater`

HighWater is the number of connections that, when exceeded, will trigger a
connection GC operation. Note: protected/recently formed connections don't count
towards this limit.

Default: `900`

Type: `integer`

##### `Swarm.ConnMgr.GracePeriod`

GracePeriod is a time duration that new connections are immune from being closed
by the connection manager.

Default: `"20s"`

Type: `duration`

### `Swarm.ResourceMgr`

**EXPERIMENTAL: `Swarm.ResourceMgr` configuration will change in future release**

The [libp2p Network Resource Manager](https://github.com/libp2p/go-libp2p-resource-manager#readme) allows setting limits per a scope,
and tracking recource usage over time.

#### `Swarm.ResourceMgr.Enabled`

**EXPERIMENTAL: `Swarm.ResourceMgr` is in active development, enable it only if you want to provide maintainers with feedback**


Enables the libp2p Network Resource Manager and auguments the default limits
using user-defined ones in `Swarm.ResourceMgr.Limits` (if present).

Various `*rcmgr_*` metrics can be accessed as the prometheus endpoint at `{Addresses.API}/debug/metrics/prometheus` (default: `http://127.0.0.1:5001/debug/metrics/prometheus`)

Default: `false`

Type: `flag`

#### `Swarm.ResourceMgr.Limits`

**EXPERIMENTAL: `Swarm.ResourceMgr.Limits` configuration will change in future release, exposed here only for convenience**

Map of resource limits [per scope](https://github.com/libp2p/go-libp2p-resource-manager#resource-scopes).

The map supports fields from [`BasicLimiterConfig`](https://github.com/libp2p/go-libp2p-resource-manager/blob/v0.3.0/limit_config.go#L165-L185)
struct from [go-libp2p-resource-manager](https://github.com/libp2p/go-libp2p-resource-manager#readme).

**Example: (format may change in future release)**

```json
{
  "Swarm": {
    "ResourceMgr": {
      "Enabled": true,
      "Limits": {
        "System": {
          "Conns": 1024,
          "ConnsInbound": 256,
          "ConnsOutbound": 1024,
          "FD": 512,
          "Memory": 1073741824,
          "Streams": 16384,
          "StreamsInbound": 4096,
          "StreamsOutbound": 16384
        }
      }
    }
  }
}
```

Current resource usage and a list of services, protocols, and peers can be
obtained via `ipfs swarm stats --help`

It is also possible to adjust some runtime limits via `ipfs stats limit --help`.
Changes made via `stats limit` are persisted in `Swarm.ResourceMgr.Limits`.

Default: `{}` (use the safe implicit defaults)

Type: `object[string->object]`

#### `Swarm.ResourceMgr.Allowlist`

A list of multiaddrs that can bypass normal system limits (but are still limited by the allowlist scope).
Convenience config around [go-libp2p-resource-manager#Allowlist.Add](https://pkg.go.dev/github.com/libp2p/go-libp2p-resource-manager#Allowlist.Add).

Default: `[]`

Type: `array[string]` (multiaddrs)

### `Swarm.Transports`

Configuration section for libp2p transports. An empty configuration will apply
the defaults.

### `Swarm.Transports.Network`

Configuration section for libp2p _network_ transports. Transports enabled in
this section will be used for dialing. However, to receive connections on these
transports, multiaddrs for these transports must be added to `Addresses.Swarm`.

Supported transports are: QUIC, TCP, WS, and Relay.

Each field in this section is a `flag`.

#### `Swarm.Transports.Network.TCP`

[TCP](https://en.wikipedia.org/wiki/Transmission_Control_Protocol) is the most
widely used transport by Kubo nodes. It doesn't directly support encryption
and/or multiplexing, so libp2p will layer a security & multiplexing transport
over it.

Default: Enabled

Type: `flag`

Listen Addresses:
* /ip4/0.0.0.0/tcp/4001 (default)
* /ip6/::/tcp/4001 (default)

#### `Swarm.Transports.Network.Websocket`

[Websocket](https://en.wikipedia.org/wiki/WebSocket) is a transport usually used
to connect to non-browser-based IPFS nodes from browser-based js-ipfs nodes.

While it's enabled by default for dialing, Kubo doesn't listen on this
transport by default.

Default: Enabled

Type: `flag`

Listen Addresses:
* /ip4/0.0.0.0/tcp/4002/ws
* /ip6/::/tcp/4002/ws

#### `Swarm.Transports.Network.QUIC`

[QUIC](https://en.wikipedia.org/wiki/QUIC) is a UDP-based transport with
built-in encryption and multiplexing. The primary benefits over TCP are:

1. It doesn't require a file descriptor per connection, easing the load on the OS.
2. It currently takes 2 round trips to establish a connection (our TCP transport
   currently takes 6).

Default: Enabled

Type: `flag`

Listen Addresses:
* /ip4/0.0.0.0/udp/4001/quic (default)
* /ip6/::/udp/4001/quic (default)

#### `Swarm.Transports.Network.Relay`

[Libp2p Relay](https://github.com/libp2p/specs/tree/master/relay) proxy
transport that forms connections by hopping between multiple libp2p nodes.
Allows IPFS node to connect to other peers using their `/p2p-circuit`
multiaddrs.  This transport is primarily useful for bypassing firewalls and
NATs.

See also:
- Docs: [Libp2p Circuit Relay](https://docs.libp2p.io/concepts/circuit-relay/)
- [`Swarm.RelayClient.Enabled`](#swarmrelayclientenabled) for getting a public
-  `/p2p-circuit` address when behind a firewall.
  - [`Swarm.EnableHolePunching`](#swarmenableholepunching) for direct connection upgrade through relay
- [`Swarm.RelayService.Enabled`](#swarmrelayserviceenabled) for becoming a
  limited relay for other peers

Default: Enabled

Type: `flag`

Listen Addresses:
* This transport is special. Any node that enables this transport can receive
  inbound connections on this transport, without specifying a listen address.

### `Swarm.Transports.Security`

Configuration section for libp2p _security_ transports. Transports enabled in
this section will be used to secure unencrypted connections.

Security transports are configured with the `priority` type.

When establishing an _outbound_ connection, Kubo will try each security
transport in priority order (lower first), until it finds a protocol that the
receiver supports. When establishing an _inbound_ connection, Kubo will let
the initiator choose the protocol, but will refuse to use any of the disabled
transports.

Supported transports are: TLS (priority 100) and Noise (priority 300).

No default priority will ever be less than 100.

#### `Swarm.Transports.Security.TLS`

[TLS](https://github.com/libp2p/specs/tree/master/tls) (1.3) is the default
security transport as of Kubo 0.5.0. It's also the most scrutinized and
trusted security transport.

Default: `100`

Type: `priority`

#### `Swarm.Transports.Security.SECIO`

Support for SECIO has been removed. Please remove this option from your config.

#### `Swarm.Transports.Security.Noise`

[Noise](https://github.com/libp2p/specs/tree/master/noise) is slated to replace
TLS as the cross-platform, default libp2p protocol due to ease of
implementation. It is currently enabled by default but with low priority as it's
not yet widely supported.

Default: `300`

Type: `priority`

### `Swarm.Transports.Multiplexers`

Configuration section for libp2p _multiplexer_ transports. Transports enabled in
this section will be used to multiplex duplex connections.

Multiplexer transports are secured the same way security transports are, with
the `priority` type. Like with security transports, the initiator gets their
first choice.

Supported transports are: Yamux (priority 100) and Mplex (priority 200)

No default priority will ever be less than 100.

### `Swarm.Transports.Multiplexers.Yamux`

Yamux is the default multiplexer used when communicating between Kubo nodes.

Default: `100`

Type: `priority`

### `Swarm.Transports.Multiplexers.Mplex`

Mplex is the default multiplexer used when communicating between Kubo and all
other IPFS and libp2p implementations. Unlike Yamux:

* Mplex is a simpler protocol.
* Mplex is more efficient.
* Mplex does not have built-in keepalives.
* Mplex does not support backpressure. Unfortunately, this means that, if a
  single stream to a peer gets backed up for a period of time, the mplex
  transport will kill the stream to allow the others to proceed. On the other
  hand, the lack of backpressure means mplex can be significantly faster on some
  high-latency connections.

Default: `200`

Type: `priority`

## `DNS`

Options for configuring DNS resolution for [DNSLink](https://docs.ipfs.tech/concepts/dnslink/) and `/dns*` [Multiaddrs](https://github.com/multiformats/multiaddr/).

### `DNS.Resolvers`

Map of [FQDNs](https://en.wikipedia.org/wiki/Fully_qualified_domain_name) to custom resolver URLs.

This allows for overriding the default DNS resolver provided by the operating system,
and using different resolvers per domain or TLD (including ones from alternative, non-ICANN naming systems).

Example:
```json
{
  "DNS": {
    "Resolvers": {
      "eth.": "https://eth.link/dns-query",
      "crypto.": "https://resolver.unstoppable.io/dns-query",
      "libre.": "https://ns1.iriseden.fr/dns-query",
      ".": "https://cloudflare-dns.com/dns-query"
    }
  }
}
```

Be mindful that:
- Currently only `https://` URLs for [DNS over HTTPS (DoH)](https://en.wikipedia.org/wiki/DNS_over_HTTPS) endpoints are supported as values.
- The default catch-all resolver is the cleartext one provided by your operating system. It can be overridden by adding a DoH entry for the DNS root indicated by  `.` as illustrated above.
- Out-of-the-box support for selected decentralized TLDs relies on a [centralized service which is provided on best-effort basis](https://www.cloudflare.com/distributed-web-gateway-terms/). The implicit DoH resolvers are:
  ```json
  {
    "eth.": "https://resolver.cloudflare-eth.com/dns-query",
    "crypto.": "https://resolver.cloudflare-eth.com/dns-query"
  }
  ```
  To get all the benefits of a decentralized naming system we strongly suggest setting DoH endpoint to an empty string and running own decentralized resolver as catch-all one on localhost.

Default: `{}`

Type: `object[string -> string]`

### `DNS.MaxCacheTTL`

Maximum duration for which entries are valid in the DoH cache.

This allows you to cap the Time-To-Live suggested by the DNS response ([RFC2181](https://datatracker.ietf.org/doc/html/rfc2181#section-8)).
If present, the upper bound is applied to DoH resolvers in [`DNS.Resolvers`](#dnsresolvers).

Note: this does NOT work with Go's default DNS resolver. To make this a global setting, add a `.` entry to `DNS.Resolvers` first.

**Examples:**
* `"5m"` DNS entries are kept for 5 minutes or less.
* `"0s"` DNS entries expire as soon as they are retrieved.

Default: Respect DNS Response TTL

Type: `optionalDuration`
