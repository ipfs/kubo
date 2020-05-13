# The go-ipfs config file

The go-ipfs config file is a JSON document located at `$IPFS_PATH/config`. It
is read once at node instantiation, either for an offline command, or when
starting the daemon. Commands that execute on a running daemon do not read the
config file at runtime.

#### Profiles

Configuration profiles allow to tweak configuration quickly. Profiles can be
applied with `--profile` flag to `ipfs init` or with the `ipfs config profile
apply` command. When a profile is applied a backup of the configuration file
will be created in `$IPFS_PATH`.

The available configuration profiles are listed below. You can also find them
documented in `ipfs config profile --help`.

- `server`

  Disables local host discovery, recommended when
  running IPFS on machines with public IPv4 addresses.

- `randomports`

  Use a random port number for swarm.

- `default-datastore`

  Configures the node to use the default datastore (flatfs).

  Read the "flatfs" profile description for more information on this datastore.

  This profile may only be applied when first initializing the node.

- `local-discovery`

  Sets default values to fields affected by the server
  profile, enables discovery in local networks.

- `test`

  Reduces external interference of IPFS daemon, this
  is useful when using the daemon in test environments.

- `default-networking`

  Restores default network settings.
  Inverse profile of the test profile.

- `flatfs`

  Configures the node to use the flatfs datastore.

  This is the most battle-tested and reliable datastore, but it's significantly
  slower than the badger datastore. You should use this datastore if:

  - You need a very simple and very reliable datastore you and trust your
    filesystem. This datastore stores each block as a separate file in the
    underlying filesystem so it's unlikely to loose data unless there's an issue
    with the underlying file system.
  - You need to run garbage collection on a small (<= 10GiB) datastore. The
    default datastore, badger, can leave several gigabytes of data behind when
    garbage collecting.
  - You're concerned about memory usage. In its default configuration, badger can
    use up to several gigabytes of memory.

  This profile may only be applied when first initializing the node.


- `badgerds`

  Configures the node to use the badger datastore.

  This is the fastest datastore. Use this datastore if performance, especially
  when adding many gigabytes of files, is critical. However:

  - This datastore will not properly reclaim space when your datastore is
    smaller than several gigabytes. If you run IPFS with '--enable-gc' (you have
    enabled block-level garbage collection), you plan on storing very little data in
    your IPFS node, and disk usage is more critical than performance, consider using
    flatfs.
  - This datastore uses up to several gigabytes of memory. 

  This profile may only be applied when first initializing the node.

- `lowpower`

  Reduces daemon overhead on the system. May affect node
  functionality - performance of content discovery and data
  fetching may be degraded.

## Table of Contents

- [`Addresses`](#addresses)
    - [`Addresses.API`](#addressesapi)
    - [`Addresses.Gateway`](#addressesgateway)
    - [`Addresses.Swarm`](#addressesswarm)
    - [`Addresses.Announce`](#addressesannounce)
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
- [`Routing`](#routing)
    - [`Routing.Type`](#routingtype)
- [`Gateway`](#gateway)
    - [`Gateway.NoFetch`](#gatewaynofetch)
    - [`Gateway.NoDNSLink`](#gatewaynodnslink)
    - [`Gateway.HTTPHeaders`](#gatewayhttpheaders)
    - [`Gateway.RootRedirect`](#gatewayrootredirect)
    - [`Gateway.Writable`](#gatewaywritable)
    - [`Gateway.PathPrefixes`](#gatewaypathprefixes)
    - [`Gateway.PublicGateways`](#gatewaypublicgateways)
- [`Identity`](#identity)
    - [`Identity.PeerID`](#identitypeerid)
    - [`Identity.PrivKey`](#identityprivkey)
- [`Ipns`](#ipns)
    - [`Ipns.RepublishPeriod`](#ipnsrepublishperiod)
    - [`Ipns.RecordLifetime`](#ipnsrecordlifetime)
    - [`Ipns.ResolveCacheSize`](#ipnsresolvecachesize)
- [`Mounts`](#mounts)
    - [`Mounts.IPFS`](#mountsipfs)
    - [`Mounts.IPNS`](#mountsipns)
    - [`Mounts.FuseAllowOther`](#mountsfuseallowother)
- [`Reprovider`](#reprovider)
    - [`Reprovider.Interval`](#reproviderinterval)
    - [`Reprovider.Strategy`](#reproviderstrategy)
- [`Swarm`](#swarm)
    - [`Swarm.AddrFilters`](#swarmaddrfilters)
    - [`Swarm.DisableBandwidthMetrics`](#swarmdisablebandwidthmetrics)
    - [`Swarm.DisableNatPortMap`](#swarmdisablenatportmap)
    - [`Swarm.DisableRelay`](#swarmdisablerelay)
    - [`Swarm.EnableRelayHop`](#swarmenablerelayhop)
    - [`Swarm.EnableAutoRelay`](#swarmenableautorelay)
    - [`Swarm.ConnMgr`](#swarmconnmgr)
        - [`Swarm.ConnMgr.Type`](#swarmconnmgrtype)
        - [`Swarm.ConnMgr.LowWater`](#swarmconnmgrlowwater)
        - [`Swarm.ConnMgr.HighWater`](#swarmconnmgrhighwater)
        - [`Swarm.ConnMgr.GracePeriod`](#swarmconnmgrgraceperiod)

## `Addresses`

Contains information about various listener addresses to be used by this node.

### `Addresses.API`

Multiaddr or array of multiaddrs describing the address to serve the local HTTP
API on.

Supported Transports:

* tcp/ip{4,6} - `/ipN/.../tcp/...`
* unix - `/unix/path/to/socket`

Default: `/ip4/127.0.0.1/tcp/5001`

### `Addresses.Gateway`

Multiaddr or array of multiaddrs describing the address to serve the local
gateway on.

Supported Transports:

* tcp/ip{4,6} - `/ipN/.../tcp/...`
* unix - `/unix/path/to/socket`

Default: `/ip4/127.0.0.1/tcp/8080`

### `Addresses.Swarm`

Array of multiaddrs describing which addresses to listen on for p2p swarm
connections.

Supported Transports:

* tcp/ip{4,6} - `/ipN/.../tcp/...`
* websocket - `/ipN/.../tcp/.../ws`
* quic - `/ipN/.../udp/.../quic`

Default:
```json
[
  "/ip4/0.0.0.0/tcp/4001",
  "/ip6/::/tcp/4001"
]
```

### `Addresses.Announce`

If non-empty, this array specifies the swarm addresses to announce to the
network. If empty, the daemon will announce inferred swarm addresses.

Default: `[]`

### `Addresses.NoAnnounce`
Array of swarm addresses not to announce to the network.

Default: `[]`

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

### `AutoNAT.Throttle`

When set, this option configure's the AutoNAT services throttling behavior. By
default, go-ipfs will rate-limit the number of NAT checks performed for other
nodes to 30 per minute, and 3 per peer.

### `AutoNAT.Throttle.GlobalLimit`

Configures how many AutoNAT requests to service per `AutoNAT.Throttle.Interval`.

Default: 30

### `AutoNAT.Throttle.PeerLimit`

Configures how many AutoNAT requests per-peer to service per `AutoNAT.Throttle.Interval`.

Default: 3

### `AutoNAT.Throttle.Interval`

Configures the interval for the above limits.

Default: 1 Minute

## `Bootstrap`

Bootstrap is an array of multiaddrs of trusted nodes to connect to in order to
initiate a connection to the network.

Default: The ipfs.io bootstrap nodes

## `Datastore`

Contains information related to the construction and operation of the on-disk
storage system.

### `Datastore.StorageMax`

A soft upper limit for the size of the ipfs repository's datastore. With `StorageGCWatermark`,
is used to calculate whether to trigger a gc run (only if `--enable-gc` flag is set).

Default: `10GB`

### `Datastore.StorageGCWatermark`

The percentage of the `StorageMax` value at which a garbage collection will be
triggered automatically if the daemon was run with automatic gc enabled (that
option defaults to false currently).

Default: `90`

### `Datastore.GCPeriod`

A time duration specifying how frequently to run a garbage collection. Only used
if automatic gc is enabled.

Default: `1h`

### `Datastore.HashOnRead`

A boolean value. If set to true, all block reads from disk will be hashed and
verified. This will cause increased CPU utilization.

Default: `false`

### `Datastore.BloomFilterSize`

A number representing the size in bytes of the blockstore's [bloom
filter](https://en.wikipedia.org/wiki/Bloom_filter). A value of zero represents
the feature being disabled.

This site generates useful graphs for various bloom filter values:
<https://hur.st/bloomfilter/?n=1e6&p=0.01&m=&k=7> You may use it to find a
preferred optimal value, where `m` is `BloomFilterSize` in bits. Remember to
convert the value `m` from bits, into bytes for use as `BloomFilterSize` in the
config file. For example, for 1,000,000 blocks, expecting a 1% false positive
rate, you'd end up with a filter size of 9592955 bits, so for `BloomFilterSize`
we'd want to use 1199120 bytes. As of writing, [7 hash
functions](https://github.com/ipfs/go-ipfs-blockstore/blob/547442836ade055cc114b562a3cc193d4e57c884/caching.go#L22)
are used, so the constant `k` is 7 in the formula.


Default: `0`

### `Datastore.Spec`

Spec defines the structure of the ipfs datastore. It is a composable structure,
where each datastore is represented by a json object. Datastores can wrap other
datastores to provide extra functionality (eg metrics, logging, or caching).

This can be changed manually, however, if you make any changes that require a
different on-disk structure, you will need to run the [ipfs-ds-convert
tool](https://github.com/ipfs/ipfs-ds-convert) to migrate data into the new
structures.

For more information on possible values for this configuration option, see
docs/datastores.md

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

## `Discovery`

Contains options for configuring ipfs node discovery mechanisms.

### `Discovery.MDNS`

Options for multicast dns peer discovery.

#### `Discovery.MDNS.Enabled`

A boolean value for whether or not mdns should be active.

Default: `true`

#### `Discovery.MDNS.Interval`

A number of seconds to wait between discovery checks.

## `Routing`

Contains options for content, peer, and IPNS routing mechanisms.

### `Routing.Type`

Content routing mode. Can be overridden with daemon `--routing` flag.

There are two core routing options: "none" and "dht" (default).

* If set to "none", your node will use _no_ routing system. You'll have to
  explicitly connect to peers that have the content you're looking for.
* If set to "dht" (or "dhtclient"/"dhtserver"), your node will use the IPFS DHT.

When the DHT is enabled, it can operate in two modes: client and server.

* In server mode, your node will query other peers for DHT records, and will
  respond to requests from other peers (both requests to store records and
  requests to retrieve records).
* In client mode, your node will query the DHT as a client but will not respond
  to requests from other peers. This mode is less resource intensive than server
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
  

## `Gateway`

Options for the HTTP gateway.

### `Gateway.NoFetch`

When set to true, the gateway will only serve content already in the local repo
and will not fetch files from the network.

Default: `false`

### `Gateway.NoDNSLink`

A boolean to configure whether DNSLink lookup for value in `Host` HTTP header
should be performed.  If DNSLink is present, content path stored in the DNS TXT
record becomes the `/` and respective payload is returned to the client.

Default: `false`

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

### `Gateway.RootRedirect`

A url to redirect requests for `/` to.

Default: `""`

### `Gateway.Writable`

A boolean to configure whether the gateway is writeable or not.

Default: `false`

### `Gateway.PathPrefixes`

Array of acceptable url paths that a client can specify in X-Ipfs-Path-Prefix
header.

The X-Ipfs-Path-Prefix header is used to specify a base path to prepend to links
in directory listings and for trailing-slash redirects. It is intended to be set
by a frontend http proxy like nginx.

Example: We mount `blog.ipfs.io` (a dnslink page) at `ipfs.io/blog`.

**.ipfs/config**
```json
"Gateway": {
  "PathPrefixes": ["/blog"],
```

**nginx_ipfs.conf**
```nginx
location /blog/ {
  rewrite "^/blog(/.*)$" $1 break;
  proxy_set_header Host blog.ipfs.io;
  proxy_set_header X-Ipfs-Gateway-Prefix /blog;
  proxy_pass http://127.0.0.1:8080;
}
```

Default: `[]`


### `Gateway.PublicGateways`

`PublicGateways` is a dictionary for defining gateway behavior on specified hostnames.

#### `Gateway.PublicGateways: Paths`

Array of paths that should be exposed on the hostname.

Example: 
```json
{
  "Gateway": {
    "PublicGateways": {
      "example.com": {
        "Paths": ["/ipfs", "/ipns"],
```

Above enables `http://example.com/ipfs/*` and `http://example.com/ipns/*` but not `http://example.com/api/*`

Default: `[]`

#### `Gateway.PublicGateways: UseSubdomains`

A boolean to configure whether the gateway at the hostname provides [Origin isolation](https://developer.mozilla.org/en-US/docs/Web/Security/Same-origin_policy)
between content roots.

- `true` - enables [subdomain gateway](#https://docs-beta.ipfs.io/how-to/address-ipfs-on-web/#subdomain-gateway) at `http://*.{hostname}/`
    - **Requires whitelist:** make sure respective `Paths` are set.
    For example, `Paths: ["/ipfs", "/ipns"]` are required for `http://{cid}.ipfs.{hostname}` and `http://{foo}.ipns.{hostname}` to work:
        ```json
        {
        "Gateway": {
            "PublicGateways": {
            "dweb.link": {
                "UseSubdomains": true,
                "Paths": ["/ipfs", "/ipns"],
        ```
    - **Backward-compatible:** requests for content paths such as `http://{hostname}/ipfs/{cid}` produce redirect to `http://{cid}.ipfs.{hostname}`
    - **API:** if `/api` is on the `Paths` whitelist, `http://{hostname}/api/{cmd}` produces redirect to `http://api.{hostname}/api/{cmd}`

- `false` - enables [path gateway](https://docs-beta.ipfs.io/how-to/address-ipfs-on-web/#path-gateway) at `http://{hostname}/*`
  - Example:
    ```json
    {
    "Gateway": {
        "PublicGateways": {
        "ipfs.io": {
            "UseSubdomains": false,
            "Paths": ["/ipfs", "/ipns", "/api"],
    ```
<!-- **(not implemented yet)** due to the lack of Origin isolation, cookies and storage on `Paths` will be disabled by [Clear-Site-Data](https://github.com/ipfs/in-web-browsers/issues/157) header -->

Default: `false`


#### `Gateway.PublicGateways: NoDNSLink`

A boolean to configure whether DNSLink for hostname present in `Host`
HTTP header should be resolved. Overrides global setting.
If `Paths` are defined, they take priority over DNSLink.

Default: `false` (DNSLink lookup enabled by default for every defined hostname)

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

* Public [subdomain gateway](https://docs-beta.ipfs.io/how-to/address-ipfs-on-web/#subdomain-gateway) at `http://{cid}.ipfs.dweb.link` (each content root gets its own Origin)
   ```console
   $ ipfs config --json Gateway.PublicGateways '{
       "dweb.link": {
         "UseSubdomains": true,
         "Paths": ["/ipfs", "/ipns"]
       }
     }'
   ```
   **Note I:** this enables automatic redirects from content paths to subdomains:  
   `http://dweb.link/ipfs/{cid}` → `http://{cid}.ipfs.dweb.link`  
   **Note II:** if you run go-ipfs behind a reverse proxy that provides TLS, make it adds a `X-Forwarded-Proto: https` HTTP header to ensure users are redirected to `https://`, not `http://`. The NGINX directive is `proxy_set_header X-Forwarded-Proto "https";`.:    
   `https://dweb.link/ipfs/{cid}` → `https://{cid}.ipfs.dweb.link`

* Public [path gateway](https://docs-beta.ipfs.io/how-to/address-ipfs-on-web/#path-gateway) at `http://ipfs.io/ipfs/{cid}` (no Origin separation)
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
  $ ipfs config --json Gateway.NoDNSLink true
  ```
  * Note that `NoDNSLink: false` is the default (it works out of the box unless set to `true` manually)

* Hardened, site-specific [DNSLink gateway](https://docs-beta.ipfs.io/how-to/address-ipfs-on-web/#dnslink-gateway).  
  Disable fetching of remote data (`NoFetch: true`)
  and resolving DNSLink at unknown hostnames (`NoDNSLink: true`).
  Then, enable DNSLink gateway only for the specific hostname (for which data
  is already present on the node), without exposing any content-addressing `Paths`:
      "NoFetch": true,
      "NoDNSLink": true,
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

### `Identity.PrivKey`

The base64 encoded protobuf describing (and containing) the nodes private key.

## `Ipns`

### `Ipns.RepublishPeriod`

A time duration specifying how frequently to republish ipns records to ensure
they stay fresh on the network. If unset, we default to 4 hours.

### `Ipns.RecordLifetime`

A time duration specifying the value to set on ipns records for their validity
lifetime.

If unset, we default to 24 hours.

### `Ipns.ResolveCacheSize`

The number of entries to store in an LRU cache of resolved ipns entries. Entries
will be kept cached until their lifetime is expired.

Default: `128`

## `Mounts`

FUSE mount point configuration options.

### `Mounts.IPFS`

Mountpoint for `/ipfs/`.

### `Mounts.IPNS`

Mountpoint for `/ipns/`.

### `Mounts.FuseAllowOther`

Sets the FUSE allow other option on the mountpoint.

## `Reprovider`

### `Reprovider.Interval`

Sets the time between rounds of reproviding local content to the routing
system. If unset, it defaults to 12 hours. If set to the value `"0"` it will
disable content reproviding.

Note: disabling content reproviding will result in other nodes on the network
not being able to discover that you have the objects that you have. If you want
to have this disabled and keep the network aware of what you have, you must
manually announce your content periodically.

### `Reprovider.Strategy`

Tells reprovider what should be announced. Valid strategies are:
  - "all" (default) - announce all stored data
  - "pinned" - only announce pinned data
  - "roots" - only announce directly pinned keys and root keys of recursive pins

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


### `Swarm.DisableBandwidthMetrics`

A boolean value that when set to true, will cause ipfs to not keep track of
bandwidth metrics. Disabling bandwidth metrics can lead to a slight performance
improvement, as well as a reduction in memory usage.

### `Swarm.DisableNatPortMap`

Disable automatic NAT port forwarding.

When not disabled (default), go-ipfs asks NAT devices (e.g., routers), to open
up an external port and forward it to the port go-ipfs is running on. When this
works (i.e., when your router supports NAT port forwarding), it makes the local
go-ipfs node accessible from the public internet.

### `Swarm.DisableRelay`

Disables the p2p-circuit relay transport. This will prevent this node from
connecting to nodes behind relays, or accepting connections from nodes behind
relays.

### `Swarm.EnableRelayHop`

Configures this node to act as a relay "hop". A relay "hop" relays traffic for other peers.

WARNING: Do not enable this option unless you know what you're doing. Other
peers will randomly decide to use your node as a relay and consume _all_
available bandwidth. There is _no_ rate-limiting.

### `Swarm.EnableAutoRelay`

Enables "automatic relay" mode for this node. This option does two _very_
different things based on the `Swarm.EnableRelayHop`. See
[#7228](https://github.com/ipfs/go-ipfs/issues/7228) for context.

#### Mode 1: `EnableRelayHop` is `false`

If `Swarm.EnableAutoRelay` is enabled and `Swarm.EnableRelayHop` is disabled,
your node will automatically _use_ public relays from the network if it detects
that it cannot be reached from the public internet (e.g., it's behind a
firewall). This is likely the feature you're looking for.

If you enable `EnableAutoRelay`, you should almost certainly disable
`EnableRelayHop`.

#### Mode 2: `EnableRelayHop` is `true`

If `EnableAutoRelay` is enabled and `EnableRelayHop` is enabled, your node will
_act_ as a public relay for the network. Furthermore, in addition to simply
relaying traffic, your node will advertise itself as a public relay. Unless you
have the bandwidth of a small ISP, do not enable both of these options at the
same time.

### `Swarm.EnableAutoNATService`

**REMOVED**

Please use [`AutoNAT.ServiceMode`][].

### `Swarm.ConnMgr`

The connection manager determines which and how many connections to keep and can
be configured to keep.

#### `Swarm.ConnMgr.Type`

Sets the type of connection manager to use, options are: `"none"` (no connection
management) and `"basic"`.

#### Basic Connection Manager

##### `Swarm.ConnMgr.LowWater`

LowWater is the minimum number of connections to maintain.

##### `Swarm.ConnMgr.HighWater`

HighWater is the number of connections that, when exceeded, will trigger a
connection GC operation.

##### `Swarm.ConnMgr.GracePeriod`

GracePeriod is a time duration that new connections are immune from being closed
by the connection manager.

The "basic" connection manager tries to keep between `LowWater` and `HighWater`
connections. It works by:

1. Keeping all connections until `HighWater` connections is reached.
2. Once `HighWater` is reached, it closes connections until `LowWater` is
   reached.
3. To prevent thrashing, it never closes connections established within the
   `GracePeriod`.

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
