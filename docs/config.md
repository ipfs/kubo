# The Kubo config file

The Kubo config file is a JSON document located at `$IPFS_PATH/config`. It
is read once at node instantiation, either for an offline command, or when
starting the daemon. Commands that execute on a running daemon do not read the
config file at runtime.

# Table of Contents

- [The Kubo config file](#the-kubo-config-file)
- [Table of Contents](#table-of-contents)
  - [`Addresses`](#addresses)
    - [`Addresses.API`](#addressesapi)
    - [`Addresses.Gateway`](#addressesgateway)
    - [`Addresses.Swarm`](#addressesswarm)
    - [`Addresses.Announce`](#addressesannounce)
    - [`Addresses.AppendAnnounce`](#addressesappendannounce)
    - [`Addresses.NoAnnounce`](#addressesnoannounce)
  - [`API`](#api)
    - [`API.HTTPHeaders`](#apihttpheaders)
    - [`API.Authorizations`](#apiauthorizations)
      - [`API.Authorizations: AuthSecret`](#apiauthorizations-authsecret)
      - [`API.Authorizations: AllowedPaths`](#apiauthorizations-allowedpaths)
  - [`AutoNAT`](#autonat)
    - [`AutoNAT.ServiceMode`](#autonatservicemode)
    - [`AutoNAT.Throttle`](#autonatthrottle)
    - [`AutoNAT.Throttle.GlobalLimit`](#autonatthrottlegloballimit)
    - [`AutoNAT.Throttle.PeerLimit`](#autonatthrottlepeerlimit)
    - [`AutoNAT.Throttle.Interval`](#autonatthrottleinterval)
  - [`AutoTLS`](#autotls)
    - [`AutoTLS.Enabled`](#autotlsenabled)
    - [`AutoTLS.AutoWSS`](#autotlsautowss)
    - [`AutoTLS.ShortAddrs`](#autotlsshortaddrs)
    - [`AutoTLS.DomainSuffix`](#autotlsdomainsuffix)
    - [`AutoTLS.RegistrationEndpoint`](#autotlsregistrationendpoint)
    - [`AutoTLS.RegistrationToken`](#autotlsregistrationtoken)
    - [`AutoTLS.RegistrationDelay`](#autotlsregistrationdelay)
    - [`AutoTLS.CAEndpoint`](#autotlscaendpoint)
  - [`AutoConf`](#autoconf)
    - [`AutoConf.URL`](#autoconfurl)
    - [`AutoConf.Enabled`](#autoconfenabled)
    - [`AutoConf.RefreshInterval`](#autoconfrefreshinterval)
    - [`AutoConf.TLSInsecureSkipVerify`](#autoconftlsinsecureskipverify)
  - [`Bitswap`](#bitswap)
    - [`Bitswap.Libp2pEnabled`](#bitswaplibp2penabled)
    - [`Bitswap.ServerEnabled`](#bitswapserverenabled)
  - [`Bootstrap`](#bootstrap)
  - [`Datastore`](#datastore)
    - [`Datastore.StorageMax`](#datastorestoragemax)
    - [`Datastore.StorageGCWatermark`](#datastorestoragegcwatermark)
    - [`Datastore.GCPeriod`](#datastoregcperiod)
    - [`Datastore.HashOnRead`](#datastorehashonread)
    - [`Datastore.BloomFilterSize`](#datastorebloomfiltersize)
    - [`Datastore.WriteThrough`](#datastorewritethrough)
    - [`Datastore.BlockKeyCacheSize`](#datastoreblockkeycachesize)
    - [`Datastore.Spec`](#datastorespec)
  - [`Discovery`](#discovery)
    - [`Discovery.MDNS`](#discoverymdns)
      - [`Discovery.MDNS.Enabled`](#discoverymdnsenabled)
      - [`Discovery.MDNS.Interval`](#discoverymdnsinterval)
  - [`Experimental`](#experimental)
  - [`Gateway`](#gateway)
    - [`Gateway.NoFetch`](#gatewaynofetch)
    - [`Gateway.NoDNSLink`](#gatewaynodnslink)
    - [`Gateway.DeserializedResponses`](#gatewaydeserializedresponses)
    - [`Gateway.DisableHTMLErrors`](#gatewaydisablehtmlerrors)
    - [`Gateway.ExposeRoutingAPI`](#gatewayexposeroutingapi)
    - [`Gateway.RetrievalTimeout`](#gatewayretrievaltimeout)
    - [`Gateway.MaxConcurrentRequests`](#gatewaymaxconcurrentrequests)
    - [`Gateway.HTTPHeaders`](#gatewayhttpheaders)
    - [`Gateway.RootRedirect`](#gatewayrootredirect)
    - [`Gateway.DiagnosticServiceURL`](#gatewaydiagnosticserviceurl)
    - [`Gateway.FastDirIndexThreshold`](#gatewayfastdirindexthreshold)
    - [`Gateway.Writable`](#gatewaywritable)
    - [`Gateway.PathPrefixes`](#gatewaypathprefixes)
    - [`Gateway.PublicGateways`](#gatewaypublicgateways)
      - [`Gateway.PublicGateways: Paths`](#gatewaypublicgateways-paths)
      - [`Gateway.PublicGateways: UseSubdomains`](#gatewaypublicgateways-usesubdomains)
      - [`Gateway.PublicGateways: NoDNSLink`](#gatewaypublicgateways-nodnslink)
      - [`Gateway.PublicGateways: InlineDNSLink`](#gatewaypublicgateways-inlinednslink)
      - [`Gateway.PublicGateways: DeserializedResponses`](#gatewaypublicgateways-deserializedresponses)
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
      - [`Internal.Bitswap.ProviderSearchDelay`](#internalbitswapprovidersearchdelay)
      - [`Internal.Bitswap.ProviderSearchMaxResults`](#internalbitswapprovidersearchmaxresults)
      - [`Internal.Bitswap.BroadcastControl`](#internalbitswapbroadcastcontrol)
        - [`Internal.Bitswap.BroadcastControl.Enable`](#internalbitswapbroadcastcontrolenable)
        - [`Internal.Bitswap.BroadcastControl.MaxPeers`](#internalbitswapbroadcastcontrolmaxpeers)
        - [`Internal.Bitswap.BroadcastControl.LocalPeers`](#internalbitswapbroadcastcontrollocalpeers)
        - [`Internal.Bitswap.BroadcastControl.PeeredPeers`](#internalbitswapbroadcastcontrolpeeredpeers)
        - [`Internal.Bitswap.BroadcastControl.MaxRandomPeers`](#internalbitswapbroadcastcontrolmaxrandompeers)
        - [`Internal.Bitswap.BroadcastControl.SendToPendingPeers`](#internalbitswapbroadcastcontrolsendtopendingpeers)
    - [`Internal.UnixFSShardingSizeThreshold`](#internalunixfsshardingsizethreshold)
  - [`Ipns`](#ipns)
    - [`Ipns.RepublishPeriod`](#ipnsrepublishperiod)
    - [`Ipns.RecordLifetime`](#ipnsrecordlifetime)
    - [`Ipns.ResolveCacheSize`](#ipnsresolvecachesize)
    - [`Ipns.MaxCacheTTL`](#ipnsmaxcachettl)
    - [`Ipns.UsePubsub`](#ipnsusepubsub)
    - [`Ipns.DelegatedPublishers`](#ipnsdelegatedpublishers)
  - [`Migration`](#migration)
    - [`Migration.DownloadSources`](#migrationdownloadsources)
    - [`Migration.Keep`](#migrationkeep)
  - [`Mounts`](#mounts)
    - [`Mounts.IPFS`](#mountsipfs)
    - [`Mounts.IPNS`](#mountsipns)
    - [`Mounts.MFS`](#mountsmfs)
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
  - [`Provide`](#provide)
    - [`Provide.Enabled`](#provideenabled)
    - [`Provide.Strategy`](#providestrategy)
    - [`Provide.DHT`](#providedht)
      - [`Provide.DHT.MaxWorkers`](#providedhtmaxworkers)
      - [`Provide.DHT.Interval`](#providedhtinterval)
      - [`Provide.DHT.SweepEnabled`](#providedhtsweepenabled)
      - [`Provide.DHT.DedicatedPeriodicWorkers`](#providedhtdedicatedperiodicworkers)
      - [`Provide.DHT.DedicatedBurstWorkers`](#providedhtdedicatedburstworkers)
      - [`Provide.DHT.MaxProvideConnsPerWorker`](#providedhtmaxprovideconnsperworker)
      - [`Provide.DHT.KeystoreBatchSize`](#providedhtkeystorebatchsize)
      - [`Provide.DHT.OfflineDelay`](#providedhtofflinedelay)
  - [`Provider`](#provider)
    - [`Provider.Enabled`](#providerenabled)
    - [`Provider.Strategy`](#providerstrategy)
    - [`Provider.WorkerCount`](#providerworkercount)
  - [`Pubsub`](#pubsub)
    - [`Pubsub.Enabled`](#pubsubenabled)
    - [`Pubsub.Router`](#pubsubrouter)
    - [`Pubsub.DisableSigning`](#pubsubdisablesigning)
    - [`Pubsub.SeenMessagesTTL`](#pubsubseenmessagesttl)
    - [`Pubsub.SeenMessagesStrategy`](#pubsubseenmessagesstrategy)
  - [`Peering`](#peering)
    - [`Peering.Peers`](#peeringpeers)
  - [`Reprovider`](#reprovider)
    - [`Reprovider.Interval`](#reproviderinterval)
    - [`Reprovider.Strategy`](#providestrategy)
  - [`Routing`](#routing)
    - [`Routing.Type`](#routingtype)
    - [`Routing.AcceleratedDHTClient`](#routingaccelerateddhtclient)
    - [`Routing.LoopbackAddressesOnLanDHT`](#routingloopbackaddressesonlandht)
    - [`Routing.IgnoreProviders`](#routingignoreproviders)
    - [`Routing.DelegatedRouters`](#routingdelegatedrouters)
    - [`Routing.Routers`](#routingrouters)
      - [`Routing.Routers: Type`](#routingrouters-type)
      - [`Routing.Routers: Parameters`](#routingrouters-parameters)
    - [`Routing: Methods`](#routing-methods)
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
        - [`Swarm.ConnMgr.SilencePeriod`](#swarmconnmgrsilenceperiod)
    - [`Swarm.ResourceMgr`](#swarmresourcemgr)
      - [`Swarm.ResourceMgr.Enabled`](#swarmresourcemgrenabled)
      - [`Swarm.ResourceMgr.MaxMemory`](#swarmresourcemgrmaxmemory)
      - [`Swarm.ResourceMgr.MaxFileDescriptors`](#swarmresourcemgrmaxfiledescriptors)
      - [`Swarm.ResourceMgr.Allowlist`](#swarmresourcemgrallowlist)
    - [`Swarm.Transports`](#swarmtransports)
    - [`Swarm.Transports.Network`](#swarmtransportsnetwork)
      - [`Swarm.Transports.Network.TCP`](#swarmtransportsnetworktcp)
      - [`Swarm.Transports.Network.Websocket`](#swarmtransportsnetworkwebsocket)
      - [`Swarm.Transports.Network.QUIC`](#swarmtransportsnetworkquic)
      - [`Swarm.Transports.Network.Relay`](#swarmtransportsnetworkrelay)
      - [`Swarm.Transports.Network.WebTransport`](#swarmtransportsnetworkwebtransport)
      - [`Swarm.Transports.Network.WebRTCDirect`](#swarmtransportsnetworkwebrtcdirect)
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
  - [`HTTPRetrieval`](#httpretrieval)
    - [`HTTPRetrieval.Enabled`](#httpretrievalenabled)
    - [`HTTPRetrieval.Allowlist`](#httpretrievalallowlist)
    - [`HTTPRetrieval.Denylist`](#httpretrievaldenylist)
    - [`HTTPRetrieval.NumWorkers`](#httpretrievalnumworkers)
    - [`HTTPRetrieval.MaxBlockSize`](#httpretrievalmaxblocksize)
    - [`HTTPRetrieval.TLSInsecureSkipVerify`](#httpretrievaltlsinsecureskipverify)
  - [`Import`](#import)
    - [`Import.CidVersion`](#importcidversion)
    - [`Import.UnixFSRawLeaves`](#importunixfsrawleaves)
    - [`Import.UnixFSChunker`](#importunixfschunker)
    - [`Import.HashFunction`](#importhashfunction)
    - [`Import.BatchMaxNodes`](#importbatchmaxnodes)
    - [`Import.BatchMaxSize`](#importbatchmaxsize)
    - [`Import.UnixFSFileMaxLinks`](#importunixfsfilemaxlinks)
    - [`Import.UnixFSDirectoryMaxLinks`](#importunixfsdirectorymaxlinks)
    - [`Import.UnixFSHAMTDirectoryMaxFanout`](#importunixfshamtdirectorymaxfanout)
    - [`Import.UnixFSHAMTDirectorySizeThreshold`](#importunixfshamtdirectorysizethreshold)
  - [`Version`](#version)
    - [`Version.AgentSuffix`](#versionagentsuffix)
    - [`Version.SwarmCheckEnabled`](#versionswarmcheckenabled)
    - [`Version.SwarmCheckPercentThreshold`](#versionswarmcheckpercentthreshold)
  - [Profiles](#profiles)
    - [`server` profile](#server-profile)
    - [`randomports` profile](#randomports-profile)
    - [`default-datastore` profile](#default-datastore-profile)
    - [`local-discovery` profile](#local-discovery-profile)
    - [`default-networking` profile](#default-networking-profile)
    - [`autoconf-on` profile](#autoconf-on-profile)
    - [`autoconf-off` profile](#autoconf-off-profile)
    - [`flatfs` profile](#flatfs-profile)
    - [`flatfs-measure` profile](#flatfs-measure-profile)
    - [`pebbleds` profile](#pebbleds-profile)
    - [`pebbleds-measure` profile](#pebbleds-measure-profile)
    - [`badgerds` profile](#badgerds-profile)
    - [`badgerds-measure` profile](#badgerds-measure-profile)
    - [`lowpower` profile](#lowpower-profile)
    - [`announce-off` profile](#announce-off-profile)
    - [`announce-on` profile](#announce-on-profile)
    - [`legacy-cid-v0` profile](#legacy-cid-v0-profile)
    - [`test-cid-v1` profile](#test-cid-v1-profile)
    - [`test-cid-v1-wide` profile](#test-cid-v1-wide-profile)
  - [Security](#security)
    - [Port and Network Exposure](#port-and-network-exposure)
    - [Security Best Practices](#security-best-practices)
  - [Types](#types)
    - [`flag`](#flag)
    - [`priority`](#priority)
    - [`strings`](#strings)
    - [`duration`](#duration)
    - [`optionalInteger`](#optionalinteger)
    - [`optionalBytes`](#optionalbytes)
    - [`optionalString`](#optionalstring)
    - [`optionalDuration`](#optionalduration)

## `Addresses`

Contains information about various listener addresses to be used by this node.

### `Addresses.API`

[Multiaddr][multiaddr] or array of multiaddrs describing the addresses to serve
the local [Kubo RPC API](https://docs.ipfs.tech/reference/kubo/rpc/) (`/api/v0`).

Supported Transports:

- tcp/ip{4,6} - `/ipN/.../tcp/...`
- unix - `/unix/path/to/socket`

> [!CAUTION]
> **NEVER EXPOSE UNPROTECTED ADMIN RPC TO LAN OR THE PUBLIC INTERNET**
>
> The RPC API grants admin-level access to your Kubo IPFS node, including
> configuration and secret key management.
>
> By default, it is bound to localhost for security reasons. Exposing it to LAN
> or the public internet is highly risky—similar to exposing a SQL database or
> backend service without authentication middleware
>
> - If you need secure access to a subset of RPC, secure it with [`API.Authorizations`](#apiauthorizations) or custom auth middleware running in front of the localhost-only RPC port defined here.
> - If you are looking for an interface designed for browsers and public internet, use [`Addresses.Gateway`](#addressesgateway) port instead.
> - See [Security section](#security) for network exposure considerations.

Default: `/ip4/127.0.0.1/tcp/5001`

Type: `strings` ([multiaddrs][multiaddr])

### `Addresses.Gateway`

[Multiaddr][multiaddr] or array of multiaddrs describing the address to serve
the local [HTTP gateway](https://specs.ipfs.tech/http-gateways/) (`/ipfs`, `/ipns`) on.

Supported Transports:

- tcp/ip{4,6} - `/ipN/.../tcp/...`
- unix - `/unix/path/to/socket`

> [!CAUTION]
> **SECURITY CONSIDERATIONS FOR GATEWAY EXPOSURE**
>
> By default, the gateway is bound to localhost for security. If you bind to `0.0.0.0`
> or a public IP, anyone with access can trigger retrieval of arbitrary CIDs, causing
> bandwidth usage and potential exposure to malicious content. Limit with
> [`Gateway.NoFetch`](#gatewaynofetch). Consider firewall rules, authentication,
> and [`Gateway.PublicGateways`](#gatewaypublicgateways) for public exposure.
> See [Security section](#security) for network exposure considerations.

Default: `/ip4/127.0.0.1/tcp/8080`

Type: `strings` ([multiaddrs][multiaddr])

### `Addresses.Swarm`

An array of [multiaddrs][multiaddr] describing which addresses to listen on for p2p swarm
connections.

Supported Transports:

- tcp/ip{4,6} - `/ipN/.../tcp/...`
- websocket - `/ipN/.../tcp/.../ws`
- quicv1 (RFC9000) - `/ipN/.../udp/.../quic-v1` - can share the same two tuple with `/quic-v1/webtransport`
- webtransport `/ipN/.../udp/.../quic-v1/webtransport` - can share the same two tuple with `/quic-v1`

> [!IMPORTANT]
> Make sure your firewall rules allow incoming connections on both TCP and UDP ports defined here.
> See [Security section](#security) for network exposure considerations.

Note that quic (Draft-29) used to be supported with the format `/ipN/.../udp/.../quic`, but has since been [removed](https://github.com/libp2p/go-libp2p/releases/tag/v0.30.0).

Default:

```json
[
  "/ip4/0.0.0.0/tcp/4001",
  "/ip6/::/tcp/4001",
  "/ip4/0.0.0.0/udp/4001/quic-v1",
  "/ip4/0.0.0.0/udp/4001/quic-v1/webtransport",
  "/ip6/::/udp/4001/quic-v1",
  "/ip6/::/udp/4001/quic-v1/webtransport"
]
```

Type: `array[string]` ([multiaddrs][multiaddr])

### `Addresses.Announce`

If non-empty, this array specifies the swarm addresses to announce to the
network. If empty, the daemon will announce inferred swarm addresses.

Default: `[]`

Type: `array[string]` ([multiaddrs][multiaddr])

### `Addresses.AppendAnnounce`

Similar to [`Addresses.Announce`](#addressesannounce) except this doesn't
override inferred swarm addresses if non-empty.

Default: `[]`

Type: `array[string]` ([multiaddrs][multiaddr])

### `Addresses.NoAnnounce`

An array of swarm addresses not to announce to the network.
Takes precedence over `Addresses.Announce` and `Addresses.AppendAnnounce`.

> [!TIP]
> The [`server` configuration profile](#server-profile) fills up this list with sensible defaults,
> preventing announcement of non-routable IP addresses (e.g., `/ip4/192.168.0.0/ipcidr/16`,
> which is the [multiaddress][multiaddr] representation of `192.168.0.0/16`) but you should always
> check settings against your own network and/or hosting provider.

Default: `[]`

Type: `array[string]` ([multiaddrs][multiaddr])

## `API`

Contains information used by the [Kubo RPC API](https://docs.ipfs.tech/reference/kubo/rpc/).

### `API.HTTPHeaders`

Map of HTTP headers to set on responses from the RPC (`/api/v0`) HTTP server.

Example:

```json
{
  "Foo": ["bar"]
}
```

Default: `null`

Type: `object[string -> array[string]]` (header names -> array of header values)

### `API.Authorizations`

The `API.Authorizations` field defines user-based access restrictions for the
[Kubo RPC API](https://docs.ipfs.tech/reference/kubo/rpc/), which is located at
`Addresses.API` under `/api/v0` paths.

By default, the admin-level RPC API is accessible without restrictions as it is only
exposed on `127.0.0.1` and safeguarded with Origin check and implicit
[CORS](https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS) headers that
block random websites from accessing the RPC.

When entries are defined in `API.Authorizations`, RPC requests will be declined
unless a corresponding secret is present in the HTTP [`Authorization` header](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Authorization),
and the requested path is included in the `AllowedPaths` list for that specific
secret.

> [!CAUTION]
> **NEVER EXPOSE UNPROTECTED ADMIN RPC TO LAN OR THE PUBLIC INTERNET**
>
> The RPC API is vast. It grants admin-level access to your Kubo IPFS node, including
> configuration and secret key management.
>
> - If you need secure access to a subset of RPC, make sure you understand the risk, block everything by default and allow basic auth access with [`API.Authorizations`](#apiauthorizations) or custom auth middleware running in front of the localhost-only port defined in [`Addresses.API`](#addressesapi).
> - If you are looking for an interface designed for browsers and public internet, use [`Addresses.Gateway`](#addressesgateway) port instead.

Default: `null`

Type: `object[string -> object]` (user name -> authorization object, see below)

For example, to limit RPC access to Alice (access `id` and MFS `files` commands with HTTP Basic Auth)
and Bob (full access with Bearer token):

```json
{
  "API": {
    "Authorizations": {
      "Alice": {
        "AuthSecret": "basic:alice:password123",
        "AllowedPaths": ["/api/v0/id", "/api/v0/files"]
      },
      "Bob": {
        "AuthSecret": "bearer:secret-token123",
        "AllowedPaths": ["/api/v0"]
      }
    }
  }
}

```

#### `API.Authorizations: AuthSecret`

The `AuthSecret` field denotes the secret used by a user to authenticate,
usually via HTTP [`Authorization` header](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Authorization).

Field format is `type:value`, and the following types are supported:

- `bearer:` For secret Bearer tokens, set as `bearer:token`.
  - If no known `type:` prefix is present, `bearer:` is assumed.
- `basic`: For HTTP Basic Auth introduced in [RFC7617](https://datatracker.ietf.org/doc/html/rfc7617). Value can be:
  - `basic:user:pass`
  - `basic:base64EncodedBasicAuth`

One can use the config value for authentication via the command line:

```
ipfs id --api-auth basic:user:pass
```

Type: `string`

#### `API.Authorizations: AllowedPaths`

The `AllowedPaths` field is an array of strings containing allowed RPC path
prefixes. Users authorized with the related `AuthSecret` will only be able to
access paths prefixed by the specified prefixes.

For instance:

- If set to `["/api/v0"]`, the user will have access to the complete RPC API.
- If set to `["/api/v0/id", "/api/v0/files"]`, the user will only have access
  to the `id` command and all MFS commands under `files`.

Note that `/api/v0/version` is always permitted access to allow version check
to ensure compatibility.

Default: `[]`

Type: `array[string]`

## `AutoNAT`

Contains the configuration options for the libp2p's [AutoNAT](https://github.com/libp2p/specs/tree/master/autonat) service. The AutoNAT service
helps other nodes on the network determine if they're publicly reachable from
the rest of the internet.

### `AutoNAT.ServiceMode`

When unset (default), the AutoNAT service defaults to _enabled_. Otherwise, this
field can take one of two values:

- `enabled` - Enable the V1+V2 service (unless the node determines that it,
  itself, isn't reachable by the public internet).
- `legacy-v1` - **DEPRECATED** Same as `enabled` but only V1 service is enabled. Used for testing
  during as few releases as we [transition to V2](https://github.com/ipfs/kubo/issues/10091), will be removed in the future.
- `disabled` - Disable the service.

Additional modes may be added in the future.

> [!IMPORTANT]
> We are in the progress of [rolling out AutoNAT V2](https://github.com/ipfs/kubo/issues/10091).
> Right now, by default, a publicly dialable Kubo provides both V1 and V2 service to other peers,
> and V1 is still used by Kubo for Autorelay feature. In a future release we will remove V1 and switch all features to use V2.

Default: `enabled`

Type: `optionalString`

### `AutoNAT.Throttle`

When set, this option configures the AutoNAT services throttling behavior. By
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

## `AutoConf`

The AutoConf feature enables Kubo nodes to automatically fetch and apply network configuration from a remote JSON endpoint. This system allows dynamic configuration updates for bootstrap peers, DNS resolvers, delegated routing, and IPNS publishing endpoints without requiring manual updates to each node's local config.

AutoConf works by using special `"auto"` placeholder values in configuration fields. When Kubo encounters these placeholders, it fetches the latest configuration from the specified URL and resolves the placeholders with the appropriate values at runtime. The original configuration file remains unchanged - `"auto"` values are preserved in the JSON and only resolved in memory during node operation.

### Key Features

- **Remote Configuration**: Fetch network defaults from a trusted URL
- **Automatic Updates**: Periodic background checks for configuration updates
- **Graceful Fallback**: Uses hardcoded IPFS Mainnet bootstrappers when remote config is unavailable
- **Validation**: Ensures all fetched configuration values are valid multiaddrs and URLs
- **Caching**: Stores multiple versions locally with ETags for efficient updates
- **User Notification**: Logs ERROR when new configuration is available requiring node restart
- **Debug Logging**: AutoConf operations can be inspected by setting `GOLOG_LOG_LEVEL="error,autoconf=debug"`

### Supported Fields

AutoConf can resolve `"auto"` placeholders in the following configuration fields:

- `Bootstrap` - Bootstrap peer addresses
- `DNS.Resolvers` - DNS-over-HTTPS resolver endpoints
- `Routing.DelegatedRouters` - Delegated routing HTTP API endpoints
- `Ipns.DelegatedPublishers` - IPNS delegated publishing HTTP API endpoints

### Usage Example

```json
{
  "AutoConf": {
    "URL": "https://example.com/autoconf.json",
    "Enabled": true,
    "RefreshInterval": "24h"
  },
  "Bootstrap": ["auto"],
  "DNS": {
    "Resolvers": {
      ".": ["auto"],
      "eth.": ["auto"],
      "custom.": ["https://dns.example.com/dns-query"]
    }
  },
  "Routing": {
    "DelegatedRouters": ["auto", "https://router.example.org/routing/v1"]
  }
}
```

**Notes:**

- Configuration fetching happens at daemon startup and periodically in the background
- When new configuration is detected, users must restart their node to apply changes
- Mixed configurations are supported: you can use both `"auto"` and static values
- If AutoConf is disabled but `"auto"` values exist, daemon startup will fail with validation errors
- Cache is stored in `$IPFS_PATH/autoconf/` with up to 3 versions retained

### Path-Based Routing Configuration

AutoConf supports path-based routing URLs that automatically enable specific routing operations based on the URL path. This allows precise control over which HTTP Routing V1 endpoints are used for different operations:

**Supported paths:**

- `/routing/v1/providers` - Enables provider record lookups only
- `/routing/v1/peers` - Enables peer routing lookups only  
- `/routing/v1/ipns` - Enables IPNS record operations only
- No path - Enables all routing operations (backward compatibility)

**AutoConf JSON structure with path-based routing:**

```json
{
  "DelegatedRouters": {
    "mainnet-for-nodes-with-dht": [
      "https://cid.contact/routing/v1/providers"
    ],
    "mainnet-for-nodes-without-dht": [
      "https://delegated-ipfs.dev/routing/v1/providers",
      "https://delegated-ipfs.dev/routing/v1/peers", 
      "https://delegated-ipfs.dev/routing/v1/ipns"
    ]
  },
  "DelegatedPublishers": {
    "mainnet-for-ipns-publishers-with-http": [
      "https://delegated-ipfs.dev/routing/v1/ipns"
    ]
  }
}
```

**Node type categories:**

- `mainnet-for-nodes-with-dht`: Mainnet nodes with DHT enabled (typically only need additional provider lookups)
- `mainnet-for-nodes-without-dht`: Mainnet nodes without DHT (need comprehensive routing services)
- `mainnet-for-ipns-publishers-with-http`: Mainnet nodes that publish IPNS records via HTTP

This design enables efficient, selective routing where each endpoint URL automatically determines its capabilities based on the path, while maintaining semantic grouping by node configuration type.

Default: `{}`

Type: `object`

### `AutoConf.Enabled`

Controls whether the AutoConf system is active. When enabled, Kubo will fetch configuration from the specified URL and resolve `"auto"` placeholders at runtime. When disabled, any `"auto"` values in the configuration will cause daemon startup to fail with validation errors.

This provides a safety mechanism to ensure nodes don't start with unresolved placeholders when AutoConf is intentionally disabled.

Default: `true`

Type: `flag`

### `AutoConf.URL`

Specifies the HTTP(S) URL from which to fetch the autoconf JSON. The endpoint should return a JSON document containing Bootstrap peers, DNS resolvers, delegated routing endpoints, and IPNS publishing endpoints that will replace `"auto"` placeholders in the local configuration.

The URL must serve a JSON document matching the AutoConf schema. Kubo validates all multiaddr and URL values before caching to ensure they are properly formatted.

When not specified in the configuration, the default mainnet URL is used automatically.

<a href="https://ipshipyard.com/"><img align="right" src="https://github.com/user-attachments/assets/39ed3504-bb71-47f6-9bf8-cb9a1698f272" /></a>

> [!NOTE]
> Public good autoconf manifest at `conf.ipfs-mainnet.org` is provided by the team at [Shipyard](https://ipshipyard.com).

Default: `"https://conf.ipfs-mainnet.org/autoconf.json"` (when not specified)

Type: `optionalString`

### `AutoConf.RefreshInterval`

Specifies how frequently Kubo should refresh autoconf data. This controls both how often cached autoconf data is considered fresh and how frequently the background service checks for new configuration updates.

When a new configuration version is detected during background updates, Kubo logs an ERROR message informing the user that a node restart is required to apply the changes to any `"auto"` entries in their configuration.

Default: `24h`

Type: `optionalDuration`

### `AutoConf.TLSInsecureSkipVerify`

**FOR TESTING ONLY** - Allows skipping TLS certificate verification when fetching autoconf from HTTPS URLs. This should never be enabled in production as it makes the configuration fetching vulnerable to man-in-the-middle attacks.

Default: `false`

Type: `flag`

## `AutoTLS`

The [AutoTLS](https://blog.libp2p.io/autotls/) feature enables publicly reachable Kubo nodes (those dialable from the public
internet) to automatically obtain a wildcard TLS certificate for a DNS name
unique to their PeerID at `*.[PeerID].libp2p.direct`. This enables direct
libp2p connections and retrieval of IPFS content from browsers [Secure Context](https://developer.mozilla.org/en-US/docs/Web/Security/Secure_Contexts)
using transports such as [Secure WebSockets](https://github.com/libp2p/specs/blob/master/websockets/README.md),
without requiring user to do any manual domain registration and certificate configuration.

Under the hood, [p2p-forge] client uses public utility service at `libp2p.direct` as an [ACME DNS-01 Challenge](https://letsencrypt.org/docs/challenge-types/#dns-01-challenge)
broker enabling peer to obtain a wildcard TLS certificate tied to public key of their [PeerID](https://docs.libp2p.io/concepts/fundamentals/peers/#peer-id).

By default, the certificates are requested from Let's Encrypt. Origin and rationale for this project can be found in [community.letsencrypt.org discussion](https://community.letsencrypt.org/t/feedback-on-raising-certificates-per-registered-domain-to-enable-peer-to-peer-networking/223003).

<a href="https://ipshipyard.com/"><img align="right" src="https://github.com/user-attachments/assets/39ed3504-bb71-47f6-9bf8-cb9a1698f272" /></a>

> [!NOTE]
> Public good DNS and [p2p-forge] infrastructure at `libp2p.direct` is run by the team at [Interplanetary Shipyard](https://ipshipyard.com).
>
[p2p-forge]: https://github.com/ipshipyard/p2p-forge

Default: `{}`

Type: `object`

### `AutoTLS.Enabled`

Enables the AutoTLS feature to provide DNS and TLS support for [libp2p Secure WebSocket](https://github.com/libp2p/specs/blob/master/websockets/README.md) over a `/tcp` port,
to allow JS clients running in web browser [Secure Context](https://w3c.github.io/webappsec-secure-contexts/) to connect to Kubo directly.

When activated, together with [`AutoTLS.AutoWSS`](#autotlsautowss) (default) or manually including a `/tcp/{port}/tls/sni/*.libp2p.direct/ws` multiaddr in [`Addresses.Swarm`](#addressesswarm)
(with SNI suffix matching [`AutoTLS.DomainSuffix`](#autotlsdomainsuffix)), Kubo retrieves a trusted PKI TLS certificate for `*.{peerid}.libp2p.direct` and configures the `/ws` listener to use it.

**Note:**

- This feature requires a publicly reachable node. If behind NAT, manual port forwarding or UPnP (`Swarm.DisableNatPortMap=false`) is required.
- The first time AutoTLS is used, it may take 5-15 minutes + [`AutoTLS.RegistrationDelay`](#autotlsregistrationdelay) before `/ws` listener is added. Be patient.
- Avoid manual configuration. [`AutoTLS.AutoWSS=true`](#autotlsautowss) should automatically add `/ws` listener to existing, firewall-forwarded `/tcp` ports.
- To troubleshoot, use `GOLOG_LOG_LEVEL="error,autotls=debug` for detailed logs, or `GOLOG_LOG_LEVEL="error,autotls=info` for quieter output.
- Certificates are stored in `$IPFS_PATH/p2p-forge-certs`; deleting this directory and restarting the daemon forces a certificate rotation.
- For now, the TLS cert applies solely to `/ws` libp2p WebSocket connections, not HTTP [`Gateway`](#gateway), which still need separate reverse proxy TLS setup with a custom domain.

Default: `true`

Type: `flag`

### `AutoTLS.AutoWSS`

Optional. Controls if Kubo should add `/tls/sni/*.libp2p.direct/ws` listener to every pre-existing `/tcp` port IFF no explicit `/ws` is defined in [`Addresses.Swarm`](#addressesswarm) already.

Default: `true` (if `AutoTLS.Enabled`)

Type: `flag`

### `AutoTLS.ShortAddrs`

Optional. Controls if final AutoTLS listeners are announced under shorter `/dnsX/A.B.C.D.peerid.libp2p.direct/tcp/4001/tls/ws` addresses instead of fully resolved `/ip4/A.B.C.D/tcp/4001/tls/sni/A-B-C-D.peerid.libp2p.direct/tls/ws`.

The main use for AutoTLS is allowing connectivity from Secure Context in a web browser, and DNS lookup needs to happen there anyway, making `/dnsX` a more compact, more interoperable option without obvious downside.

Default: `true`

Type: `flag`

### `AutoTLS.DomainSuffix`

Optional override of the parent domain suffix that will be used in DNS+TLS+WebSockets multiaddrs generated by [p2p-forge] client.
Do not change this unless you self-host [p2p-forge].

Default: `libp2p.direct` (public good run by [Interplanetary Shipyard](https://ipshipyard.com))

Type: `optionalString`

### `AutoTLS.RegistrationEndpoint`

Optional override of [p2p-forge] HTTP registration API.
Do not change this unless you self-host [p2p-forge] under own domain.

> [!IMPORTANT]
> The default endpoint performs [libp2p Peer ID Authentication over HTTP](https://github.com/libp2p/specs/blob/master/http/peer-id-auth.md)
> (proving ownership of PeerID), probes if your Kubo node can correctly answer to a [libp2p Identify](https://github.com/libp2p/specs/tree/master/identify) query.
> This ensures only a correctly configured, publicly dialable Kubo can initiate [ACME DNS-01 challenge](https://letsencrypt.org/docs/challenge-types/#dns-01-challenge) for `peerid.libp2p.direct`.

Default: `https://registration.libp2p.direct` (public good run by [Interplanetary Shipyard](https://ipshipyard.com))

Type: `optionalString`

### `AutoTLS.RegistrationToken`

Optional value for `Forge-Authorization` token sent with request to `RegistrationEndpoint`
(useful for private/self-hosted/test instances of [p2p-forge], unset by default).

Default: `""`

Type: `optionalString`

### `AutoTLS.RegistrationDelay`

An additional delay applied before sending a request to the `RegistrationEndpoint`.

The default delay is bypassed if the user explicitly set `AutoTLS.Enabled=true` in the JSON configuration file.
This ensures that ephemeral nodes using the default configuration do not spam the`AutoTLS.CAEndpoint` with unnecessary ACME requests.

Default: `1h` (or `0` if explicit `AutoTLS.Enabled=true`)

Type: `optionalDuration`

### `AutoTLS.CAEndpoint`

Optional override of CA ACME API used by [p2p-forge] system.
Do not change this unless you self-host [p2p-forge] under own domain.

> [!IMPORTANT]
> CAA DNS record at `libp2p.direct` limits CA choice to Let's Encrypt. If you want to use a different CA, use your own domain.

Default: [certmagic.LetsEncryptProductionCA](https://pkg.go.dev/github.com/caddyserver/certmagic#pkg-constants) (see [community.letsencrypt.org discussion](https://community.letsencrypt.org/t/feedback-on-raising-certificates-per-registered-domain-to-enable-peer-to-peer-networking/223003))

Type: `optionalString`

## `Bitswap`

High level client and server configuration of the [Bitswap Protocol](https://specs.ipfs.tech/bitswap-protocol/) over libp2p.

For internal configuration see [`Internal.Bitswap`](#internalbitswap).

For HTTP version see [`HTTPRetrieval`](#httpretrieval).

### `Bitswap.Libp2pEnabled`

Determines whether Kubo will use Bitswap over libp2p.

Disabling this, will remove `/ipfs/bitswap/*` protocol support from [libp2p identify](https://github.com/libp2p/specs/blob/master/identify/README.md) responses, effectively shutting down both Bitswap libp2p client and server.

> [!WARNING]
> Bitswap over libp2p is a core component of Kubo and the oldest way of exchanging blocks. Disabling it completely may cause unpredictable outcomes, such as retrieval failures, if the only providers were libp2p ones. Treat this as experimental and use it solely for testing purposes with `HTTPRetrieval.Enabled`.

Default: `true`

Type: `flag`

### `Bitswap.ServerEnabled`

Determines whether Kubo functions as a Bitswap server to host and respond to block requests.

Disabling the server retains client and protocol support in [libp2p identify](https://github.com/libp2p/specs/blob/master/identify/README.md) responses but causes Kubo to reply with "don't have" to all block requests.

Default: `true` (requires `Bitswap.Libp2pEnabled`)

Type: `flag`

## `Bootstrap`

Bootstrap peers help your node discover and connect to the IPFS network when starting up. This array contains [multiaddrs][multiaddr] of trusted nodes that your node contacts first to find other peers and content.

The special value `"auto"` automatically uses curated, up-to-date bootstrap peers from [AutoConf](#autoconf), ensuring your node can always connect to the healthy network without manual maintenance.

**What this gives you:**

- **Reliable startup**: Your node can always find the network, even if some bootstrap peers go offline
- **Automatic updates**: New bootstrap peers are added as the network evolves
- **Custom control**: Add your own trusted peers alongside or instead of the defaults

Default: `["auto"]`

Type: `array[string]` ([multiaddrs][multiaddr] or `"auto"`)

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

Enabling the BloomFilter can provide performance improvements specially when
responding to many requests for inexistent blocks. It however requires a full
sweep of all the datastore keys on daemon start. On very large datastores this
can be a very taxing operation, particularly if the datastore does not support
querying existing keys without reading their values at the same time (blocks).

Default: `0` (disabled)

Type: `integer` (non-negative, bytes)

### `Datastore.WriteThrough`

This option controls whether a block that already exist in the datastore
should be written to it. When set to `false`, a `Has()` call is performed
against the datastore prior to writing every block. If the block is already
stored, the write is skipped. This check happens both on the Blockservice and
the Blockstore layers and this setting affects both.

When set to `true`, no checks are performed and blocks are written to the
datastore, which depending on the implementation may perform its own checks.

This option can affect performance and the strategy should be taken in
conjunction with [`BlockKeyCacheSize`](#datastoreblockkeycachesize) and
[`BloomFilterSize`](#datastoreboomfiltersize`).

Default: `true`

Type: `bool`

### `Datastore.BlockKeyCacheSize`

A number representing the maximum size in bytes of the blockstore's Two-Queue
cache, which caches block-cids and their block-sizes. Use `0` to disable.

This cache, once primed, can greatly speed up operations like `ipfs repo stat`
as there is no need to read full blocks to know their sizes. Size should be
adjusted depending on the number of CIDs on disk (`NumObjects in`ipfs repo stat`).

Default: `65536` (64KiB)

Type: `optionalInteger` (non-negative, bytes)

### `Datastore.Spec`

Spec defines the structure of the ipfs datastore. It is a composable structure,
where each datastore is represented by a json object. Datastores can wrap other
datastores to provide extra functionality (eg metrics, logging, or caching).

> [!NOTE]
> For more information on possible values for this configuration option, see [`kubo/docs/datastores.md`](datastores.md)

Default:

```
{
  "mounts": [
  {
    "mountpoint": "/blocks",
    "path": "blocks",
    "prefix": "flatfs.datastore",
    "shardFunc": "/repo/flatfs/shard/v1/next-to-last/2",
    "sync": false,
    "type": "flatfs"
  },
  {
    "compression": "none",
    "mountpoint": "/",
    "path": "datastore",
    "prefix": "leveldb.datastore",
    "type": "levelds"
  }
  ],
  "type": "mount"
}
```

With `flatfs-measure` profile:

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

A boolean value to activate or deactivate Multicast DNS-SD.

Default: `true`

Type: `bool`

#### `Discovery.MDNS.Interval`

**REMOVED:**  this is not configurable anymore
in the [new mDNS implementation](https://github.com/libp2p/zeroconf#readme).

## `Experimental`

Toggle and configure experimental features of Kubo. Experimental features are listed [here](./experimental-features.md).

## `Gateway`

Options for the HTTP gateway.

**NOTE:** support for `/api/v0` under the gateway path is now deprecated. It will be removed in future versions: <https://github.com/ipfs/kubo/issues/10312>.

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

### `Gateway.DeserializedResponses`

An optional flag to explicitly configure whether this gateway responds to deserialized
requests, or not. By default, it is enabled. When disabling this option, the gateway
operates as a Trustless Gateway only: <https://specs.ipfs.tech/http-gateways/trustless-gateway/>.

Default: `true`

Type: `flag`

### `Gateway.DisableHTMLErrors`

An optional flag to disable the pretty HTML error pages of the gateway. Instead,
a `text/plain` page will be returned with the raw error message from Kubo.

It is useful for whitelabel or middleware deployments that wish to avoid
`text/html` responses with IPFS branding and links on error pages in browsers.

Default: `false`

Type: `flag`

### `Gateway.ExposeRoutingAPI`

An optional flag to expose Kubo `Routing` system on the gateway port
as an [HTTP `/routing/v1`](https://specs.ipfs.tech/routing/http-routing-v1/) endpoint on `127.0.0.1`.
Use reverse proxy to expose it on a different hostname.

This endpoint can be used by other Kubo instances, as illustrated in
[`delegated_routing_v1_http_proxy_test.go`](https://github.com/ipfs/kubo/blob/master/test/cli/delegated_routing_v1_http_proxy_test.go).
Kubo will filter out routing results which are not actionable, for example, all
graphsync providers will be skipped. If you need a generic pass-through, see
standalone router implementation named [someguy](https://github.com/ipfs/someguy).

Default: `false`

Type: `flag`

### `Gateway.RetrievalTimeout`

Maximum duration Kubo will wait for content retrieval (new bytes to arrive).

**Timeout behavior:**

- **Time to first byte**: Returns 504 Gateway Timeout if the gateway cannot start writing within this duration (e.g., stuck searching for providers)
- **Time between writes**: After first byte, timeout resets with each write. Response terminates if no new data can be written within this duration

**Truncation handling:** When timeout occurs after HTTP 200 headers are sent (e.g., during CAR streams), the gateway:

- Appends error message to indicate truncation
- Forces TCP reset (RST) to prevent caching incomplete responses
- Records in metrics with original status code and `truncated=true` flag

**Monitoring:** Track `ipfs_http_gw_retrieval_timeouts_total` by status code and truncation status.

**Tuning guidance:**

- Compare timeout rates (`ipfs_http_gw_retrieval_timeouts_total`) with success rates (`ipfs_http_gw_responses_total{status="200"}`)
- High timeout rate: consider increasing timeout or scaling horizontally if hardware is constrained
- Many 504s may indicate routing problems - check requested CIDs and provider availability using <https://check.ipfs.network/>
- `truncated=true` timeouts indicate retrieval stalled mid-file with no new bytes for the timeout duration

A value of 0 disables this timeout.

Default: `30s`

Type: `optionalDuration`

### `Gateway.MaxConcurrentRequests`

Limits concurrent HTTP requests. Requests beyond limit receive 429 Too Many Requests.

Protects nodes from traffic spikes and resource exhaustion, especially behind reverse proxies without rate-limiting. Default (4096) aligns with common reverse proxy configurations (e.g., nginx: 8 workers × 1024 connections).

**Monitoring:** `ipfs_http_gw_concurrent_requests` tracks current requests in flight.

**Tuning guidance:**

- Monitor `ipfs_http_gw_concurrent_requests` gauge for usage patterns
- Track 429s (`ipfs_http_gw_responses_total{status="429"}`) and success rate (`{status="200"}`)
- Near limit with low resource usage → increase value
- Memory pressure or OOMs → decrease value and consider scaling
- Set slightly below reverse proxy limit for graceful degradation
- Start with default, adjust based on observed performance for your hardware

A value of 0 disables the limit.

Default: `4096`

Type: `optionalInteger`

### `Gateway.HTTPHeaders`

Headers to set on gateway responses.

Default: `{}` + implicit CORS headers from `boxo/gateway#AddAccessControlHeaders` and [ipfs/specs#423](https://github.com/ipfs/specs/issues/423)

Type: `object[string -> array[string]]`

### `Gateway.RootRedirect`

A URL to redirect requests for `/` to.

Default: `""`

Type: `string` (url)

### `Gateway.DiagnosticServiceURL`

URL for a service to diagnose CID retrievability issues. When the gateway returns a 504 Gateway Timeout error, an "Inspect retrievability of CID" button will be shown that links to this service with the CID appended as `?cid=<CID-to-diagnose>`.

Set to empty string to disable the button.

Default: `"https://check.ipfs.network"`

Type: `optionalstring` (url)

### `Gateway.FastDirIndexThreshold`

**REMOVED**: this option is [no longer necessary](https://github.com/ipfs/kubo/pull/9481). Ignored since  [Kubo 0.18](https://github.com/ipfs/kubo/blob/master/docs/changelogs/v0.18.md).

### `Gateway.Writable`

**REMOVED**: this option no longer available as of [Kubo 0.20](https://github.com/ipfs/kubo/blob/master/docs/changelogs/v0.20.md).

We are working on developing a modern replacement. To support our efforts, please leave a comment describing your use case in [ipfs/specs#375](https://github.com/ipfs/specs/issues/375).

### `Gateway.PathPrefixes`

**REMOVED:** see [go-ipfs#7702](https://github.com/ipfs/go-ipfs/issues/7702)

### `Gateway.PublicGateways`

> [!IMPORTANT]
> This configuration is **NOT** for HTTP Client, it is for HTTP Server – use this ONLY if you want to run your own IPFS gateway.

`PublicGateways` is a configuration map used for dictionary for customizing gateway behavior
on specified hostnames that point at your Kubo instance.

It is useful when you want to run [Path gateway](https://specs.ipfs.tech/http-gateways/path-gateway/) on `example.com/ipfs/cid`,
and [Subdomain gateway](https://specs.ipfs.tech/http-gateways/subdomain-gateway/) on `cid.ipfs.example.org`,
or limit `verifiable.example.net` to response types defined in [Trustless Gateway](https://specs.ipfs.tech/http-gateways/trustless-gateway/) specification.

> [!CAUTION]
> Keys (Hostnames) MUST be unique. Do not use the same parent domain for multiple gateway types, it will break origin isolation.

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
        "Paths": ["/ipfs"],
      }
    }
  }
}
```

Above enables `http://example.com/ipfs/*` but not `http://example.com/ipns/*`

Default: `[]`

Type: `array[string]`

#### `Gateway.PublicGateways: UseSubdomains`

A boolean to configure whether the gateway at the hostname should be
a [Subdomain Gateway](https://specs.ipfs.tech/http-gateways/subdomain-gateway/)
and provide [Origin isolation](https://developer.mozilla.org/en-US/docs/Web/Security/Same-origin_policy)
between content roots.

- `true` - enables [subdomain gateway](https://docs.ipfs.tech/how-to/address-ipfs-on-web/#subdomain-gateway) at `http://*.{hostname}/`
  - **Requires whitelist:** make sure respective `Paths` are set.
      For example, `Paths: ["/ipfs", "/ipns"]` are required for `http://{cid}.ipfs.{hostname}` and `http://{foo}.ipns.{hostname}` to work:

        ```json
        "Gateway": {
            "PublicGateways": {
                "dweb.link": {
                    "UseSubdomains": true,
                    "Paths": ["/ipfs", "/ipns"]
                }
            }
        }
        ```

  - **Backward-compatible:** requests for content paths such as `http://{hostname}/ipfs/{cid}` produce redirect to `http://{cid}.ipfs.{hostname}`

- `false` - enables [path gateway](https://docs.ipfs.tech/how-to/address-ipfs-on-web/#path-gateway) at `http://{hostname}/*`
  - Example:

    ```json
    "Gateway": {
        "PublicGateways": {
            "ipfs.io": {
                "UseSubdomains": false,
                "Paths": ["/ipfs", "/ipns"]
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

#### `Gateway.PublicGateways: InlineDNSLink`

An optional flag to explicitly configure whether subdomain gateway's redirects
(enabled by `UseSubdomains: true`) should always inline a DNSLink name (FQDN)
into a single DNS label ([specification](https://specs.ipfs.tech/http-gateways/subdomain-gateway/#host-request-header)):

```
//example.com/ipns/example.net → HTTP 301 → //example-net.ipns.example.com
```

DNSLink name inlining allows for HTTPS on public subdomain gateways with single
label wildcard TLS certs (also enabled when passing `X-Forwarded-Proto: https`),
and provides disjoint Origin per root CID when special rules like
<https://publicsuffix.org>, or a custom localhost logic in browsers like Brave
has to be applied.

Default: `false`

Type: `flag`

#### `Gateway.PublicGateways: DeserializedResponses`

An optional flag to explicitly configure whether this gateway responds to deserialized
requests, or not. By default, it is enabled.

When disabled, the gateway operates strictly as a [Trustless Gateway](https://specs.ipfs.tech/http-gateways/trustless-gateway/).

> [!TIP]
> Disabling deserialized responses will protect you from acting as a free web hosting,
> while still allowing trustless clients like [@helia/verified-fetch](https://www.npmjs.com/package/@helia/verified-fetch)
> to utilize it for [trustless, verifiable data retrieval](https://docs.ipfs.tech/reference/http/gateway/#trustless-verifiable-retrieval).

Default: same as global `Gateway.DeserializedResponses`

Type: `flag`

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
ipfs config --json Gateway.PublicGateways '{"localhost": null }'
```

### `Gateway` recipes

Below is a list of the most common gateway setups.

- Public [subdomain gateway](https://docs.ipfs.tech/how-to/address-ipfs-on-web/#subdomain-gateway) at `http://{cid}.ipfs.dweb.link` (each content root gets its own Origin)

   ```console
   $ ipfs config --json Gateway.PublicGateways '{
       "dweb.link": {
         "UseSubdomains": true,
         "Paths": ["/ipfs", "/ipns"]
       }
     }'
   ```

  - **Performance:** Consider enabling `Routing.AcceleratedDHTClient=true` to improve content routing lookups. Separately, gateway operators should decide if the gateway node should also co-host and provide (announce) fetched content to the DHT. If providing content, enable `Provide.DHT.SweepEnabled=true` for efficient announcements. If announcements are still not fast enough, adjust `Provide.DHT.MaxWorkers`. For a read-only gateway that doesn't announce content, use `Provide.Enabled=false`.
  - **Backward-compatible:** this feature enables automatic redirects from content paths to subdomains:

     `http://dweb.link/ipfs/{cid}` → `http://{cid}.ipfs.dweb.link`

  - **X-Forwarded-Proto:** if you run Kubo behind a reverse proxy that provides TLS, make it add a `X-Forwarded-Proto: https` HTTP header to ensure users are redirected to `https://`, not `http://`. It will also ensure DNSLink names are inlined to fit in a single DNS label, so they work fine with a wildcard TLS cert ([details](https://github.com/ipfs/in-web-browsers/issues/169)). The NGINX directive is `proxy_set_header X-Forwarded-Proto "https";`.:

     `http://dweb.link/ipfs/{cid}` → `https://{cid}.ipfs.dweb.link`

     `http://dweb.link/ipns/your-dnslink.site.example.com` → `https://your--dnslink-site-example-com.ipfs.dweb.link`

  - **X-Forwarded-Host:** we also support `X-Forwarded-Host: example.com` if you want to override subdomain gateway host from the original request:

     `http://dweb.link/ipfs/{cid}` → `http://{cid}.ipfs.example.com`

- Public [path gateway](https://docs.ipfs.tech/how-to/address-ipfs-on-web/#path-gateway) at `http://ipfs.io/ipfs/{cid}` (no Origin separation)

   ```console
   $ ipfs config --json Gateway.PublicGateways '{
       "ipfs.io": {
         "UseSubdomains": false,
         "Paths": ["/ipfs", "/ipns"]
       }
     }'
   ```

  - **Performance:** Consider enabling `Routing.AcceleratedDHTClient=true` to improve content routing lookups. When running an open, recursive gateway, decide if the gateway should also co-host and provide (announce) fetched content to the DHT. If providing content, enable `Provide.DHT.SweepEnabled=true` for efficient announcements. If announcements are still not fast enough, adjust `Provide.DHT.MaxWorkers`. For a read-only gateway that doesn't announce content, use `Provide.Enabled=false`.

- Public [DNSLink](https://dnslink.io/) gateway resolving every hostname passed in `Host` header.

  ```console
  ipfs config --json Gateway.NoDNSLink false
  ```

  - Note that `NoDNSLink: false` is the default (it works out of the box unless set to `true` manually)

- Hardened, site-specific [DNSLink gateway](https://docs.ipfs.tech/how-to/address-ipfs-on-web/#dnslink-gateway).

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

> [!TIP]
> For high level configuration see [`Bitswap`](#bitswap).

The knobs (below) document how their value should related to each other.
Whether their values should be raised or lowered should be determined
based on the metrics `ipfs_bitswap_active_tasks`, `ipfs_bitswap_pending_tasks`,
`ipfs_bitswap_pending_block_tasks` and `ipfs_bitswap_active_block_tasks`
reported by bitswap.

These metrics can be accessed as the Prometheus endpoint at `{Addresses.API}/debug/metrics/prometheus` (default: `http://127.0.0.1:5001/debug/metrics/prometheus`)

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
If this adjustment still does not increase the throughput of the node, there might
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
This number controls fairness and can vary from 250Kb (very fair) to 10Mb (less fair, with more work
dedicated to peers who ask for more). Values below 250Kb could cause thrashing.
Values above 10Mb open the potential for aggressively-wanting peers to consume all resources and
deteriorate the quality provided to less aggressively-wanting peers.

Type: `optionalInteger` (byte count, `null` means default which is 1MB)

#### `Internal.Bitswap.ProviderSearchDelay`

This parameter determines how long to wait before looking for providers outside of bitswap.
Other routing systems like the Amino DHT are able to provide results in less than a second, so lowering
this number will allow faster peers lookups in some cases.

Type: `optionalDuration` (`null` means default which is 1s)

#### `Internal.Bitswap.ProviderSearchMaxResults`

Maximum number of providers bitswap client should aim at before it stops searching for new ones.
Setting to 0 means unlimited.

Type: `optionalInteger` (`null` means default which is 10)

#### `Internal.Bitswap.BroadcastControl`

`Internal.Bitswap.BroadcastControl` contains settings for the bitswap client's broadcast control functionality.

Broadcast control tries to reduce the number of bitswap broadcast messages sent to peers by choosing a subset of of the peers to send to. Peers are chosen based on whether they have previously responded indicating they have wanted blocks, as well as other configurable criteria. The settings here change how peers are selected as broadcast targets. Broadcast control can also be completely disabled to return bitswap to its previous behavior before broadcast control was introduced.

Enabling broadcast control should generally reduce the number of broadcasts significantly without significantly degrading the ability to discover which peers have wanted blocks. However, if block discovery on your network relies sufficiently on broadcasts to discover peers that have wanted blocks, then adjusting the broadcast control configuration or disabling it altogether, may be helpful.

##### `Internal.Bitswap.BroadcastControl.Enable`

Enables or disables broadcast control functionality. Setting this to `false` disables broadcast reduction logic and restores the previous (Kubo < 0.36) broadcast behavior of sending broadcasts to all peers. When disabled, all other `Bitswap.BroadcastControl` configuration items are ignored.

Default: `true` (Enabled)

Type: `flag`

##### `Internal.Bitswap.BroadcastControl.MaxPeers`

Sets a hard limit on the number of peers to send broadcasts to. A value of `0` means no broadcasts are sent. A value of `-1` means there is no limit.

Default: `0` (no limit)

Type: `optionalInteger` (non-negative, 0 means no limit)

##### `Internal.Bitswap.BroadcastControl.LocalPeers`

Enables or disables broadcast control for peers on the local network. Peers that have private or loopback addresses are considered to be on the local network. If this setting is `false`, than always broadcast to peers on the local network. If `true`, apply broadcast control to local peers.

Default: `false` (Always broadcast to peers on local network)

Type: `flag`

##### `Internal.Bitswap.BroadcastControl.PeeredPeers`

Enables or disables broadcast reduction for peers configured for peering. If `false`, than always broadcast to peers configured for peering. If `true`, apply broadcast reduction to peered peers.

Default: `false` (Always broadcast to peers configured for peering)

Type: `flag`

##### `Internal.Bitswap.BroadcastControl.MaxRandomPeers`

Sets the number of peers to broadcast to anyway, even though broadcast control logic has determined that they are not broadcast targets. Setting this to a non-zero value ensures at least this number of random peers receives a broadcast. This may be helpful in cases where peers that are not receiving broadcasts my have wanted blocks.

Default: `0` (do not send broadcasts to peers not already targeted broadcast control)

Type: `optionalInteger` (non-negative, 0 means do not broadcast to any random peers)

##### `Internal.Bitswap.BroadcastControl.SendToPendingPeers`

Enables or disables sending broadcasts to any peers to which there is a pending message to send. When enabled, this sends broadcasts to many more peers, but does so in a way that does not increase the number of separate broadcast messages. There is still the increased cost of the recipients having to process and respond to the broadcasts.

Default: `false` (Do not send broadcasts to all peers for which there are pending messages)

Type: `flag`

### `Internal.UnixFSShardingSizeThreshold`

**MOVED:** see [`Import.UnixFSHAMTDirectorySizeThreshold`](#importunixfshamtdirectorysizethreshold)

### `Internal.MFSNoFlushLimit`

Controls the maximum number of consecutive MFS operations allowed with `--flush=false`
before requiring a manual flush. This prevents unbounded memory growth and ensures
data consistency when using deferred flushing with `ipfs files` commands.

When the limit is reached, further operations will fail with an error message
instructing the user to run `ipfs files flush`, use `--flush=true`, or increase
this limit in the configuration.

**Why operations fail instead of auto-flushing:** Automatic flushing once the limit
is reached was considered but rejected because it can lead to data corruption issues
that are difficult to debug. When the system decides to flush without user knowledge, it can:
- Create partial states that violate user expectations about atomicity
- Interfere with concurrent operations in unexpected ways
- Make debugging and recovery much harder when issues occur

By failing explicitly, users maintain control over when their data is persisted,
allowing them to:
- Batch related operations together before flushing
- Handle errors predictably at natural transaction boundaries
- Understand exactly when and why their data is written to disk

If you expect automatic flushing behavior, simply use the default `--flush=true`
(or omit the flag entirely) instead of `--flush=false`.

**⚠️ WARNING:** Increasing this limit or disabling it (setting to 0) can lead to:
- **Out-of-memory errors (OOM)** - Each unflushed operation consumes memory
- **Data loss** - If the daemon crashes before flushing, all unflushed changes are lost
- **Degraded performance** - Large unflushed caches slow down MFS operations

Default: `256`

Type: `optionalInteger` (0 disables the limit, strongly discouraged)

**Note:** This is an EXPERIMENTAL feature and may change or be removed in future releases.
See [#10842](https://github.com/ipfs/kubo/issues/10842) for more information.

## `Ipns`

### `Ipns.RepublishPeriod`

A time duration specifying how frequently to republish ipns records to ensure
they stay fresh on the network.

Default: 4 hours.

Type: `interval` or an empty string for the default.

### `Ipns.RecordLifetime`

A time duration specifying the value to set on ipns records for their validity
lifetime.

Default: 48 hours.

Type: `interval` or an empty string for the default.

### `Ipns.ResolveCacheSize`

The number of entries to store in an LRU cache of resolved ipns entries. Entries
will be kept cached until their lifetime is expired.

Default: `128`

Type: `integer` (non-negative, 0 means the default)

### `Ipns.MaxCacheTTL`

Maximum duration for which entries are valid in the name system cache. Applied
to everything under `/ipns/` namespace, allows you to cap
the [Time-To-Live (TTL)](https://specs.ipfs.tech/ipns/ipns-record/#ttl-uint64) of
[IPNS Records](https://specs.ipfs.tech/ipns/ipns-record/)
AND also DNSLink TXT records (when DoH-specific [`DNS.MaxCacheTTL`](https://github.com/ipfs/kubo/blob/master/docs/config.md#dnsmaxcachettl)
is not set to a lower value).

When `Ipns.MaxCacheTTL` is set, it defines the upper bound limit of how long a
[IPNS Name](https://specs.ipfs.tech/ipns/ipns-record/#ipns-name) lookup result
will be cached and read from cache before checking for updates.

**Examples:**

- `"1m"` IPNS results are cached 1m or less (good compromise for system where
  faster updates are desired).
- `"0s"` IPNS caching is effectively turned off (useful for testing, bad for production use)
  - **Note:** setting this to `0` will turn off TTL-based caching entirely.
    This is discouraged in production environments. It will make IPNS websites
    artificially slow because IPNS resolution results will expire as soon as
    they are retrieved, forcing expensive IPNS lookup to happen on every
    request. If you want near-real-time IPNS, set it to a low, but still
    sensible value, such as `1m`.

Default: No upper bound, [TTL from IPNS Record](https://specs.ipfs.tech/ipns/ipns-record/#ttl-uint64)  (see `ipns name publish --help`) is always respected.

Type: `optionalDuration`

### `Ipns.UsePubsub`

Enables IPFS over pubsub experiment for publishing IPNS records in real time.

**EXPERIMENTAL:**  read about current limitations at [experimental-features.md#ipns-pubsub](./experimental-features.md#ipns-pubsub).

Default: `disabled`

Type: `flag`

### `Ipns.DelegatedPublishers`

HTTP endpoints for delegated IPNS publishing operations. These endpoints must support the [IPNS API](https://specs.ipfs.tech/routing/http-routing-v1/#ipns-api) from the Delegated Routing V1 HTTP specification.

The special value `"auto"` loads delegated publishers from [AutoConf](#autoconf) when enabled.

**Publishing behavior depends on routing configuration:**

- `Routing.Type=auto` (default): Uses DHT for publishing, `"auto"` resolves to empty list
- `Routing.Type=delegated`: Uses HTTP delegated publishers only, `"auto"` resolves to configured endpoints

When using `"auto"`, inspect the effective publishers with: `ipfs config Ipns.DelegatedPublishers --expand-auto`

**Command flags override publishing behavior:**

- `--allow-offline` - Publishes to local datastore without requiring network connectivity
- `--allow-delegated` - Uses local datastore and HTTP delegated publishers only (no DHT connectivity required)

For self-hosting, you can run your own `/routing/v1/ipns` endpoint using [someguy](https://github.com/ipfs/someguy/).

Default: `["auto"]`

Type: `array[string]` (URLs or `"auto"`)

## `Migration`

> [!WARNING]
> **DEPRECATED:** Only applies to legacy migrations (repo versions <16). Modern repos (v16+) use embedded migrations.
> This section is optional and will not appear in new configurations.

### `Migration.DownloadSources`

**DEPRECATED:** Download sources for legacy migrations. Only `"HTTPS"` is supported.

Type: `array[string]` (optional)

Default: `["HTTPS"]`

### `Migration.Keep`

**DEPRECATED:** Controls retention of legacy migration binaries. Options: `"cache"` (default), `"discard"`, `"keep"`.

Type: `string` (optional)

Default: `"cache"`

## `Mounts`

> [!CAUTION]
> **EXPERIMENTAL:**
> This feature is disabled by default, requires an explicit opt-in with  `ipfs mount` or `ipfs daemon --mount`.
>
> Read about current limitations at [fuse.md](./fuse.md).

FUSE mount point configuration options.

### `Mounts.IPFS`

Mountpoint for `/ipfs/`.

Default: `/ipfs`

Type: `string` (filesystem path)

### `Mounts.IPNS`

Mountpoint for `/ipns/`.

Default: `/ipns`

Type: `string` (filesystem path)

### `Mounts.MFS`

Mountpoint for Mutable File System (MFS) behind the `ipfs files` API.

> [!CAUTION]
>
> - Write support is highly experimental and not recommended for mission-critical deployments.
> - Avoid storing lazy-loaded datasets in MFS. Exposing a partially local, lazy-loaded DAG risks operating system search indexers crawling it, which may trigger unintended network prefetching of non-local DAG components.

Default: `/mfs`

Type: `string` (filesystem path)

### `Mounts.FuseAllowOther`

Sets the 'FUSE allow-other' option on the mount point.

## `Pinning`

Pinning configures the options available for pinning content
(i.e. keeping content longer-term instead of as temporarily cached storage).

### `Pinning.RemoteServices`

`RemoteServices` maps a name for a remote pinning service to its configuration.

A remote pinning service is a remote service that exposes an API for managing
that service's interest in long-term data storage.

The exposed API conforms to the specification defined at
<https://ipfs.github.io/pinning-services-api-spec/>

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

Example: "<https://pinningservice.tld:1234/my/api/path>"

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

## `Provide`

Configures CID announcements to the routing system, including both immediate
announcements for new content (provide) and periodic re-announcements
(reprovide) on systems that require it, like Amino DHT. While designed to support
multiple routing systems in the future, the current default configuration only supports providing to the Amino DHT.

### `Provide.Enabled`

Controls whether Kubo provide and reprovide systems are enabled.

> [!CAUTION]
> Disabling this will prevent other nodes from discovering your content.
> Your node will stop announcing data to the routing system, making it
> inaccessible unless peers connect to you directly.

Default: `true`

Type: `flag`

### `Provide.Strategy`

Tells the provide system what should be announced. Valid strategies are:

- `"all"` - announce all CIDs of stored blocks
- `"pinned"` - only announce recursively pinned CIDs (`ipfs pin add -r`, both roots and child blocks)
  - Order: root blocks of direct and recursive pins are announced first, then the child blocks of recursive pins
- `"roots"` - only announce the root block of explicitly pinned CIDs (`ipfs pin add`)
  - **⚠️  BE CAREFUL:** node with `roots` strategy will not announce child blocks.
    It makes sense only for use cases where the entire DAG is fetched in full,
    and a graceful resume does not have to be guaranteed: the lack of child
    announcements means an interrupted retrieval won't be able to find
    providers for the missing block in the middle of a file, unless the peer
    happens to already be connected to a provider and asks for child CID over
    bitswap.
- `"mfs"` - announce only the local CIDs that are part of the MFS (`ipfs files`)
  - Note: MFS is lazy-loaded. Only the MFS blocks present in local datastore are announced.
- `"pinned+mfs"` - a combination of the `pinned` and `mfs` strategies.
  - **ℹ️ NOTE:** This is the suggested strategy for users who run without GC and don't want to provide everything in cache.
  - Order: first `pinned` and then the locally available part of `mfs`.

**Strategy changes automatically clear the provide queue.** When you change `Provide.Strategy` and restart Kubo, the provide queue is automatically cleared to ensure only content matching your new strategy is announced. You can also manually clear the queue using `ipfs provide clear`.

**Memory requirements:**

- Reproviding larger pinsets using the `mfs`, `pinned`, `pinned+mfs` or `roots` strategies requires additional memory, with an estimated ~1 GiB of RAM per 20 million CIDs for reproviding to the Amino DHT.
- This is due to the use of a buffered provider, which loads all CIDs into memory to avoid holding a lock on the entire pinset during the reprovide cycle.

Default: `"all"`

Type: `optionalString` (unset for the default)

### `Provide.DHT`

Configuration for providing data to Amino DHT peers.

#### Monitoring Provide Operations

You can monitor the effectiveness of your provide configuration through metrics exposed at the Prometheus endpoint: `{Addresses.API}/debug/metrics/prometheus` (default: `http://127.0.0.1:5001/debug/metrics/prometheus`).

Different metrics are available depending on whether you use legacy mode (`SweepEnabled=false`) or sweep mode (`SweepEnabled=true`). See [Provide metrics documentation](https://github.com/ipfs/kubo/blob/master/docs/metrics.md#provide) for details.

To enable detailed debug logging for both providers, set:

```sh
GOLOG_LOG_LEVEL=error,provider=debug,dht/provider=debug
```

- `provider=debug` enables generic logging (legacy provider and any non-dht operations)
- `dht/provider=debug` enables logging for the sweep provider

#### `Provide.DHT.Interval`

Sets how often to re-announce content to the DHT. Provider records on Amino DHT
expire after [`amino.DefaultProvideValidity`](https://github.com/libp2p/go-libp2p-kad-dht/blob/v0.34.0/amino/defaults.go#L40-L43),
also known as Provider Record Expiration Interval.

An interval of about half the expiration window ensures provider records
are refreshed well before they expire. This keeps your content continuously
discoverable accounting for network churn without overwhelming the network with too frequent announcements.

- If unset, it uses the implicit safe default.
- If set to the value `"0"` it will disable content reproviding to DHT.

> [!CAUTION]
> Disabling this will prevent other nodes from discovering your content via the DHT.
> Your node will stop announcing data to the DHT, making it
> inaccessible unless peers connect to you directly. Since provider
> records expire after `amino.DefaultProvideValidity`, your content will become undiscoverable
> after this period.

Default: `22h`

Type: `optionalDuration` (unset for the default)

#### `Provide.DHT.MaxWorkers`

Sets the maximum number of _concurrent_ DHT provide operations.

**When `Provide.DHT.SweepEnabled` is false (legacy mode):**

- Controls NEW CID announcements only
- Reprovide operations do **not** count against this limit
- A value of `0` allows unlimited provide workers

**When `Provide.DHT.SweepEnabled` is true:**

- Controls the total worker pool for both provide and reprovide operations
- Workers are split between periodic reprovides and burst provides
- Use a positive value to control resource usage
- See [`DedicatedPeriodicWorkers`](#providedhtdedicatedperiodicworkers) and [`DedicatedBurstWorkers`](#providedhtdedicatedburstworkers) for task allocation

If the [accelerated DHT client](#routingaccelerateddhtclient) is enabled, each
provide operation opens ~20 connections in parallel. With the standard DHT
client (accelerated disabled), each provide opens between 20 and 60
connections, with at most 10 active at once. Provides complete more quickly
when using the accelerated client. Be mindful of how many simultaneous
connections this setting can generate.

> [!CAUTION]
> For nodes without strict connection limits that need to provide large volumes
> of content, we recommend first trying `Provide.DHT.SweepEnabled=true` for efficient
> announcements. If announcements are still not fast enough, adjust `Provide.DHT.MaxWorkers`.
> As a last resort, consider enabling `Routing.AcceleratedDHTClient=true` but be aware that it is very resource hungry.
>
> At the same time, mind that raising this value too high may lead to increased load.
> Proceed with caution, ensure proper hardware and networking are in place.

Default: `16`

Type: `optionalInteger` (non-negative; `0` means unlimited number of workers)

#### `Provide.DHT.SweepEnabled`

Whether Provide Sweep is enabled. If not enabled, the legacy
[`boxo/provider`](https://github.com/ipfs/boxo/tree/main/provider) is used for
both provides and reprovides.

Provide Sweep is a resource efficient technique for advertising content to
the Amino DHT swarm. The Provide Sweep module tracks the keys that should be periodically reprovided in
the `Keystore`. It splits the keys into DHT keyspace regions by proximity (XOR
distance), and schedules when reprovides should happen in order to spread the
reprovide operation over time to avoid a spike in resource utilization. It
basically sweeps the keyspace _from left to right_ over the
[`Provide.DHT.Interval`](#providedhtinterval) time period, and reprovides keys
matching to the visited keyspace region.

Provide Sweep aims at replacing the inefficient legacy `boxo/provider`
module, and is currently opt-in. You can compare the effectiveness of sweep mode vs legacy mode by monitoring the appropriate metrics (see [Monitoring Provide Operations](#monitoring-provide-operations) above).

Whenever new keys should be advertised to the Amino DHT, `kubo` calls
`StartProviding()`, triggering an initial `provide` operation for the given
keys. The keys will be added to the `Keystore` tracking which keys should be
reprovided and when they should be reprovided. Calling `StopProviding()`
removes the keys from the `Keystore`. However, it is currently tricky for
`kubo` to detect when a key should stop being advertised. Hence, `kubo` will
periodically refresh the `Keystore` at each [`Provide.DHT.Interval`](#providedhtinterval)
by providing it a channel of all the keys it is expected to contain according
to the [`Provide.Strategy`](#providestrategy). During this operation,
all keys in the `Keystore` are purged, and only the given ones remain scheduled.

> <picture>
>   <source media="(prefers-color-scheme: dark)" srcset="https://github.com/user-attachments/assets/f6e06b08-7fee-490c-a681-1bf440e16e27">
>   <source media="(prefers-color-scheme: light)" srcset="https://github.com/user-attachments/assets/e1662d7c-f1be-4275-a9ed-f2752fcdcabe">
>   <img alt="Reprovide Cycle Comparison" src="https://github.com/user-attachments/assets/e1662d7c-f1be-4275-a9ed-f2752fcdcabe">
> </picture>
>
> The diagram above visualizes the performance patterns:
>
> - **Legacy mode**: Individual (slow) provides per CID, can struggle with large datasets
> - **Sweep mode**: Even distribution matching the keyspace sweep described with low resource usage
> - **Accelerated DHT**: Hourly traffic spikes with high resource usage
>
> Sweep mode provides similar effectiveness to Accelerated DHT but with steady resource usage - better for machines with limited CPU, memory, or network bandwidth.

> [!NOTE]
> This feature is opt-in for now, but will become the default in a future release.
> Eventually, this configuration flag will be removed once the feature is stable.

Default: `false`

Type: `flag`

#### `Provide.DHT.DedicatedPeriodicWorkers`

Number of workers dedicated to periodic keyspace region reprovides. Only applies when `Provide.DHT.SweepEnabled` is true.

Among the [`Provide.DHT.MaxWorkers`](#providedhtmaxworkers), this
number of workers will be dedicated to the periodic region reprovide only. The sum of
`DedicatedPeriodicWorkers` and `DedicatedBurstWorkers` should not exceed `MaxWorkers`.
Any remaining workers (MaxWorkers - DedicatedPeriodicWorkers - DedicatedBurstWorkers)
form a shared pool that can be used for either type of work as needed.

Default: `2`

Type: `optionalInteger` (`0` means there are no dedicated workers, but the
operation can be performed by free non-dedicated workers)

#### `Provide.DHT.DedicatedBurstWorkers`

Number of workers dedicated to burst provides. Only applies when `Provide.DHT.SweepEnabled` is true.

Burst provides are triggered by:

- Manual provide commands (`ipfs routing provide`)
- New content matching your `Provide.Strategy` (blocks from `ipfs add`, bitswap, or trustless gateway requests)
- Catch-up reprovides after being disconnected/offline for a while

Having dedicated burst workers ensures that bulk operations (like adding many CIDs
or reconnecting to the network) don't delay regular periodic reprovides, and vice versa.

Among the [`Provide.DHT.MaxWorkers`](#providedhtmaxworkers), this
number of workers will be dedicated to burst provides only. In addition to
these, if there are available workers in the pool, they can also be used for
burst provides.

Default: `1`

Type: `optionalInteger` (`0` means there are no dedicated workers, but the
operation can be performed by free non-dedicated workers)

#### `Provide.DHT.MaxProvideConnsPerWorker`

Maximum number of connections that a single worker can use to send provider
records over the network.

When reproviding CIDs corresponding to a keyspace region, the reprovider must
send a provider record to the 20 closest peers to the CID (in XOR distance) for
each CID belonging to this keyspace region.

The reprovider opens a connection to a peer from that region, sends it all its
allocated provider records. Once done, it opens a connection to the next peer
from that keyspace region until all provider records are assigned.

This option defines how many such connections can be open concurrently by a
single worker.

Default: `16`

Type: `optionalInteger` (non-negative)

#### `Provide.DHT.KeystoreBatchSize`

During the garbage collection, all keys stored in the Keystore are removed, and
the keys are streamed from a channel to fill the Keystore again with up-to-date
keys. Since a high number of CIDs to reprovide can easily fill up the memory,
keys are read and written in batches to optimize for memory usage.

This option defines how many multihashes should be contained within a batch. A
multihash is usually represented by 34 bytes.

Default: `16384` (~544 KiB per batch)

Type: `optionalInteger` (non-negative)

#### `Provide.DHT.OfflineDelay`

The `SweepingProvider` has 3 states: `ONLINE`, `DISCONNECTED` and `OFFLINE`. It
starts `OFFLINE`, and as the node bootstraps, it changes its state to `ONLINE`.

When the provider loses connection to all DHT peers, it switches to the
`DISCONNECTED` state. In this state, new provides will be added to the provide
queue, and provided as soon as the node comes back online.

After a node has been `DISCONNECTED` for `OfflineDelay`, it goes to `OFFLINE`
state. When `OFFLINE`, the provider drops the provide queue, and returns errors
to new provide requests. However, when `OFFLINE` the provider still adds the
keys to its state, so keys will eventually be provided in the
[`Provide.DHT.Interval`](#providedhtinterval) after the provider comes back
`ONLINE`.

Default: `2h`

Type: `optionalDuration`

## `Provider`

### `Provider.Enabled`

**REMOVED**

Replaced with [`Provide.Enabled`](#provideenabled).

### `Provider.Strategy`

**REMOVED**

This field was unused. Use [`Provide.Strategy`](#providestrategy) instead.

### `Provider.WorkerCount`

**REMOVED**

Replaced with [`Provide.DHT.MaxWorkers`](#providedhtmaxworkers).

## `Pubsub`

**DEPRECATED**: See [#9717](https://github.com/ipfs/kubo/issues/9717)

Pubsub configures the `ipfs pubsub` subsystem. To use, it must be enabled by
passing the `--enable-pubsub-experiment` flag to the daemon
or via the `Pubsub.Enabled` flag below.

### `Pubsub.Enabled`

**DEPRECATED**: See [#9717](https://github.com/ipfs/kubo/issues/9717)

Enables the pubsub system.

Default: `false`

Type: `flag`

### `Pubsub.Router`

**DEPRECATED**: See [#9717](https://github.com/ipfs/kubo/issues/9717)

Sets the default router used by pubsub to route messages to peers. This can be one of:

- `"floodsub"` - floodsub is a basic router that simply _floods_ messages to all
  connected peers. This router is extremely inefficient but _very_ reliable.
- `"gossipsub"` - [gossipsub][] is a more advanced routing algorithm that will
  build an overlay mesh from a subset of the links in the network.

Default: `"gossipsub"`

Type: `string` (one of `"floodsub"`, `"gossipsub"`, or `""` (apply default))

[gossipsub]: https://github.com/libp2p/specs/tree/master/pubsub/gossipsub

### `Pubsub.DisableSigning`

**DEPRECATED**: See [#9717](https://github.com/ipfs/kubo/issues/9717)

Disables message signing and signature verification. Enable this option if
you're operating in a completely trusted network.

It is _not_ safe to disable signing even if you don't care _who_ sent the
message because spoofed messages can be used to silence real messages by
intentionally re-using the real message's message ID.

Default: `false`

Type: `bool`

### `Pubsub.SeenMessagesTTL`

**DEPRECATED**: See [#9717](https://github.com/ipfs/kubo/issues/9717)

Controls the time window within which duplicate messages, identified by Message
ID, will be identified and won't be emitted again.

A smaller value for this parameter means that Pubsub messages in the cache will
be garbage collected sooner, which can result in a smaller cache. At the same
time, if there are slower nodes in the network that forward older messages,
this can cause more duplicates to be propagated through the network.

Conversely, a larger value for this parameter means that Pubsub messages in the
cache will be garbage collected later, which can result in a larger cache for
the same traffic pattern. However, it is less likely that duplicates will be
propagated through the network.

Default: see `TimeCacheDuration` from [go-libp2p-pubsub](https://github.com/libp2p/go-libp2p-pubsub)

Type: `optionalDuration`

### `Pubsub.SeenMessagesStrategy`

**DEPRECATED**: See [#9717](https://github.com/ipfs/kubo/issues/9717)

Determines how the time-to-live (TTL) countdown for deduplicating Pubsub
messages is calculated.

The Pubsub seen messages cache is a LRU cache that keeps messages for up to a
specified time duration. After this duration has elapsed, expired messages will
be purged from the cache.

The `last-seen` cache is a sliding-window cache. Every time a message is seen
again with the SeenMessagesTTL duration, its timestamp slides forward. This
keeps frequently occurring messages cached and prevents them from being
continually propagated, especially because of issues that might increase the
number of duplicate messages in the network.

The `first-seen` cache will store new messages and purge them after the
SeenMessagesTTL duration, even if they are seen multiple times within this
duration.

Default: `last-seen` (see [go-libp2p-pubsub](https://github.com/libp2p/go-libp2p-pubsub))

Type: `optionalString`

## `Peering`

Configures the peering subsystem. The peering subsystem configures Kubo to
connect to, remain connected to, and reconnect to a set of nodes. Nodes should
use this subsystem to create "sticky" links between frequently useful peers to
improve reliability.

Use-cases:

- An IPFS gateway connected to an IPFS cluster should peer to ensure that the
  gateway can always fetch content from the cluster.
- A dapp may peer embedded Kubo nodes with a set of pinning services or
  textile cafes/hubs.
- A set of friends may peer to ensure that they can always fetch each other's
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

- When symmetric, the connection will be protected by both nodes and will likely
  be very stable.
- When asymmetric, only one node (the node that configured peering) will protect
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
        "Addrs": ["/ip4/18.1.1.2/tcp/4001", "/ip4/18.1.1.2/udp/4001/quic-v1"]
      }
    ]
  }
  ...
}
```

Where `ID` is the peer ID and `Addrs` is a set of known addresses for the peer. If no addresses are specified, the Amino DHT will be queried.

Additional fields may be added in the future.

Default: empty.

Type: `array[peering]`

## `Reprovider`

### `Reprovider.Interval`

**REMOVED**

Replaced with [`Provide.DHT.Interval`](#providedhtinterval).

### `Reprovider.Strategy`

**REMOVED**

Replaced with [`Provide.Strategy`](#providestrategy).

## `Routing`

Contains options for content, peer, and IPNS routing mechanisms.

### `Routing.Type`

There are multiple routing options: "auto", "autoclient", "none", "dht", "dhtclient", "delegated", and "custom".

- **DEFAULT:** If unset, or set to "auto", your node will use the public IPFS DHT (aka "Amino")
  and parallel [`Routing.DelegatedRouters`](#routingdelegatedrouters) for additional speed.

- If set to "autoclient", your node will behave as in "auto" but without running a DHT server.

- If set to "none", your node will use _no_ routing system. You'll have to
  explicitly connect to peers that have the content you're looking for.

- If set to "dht" (or "dhtclient"/"dhtserver"), your node will ONLY use the Amino DHT (no HTTP routers).

- If set to "custom", all default routers are disabled, and only ones defined in `Routing.Routers` will be used.

When the DHT is enabled, it can operate in two modes: client and server.

- In server mode, your node will query other peers for DHT records, and will
  respond to requests from other peers (both requests to store records and
  requests to retrieve records).

- In client mode, your node will query the DHT as a client but will not respond
  to requests from other peers. This mode is less resource-intensive than server
  mode.

When `Routing.Type` is set to `auto` or `dht`, your node will start as a DHT client, and
switch to a DHT server when and if it determines that it's reachable from the
public internet (e.g., it's not behind a firewall).

To force a specific Amino DHT-only mode, client or server, set `Routing.Type` to
`dhtclient` or `dhtserver` respectively. Please do not set this to `dhtserver`
unless you're sure your node is reachable from the public network.

When `Routing.Type` is set to `auto` or `autoclient` your node will accelerate some types of routing
by leveraging [`Routing.DelegatedRouters`](#routingdelegatedrouters) HTTP endpoints compatible with [Delegated Routing V1 HTTP API](https://specs.ipfs.tech/routing/http-routing-v1/)
introduced in [IPIP-337](https://github.com/ipfs/specs/pull/337)
in addition to the Amino DHT.

When `Routing.Type` is set to `delegated`, your node will use **only** HTTP delegated routers and IPNS publishers,
without initializing the Amino DHT at all. This mode is useful for environments where peer-to-peer DHT connectivity
is not available or desired, while still enabling content routing and IPNS publishing via HTTP APIs.
This mode requires configuring [`Routing.DelegatedRouters`](#routingdelegatedrouters) for content routing and
[`Ipns.DelegatedPublishers`](#ipnsdelegatedpublishers) for IPNS publishing.

**Note:** `delegated` mode operates as read-only for content providing - your node cannot announce content to the network
since there is no DHT connectivity. Content providing is automatically disabled when using this routing type.

[Advanced routing rules](https://github.com/ipfs/kubo/blob/master/docs/delegated-routing.md) can be configured in `Routing.Routers` after setting `Routing.Type` to `custom`.

Default: `auto` (DHT + [`Routing.DelegatedRouters`](#routingdelegatedrouters))

Type: `optionalString` (`null`/missing means the default)

### `Routing.AcceleratedDHTClient`

This alternative Amino DHT client with a Full-Routing-Table strategy will
do a complete scan of the DHT every hour and record all nodes found.
Then when a lookup is tried instead of having to go through multiple Kad hops it
is able to find the 20 final nodes by looking up the in-memory recorded network table.

This means sustained higher memory to store the routing table
and extra CPU and network bandwidth for each network scan.
However the latency of individual read/write operations should be ~10x faster
and provide throughput up to 6 million times faster on larger datasets!

This is not compatible with `Routing.Type` `custom`. If you are using composable routers
you can configure this individually on each router.

When it is enabled:

- Client DHT operations (reads and writes) should complete much faster
- The provider will now use a keyspace sweeping mode allowing to keep alive
  CID sets that are multiple orders of magnitude larger.
  - **Note:** For improved provide/reprovide operations specifically, consider using
    [`Provide.DHT.SweepEnabled`](#providedhtsweepenabled) instead, which offers similar
    benefits without the hourly traffic spikes.
  - The standard Bucket-Routing-Table DHT will still run for the DHT server (if
    the DHT server is enabled). This means the classical routing table will
    still be used to answer other nodes.
    This is critical to maintain to not harm the network.
- The operations `ipfs stats dht` will default to showing information about the accelerated DHT client

**Caveats:**

1. Running the accelerated client likely will result in more resource consumption (connections, RAM, CPU, bandwidth)
   - Users that are limited in the number of parallel connections their machines/networks can perform will likely suffer
   - The resource usage is not smooth as the client crawls the network in rounds and reproviding is similarly done in rounds
   - Users who previously had a lot of content but were unable to advertise it on the network will see an increase in
     egress bandwidth as their nodes start to advertise all of their CIDs into the network. If you have lots of data
     entering your node that you don't want to advertise, then consider using [Provide Strategies](#providestrategy)
     to reduce the number of CIDs that you are reproviding. Similarly, if you are running a node that deals mostly with
     short-lived temporary data (e.g. you use a separate node for ingesting data then for storing and serving it) then
     you may benefit from using [Strategic Providing](experimental-features.md#strategic-providing) to prevent advertising
     of data that you ultimately will not have.
2. Currently, the DHT is not usable for queries for the first 5-10 minutes of operation as the routing table is being
prepared. This means operations like searching the DHT for particular peers or content will not work initially.
   - You can see if the DHT has been initially populated by running `ipfs stats dht`
3. Currently, the accelerated DHT client is not compatible with LAN-based DHTs and will not perform operations against
them

Default: `false`

Type: `flag`

### `Routing.LoopbackAddressesOnLanDHT`

**EXPERIMENTAL: `Routing.LoopbackAddressesOnLanDHT` configuration may change in future release**

Whether loopback addresses (e.g. 127.0.0.1) should not be ignored on the local LAN DHT.

Most users do not need this setting. It can be useful during testing, when multiple Kubo nodes run on the same machine but some of them do not have `Discovery.MDNS.Enabled`.

Default: `false`

Type: `bool` (missing means `false`)

### `Routing.IgnoreProviders`

An array of [string-encoded PeerIDs](https://github.com/libp2p/specs/blob/master/peer-ids/peer-ids.md#string-representation). Any provider record associated to one of these peer IDs is ignored.

Apart from ignoring specific providers for reasons like misbehaviour etc. this
setting is useful to ignore providers as a way to indicate preference, when the same provider
is found under different peerIDs (i.e. one for HTTP and one for Bitswap retrieval).

> [!TIP]
> This denylist operates on PeerIDs.
> To deny specific HTTP Provider URL, use [`HTTPRetrieval.Denylist`](#httpretrievaldenylist) instead.

Default: `[]`

Type: `array[string]`

### `Routing.DelegatedRouters`

An array of URL hostnames for delegated routers to be queried in addition to the Amino DHT when `Routing.Type` is set to `auto` (default) or `autoclient`.
These endpoints must support the [Delegated Routing V1 HTTP API](https://specs.ipfs.tech/routing/http-routing-v1/).

The special value `"auto"` uses delegated routers from [AutoConf](#autoconf) when enabled.

> [!TIP]
> Delegated routing allows IPFS implementations to offload tasks like content routing, peer routing, and naming to a separate process or server while also benefiting from HTTP caching.
>
> One can run their own delegated router either by implementing the [Delegated Routing V1 HTTP API](https://specs.ipfs.tech/routing/http-routing-v1/) themselves, or by using [Someguy](https://github.com/ipfs/someguy), a turn-key implementation that proxies requests to other routing systems. A public utility instance of Someguy is hosted at [`https://delegated-ipfs.dev`](https://docs.ipfs.tech/concepts/public-utilities/#delegated-routing).

Default: `["auto"]`

Type: `array[string]` (URLs or `"auto"`)

### `Routing.Routers`

Alternative configuration used when `Routing.Type=custom`.

> [!WARNING]
> **EXPERIMENTAL: `Routing.Routers` configuration may change in future release**
>
> Consider this advanced low-level config: Most users can simply use `Routing.Type=auto` or `autoclient` and set up basic config in user-friendly [`Routing.DelegatedRouters`](https://github.com/ipfs/kubo/blob/master/docs/config.md#routingdelegatedrouters).

Allows for replacing the default routing (Amino DHT) with alternative Router
implementations.

The map key is a name of a Router, and the value is its configuration.

Default: `{}`

Type: `object[string->object]`

#### `Routing.Routers: Type`

**EXPERIMENTAL: `Routing.Routers` configuration may change in future release**

It specifies the routing type that will be created.

Currently supported types:

- `http` simple delegated routing based on HTTP protocol from [IPIP-337](https://github.com/ipfs/specs/pull/337)
- `dht` provides decentralized routing based on [libp2p's kad-dht](https://github.com/libp2p/specs/tree/master/kad-dht)
- `parallel` and `sequential`: Helpers that can be used to run several routers sequentially or in parallel.

Type: `string`

#### `Routing.Routers: Parameters`

**EXPERIMENTAL: `Routing.Routers` configuration may change in future release**

Parameters needed to create the specified router. Supported params per router type:

HTTP:

- `Endpoint` (mandatory): URL that will be used to connect to a specified router.
- `MaxProvideBatchSize`: This number determines the maximum amount of CIDs sent per batch. Servers might not accept more than 100 elements per batch. 100 elements by default.
- `MaxProvideConcurrency`: It determines the number of threads used when providing content. GOMAXPROCS by default.

DHT:

- `"Mode"`: Mode used by the Amino DHT. Possible values: "server", "client", "auto"
- `"AcceleratedDHTClient"`: Set to `true` if you want to use the acceleratedDHT.
- `"PublicIPNetwork"`: Set to `true` to create a `WAN` DHT. Set to `false` to create a `LAN` DHT.

Parallel:

- `Routers`: A list of routers that will be executed in parallel:
  - `Name:string`: Name of the router. It should be one of the previously added to `Routers` list.
  - `Timeout:duration`: Local timeout. It accepts strings compatible with Go `time.ParseDuration(string)` (`10s`, `1m`, `2h`). Time will start counting when this specific router is called, and it will stop when the router returns, or we reach the specified timeout.
  - `ExecuteAfter:duration`: Providing this param will delay the execution of that router at the specified time. It accepts strings compatible with Go `time.ParseDuration(string)` (`10s`, `1m`, `2h`).
  - `IgnoreErrors:bool`: It will specify if that router should be ignored if an error occurred.
- `Timeout:duration`: Global timeout.  It accepts strings compatible with Go `time.ParseDuration(string)` (`10s`, `1m`, `2h`).

Sequential:

- `Routers`: A list of routers that will be executed in order:
  - `Name:string`: Name of the router. It should be one of the previously added to `Routers` list.
  - `Timeout:duration`: Local timeout. It accepts strings compatible with Go `time.ParseDuration(string)`. Time will start counting when this specific router is called, and it will stop when the router returns, or we reach the specified timeout.
  - `IgnoreErrors:bool`: It will specify if that router should be ignored if an error occurred.
- `Timeout:duration`: Global timeout.  It accepts strings compatible with Go `time.ParseDuration(string)`.

Default: `{}` (use the safe implicit defaults)

Type: `object[string->string]`

### `Routing: Methods`

`Methods:map` will define which routers will be executed per method used when `Routing.Type=custom`.

> [!WARNING]
> **EXPERIMENTAL: `Routing.Routers` configuration may change in future release**
>
> Consider this advanced low-level config: Most users can simply use `Routing.Type=auto` or `autoclient` and set up basic config in user-friendly [`Routing.DelegatedRouters`](https://github.com/ipfs/kubo/blob/master/docs/config.md#routingdelegatedrouters).

The key will be the name of the method: `"provide"`, `"find-providers"`, `"find-peers"`, `"put-ipns"`, `"get-ipns"`. All methods must be added to the list.

The value will contain:

- `RouterName:string`: Name of the router. It should be one of the previously added to `Routing.Routers` list.

Type: `object[string->object]`

**Examples:**

Complete example using 2 Routers, Amino DHT (LAN/WAN) and parallel.

```
$ ipfs config Routing.Type --json '"custom"'

$ ipfs config Routing.Routers.WanDHT --json '{
  "Type": "dht",
  "Parameters": {
    "Mode": "auto",
    "PublicIPNetwork": true,
    "AcceleratedDHTClient": false
  }
}'

$ ipfs config Routing.Routers.LanDHT --json '{
  "Type": "dht",
  "Parameters": {
    "Mode": "auto",
    "PublicIPNetwork": false,
    "AcceleratedDHTClient": false
  }
}'

$ ipfs config Routing.Routers.ParallelHelper --json '{
  "Type": "parallel",
  "Parameters": {
    "Routers": [
        {
        "RouterName" : "LanDHT",
        "IgnoreErrors" : true,
        "Timeout": "3s"
        },
        {
        "RouterName" : "WanDHT",
        "IgnoreErrors" : false,
        "Timeout": "5m",
        "ExecuteAfter": "2s"
        }
    ]
  }
}'

ipfs config Routing.Methods --json '{
      "find-peers": {
        "RouterName": "ParallelHelper"
      },
      "find-providers": {
        "RouterName": "ParallelHelper"
      },
      "get-ipns": {
        "RouterName": "ParallelHelper"
      },
      "provide": {
        "RouterName": "ParallelHelper"
      },
      "put-ipns": {
        "RouterName": "ParallelHelper"
      }
    }'

```

## `Swarm`

Options for configuring the swarm.

### `Swarm.AddrFilters`

An array of addresses (multiaddr netmasks) to not dial. By default, IPFS nodes
advertise _all_ addresses, even internal ones. This makes it easier for nodes on
the same network to reach each other. Unfortunately, this means that an IPFS
node will try to connect to one or more private IP addresses whenever dialing
another node, even if this other node is on a different network. This may
trigger netscan alerts on some hosting providers or cause strain in some setups.

> [!TIP]
> The [`server` configuration profile](#server-profile) fills up this list with sensible defaults,
> preventing dials to all non-routable IP addresses (e.g., `/ip4/192.168.0.0/ipcidr/16`,
> which is the [multiaddress][multiaddr] representation of `192.168.0.0/16`) but you should always
> check settings against your own network and/or hosting provider.

Default: `[]`

Type: `array[string]`

### `Swarm.DisableBandwidthMetrics`

A boolean value that when set to true, will cause ipfs to not keep track of
bandwidth metrics. Disabling bandwidth metrics can lead to a slight performance
improvement, as well as a reduction in memory usage.

Default: `false`

Type: `bool`

### `Swarm.DisableNatPortMap`

Disable automatic NAT port forwarding (turn off [UPnP](https://en.wikipedia.org/wiki/Universal_Plug_and_Play)).

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

Your node will use these statically configured relay servers
instead of discovering public relays ([Circuit Relay v2](https://github.com/libp2p/specs/blob/master/relay/circuit-v2.md)) from the network.

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

Limits are applied to every relayed connection.

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

**REMOVED in kubo 0.32 due to [go-libp2p#2974](https://github.com/libp2p/go-libp2p/pull/2974)**

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

- none: never close idle connections.
- basic: the default connection manager.

By default, this section is empty and the implicit defaults defined below
are used.

#### `Swarm.ConnMgr.Type`

Sets the type of connection manager to use, options are: `"none"` (no connection
management) and `"basic"`.

Default: "basic".

Type: `optionalString` (default when unset or empty)

#### Basic Connection Manager

The basic connection manager uses a "high water", a "low water", and internal
scoring to periodically close connections to free up resources. When a node
using the basic connection manager reaches `HighWater` idle connections, it
will close the least useful ones until it reaches `LowWater` idle
connections. The process of closing connections happens every `SilencePeriod`.

The connection manager considers a connection idle if:

- It has not been explicitly _protected_ by some subsystem. For example, Bitswap
  will protect connections to peers from which it is actively downloading data,
  the DHT will protect some peers for routing, and the peering subsystem will
  protect all "peered" nodes.
- It has existed for longer than the `GracePeriod`.

**Example:**

```json
{
  "Swarm": {
    "ConnMgr": {
      "Type": "basic",
      "LowWater": 100,
      "HighWater": 200,
      "GracePeriod": "30s",
      "SilencePeriod": "10s"
    }
  }
}
```

##### `Swarm.ConnMgr.LowWater`

LowWater is the number of connections that the basic connection manager will
trim down to.

Default: `32`

Type: `optionalInteger`

##### `Swarm.ConnMgr.HighWater`

HighWater is the number of connections that, when exceeded, will trigger a
connection GC operation. Note: protected/recently formed connections don't count
towards this limit.

Default: `96`

Type: `optionalInteger`

##### `Swarm.ConnMgr.GracePeriod`

GracePeriod is a time duration that new connections are immune from being closed
by the connection manager.

Default: `"20s"`

Type: `optionalDuration`

##### `Swarm.ConnMgr.SilencePeriod`

SilencePeriod is the time duration between connection manager runs, when connections that are idle are closed.

Default: `"10s"`

Type: `optionalDuration`

### `Swarm.ResourceMgr`

Learn more about Kubo's usage of libp2p Network Resource Manager
in the [dedicated resource management docs](./libp2p-resource-management.md).

#### `Swarm.ResourceMgr.Enabled`

Enables the libp2p Resource Manager using limits based on the defaults and/or other configuration as discussed in [libp2p resource management](./libp2p-resource-management.md).

Default: `true`
Type: `flag`

#### `Swarm.ResourceMgr.MaxMemory`

This is the max amount of memory to allow go-libp2p to use.

libp2p's resource manager will prevent additional resource creation while this limit is reached.
This value is also used to scale the limit on various resources at various scopes
when the default limits (discussed in [libp2p resource management](./libp2p-resource-management.md)) are used.
For example, increasing this value will increase the default limit for incoming connections.

It is possible to inspect the runtime limits via `ipfs swarm resources --help`.

> [!IMPORTANT]
> `Swarm.ResourceMgr.MaxMemory` is the memory limit for go-libp2p networking stack alone, and not for entire Kubo or Bitswap.
>
> To set memory limit for the entire Kubo process, use [`GOMEMLIMIT` environment variable](http://web.archive.org/web/20240222201412/https://kupczynski.info/posts/go-container-aware/) which all Go programs recognize, and then set `Swarm.ResourceMgr.MaxMemory` to less than your custom `GOMEMLIMIT`.

Default: `[TOTAL_SYSTEM_MEMORY]/2`
Type: `optionalBytes`

#### `Swarm.ResourceMgr.MaxFileDescriptors`

This is the maximum number of file descriptors to allow libp2p to use.
libp2p's resource manager will prevent additional file descriptor consumption while this limit is reached.

This param is ignored on Windows.

Default `[TOTAL_SYSTEM_FILE_DESCRIPTORS]/2`
Type: `optionalInteger`

#### `Swarm.ResourceMgr.Allowlist`

A list of [multiaddrs][libp2p-multiaddrs] that can bypass normal system limits (but are still limited by the allowlist scope).
Convenience config around [go-libp2p-resource-manager#Allowlist.Add](https://pkg.go.dev/github.com/libp2p/go-libp2p/p2p/host/resource-manager#Allowlist.Add).

Default: `[]`

Type: `array[string]` ([multiaddrs][multiaddr])

### `Swarm.Transports`

Configuration section for libp2p transports. An empty configuration will apply
the defaults.

### `Swarm.Transports.Network`

Configuration section for libp2p _network_ transports. Transports enabled in
this section will be used for dialing. However, to receive connections on these
transports, multiaddrs for these transports must be added to `Addresses.Swarm`.

Supported transports are: QUIC, TCP, WS, Relay, WebTransport and WebRTCDirect.

> [!CAUTION]
> **SECURITY CONSIDERATIONS FOR NETWORK TRANSPORTS**
>
> Enabling network transports allows your node to accept connections from the internet.
> Ensure your firewall rules and [`Addresses.Swarm`](#addressesswarm) configuration
> align with your security requirements.
> See [Security section](#security) for network exposure considerations.

Each field in this section is a `flag`.

#### `Swarm.Transports.Network.TCP`

[TCP](https://en.wikipedia.org/wiki/Transmission_Control_Protocol) is a simple
and widely deployed transport, it should be compatible with most implementations
and network configurations.  TCP doesn't directly support encryption and/or
multiplexing, so libp2p will layer a security & multiplexing transport over it.

Default: Enabled

Type: `flag`

Listen Addresses:

- /ip4/0.0.0.0/tcp/4001 (default)
- /ip6/::/tcp/4001 (default)

#### `Swarm.Transports.Network.Websocket`

[Websocket](https://en.wikipedia.org/wiki/WebSocket) is a transport usually used
to connect to non-browser-based IPFS nodes from browser-based js-ipfs nodes.

While it's enabled by default for dialing, Kubo doesn't listen on this
transport by default.

Default: Enabled

Type: `flag`

Listen Addresses:

- /ip4/0.0.0.0/tcp/4001/ws
- /ip6/::/tcp/4001/ws

#### `Swarm.Transports.Network.QUIC`

[QUIC](https://en.wikipedia.org/wiki/QUIC) is the most widely used transport by
Kubo nodes. It is a UDP-based transport with built-in encryption and
multiplexing. The primary benefits over TCP are:

1. It takes 1 round trip to establish a connection (our TCP transport
   currently takes 4).
2. No [Head-of-Line blocking](https://en.wikipedia.org/wiki/Head-of-line_blocking).
3. It doesn't require a file descriptor per connection, easing the load on the OS.

Default: Enabled

Type: `flag`

Listen Addresses:

- `/ip4/0.0.0.0/udp/4001/quic-v1` (default)
- `/ip6/::/udp/4001/quic-v1` (default)

#### `Swarm.Transports.Network.Relay`

[Libp2p Relay](https://github.com/libp2p/specs/tree/master/relay) proxy
transport that forms connections by hopping between multiple libp2p nodes.
Allows IPFS node to connect to other peers using their `/p2p-circuit`
[multiaddrs][libp2p-multiaddrs].  This transport is primarily useful for bypassing firewalls and
NATs.

See also:

- Docs: [Libp2p Circuit Relay](https://docs.libp2p.io/concepts/circuit-relay/)
- [`Swarm.RelayClient.Enabled`](#swarmrelayclientenabled) for getting a public
- `/p2p-circuit` address when behind a firewall.
- [`Swarm.EnableHolePunching`](#swarmenableholepunching) for direct connection upgrade through relay
- [`Swarm.RelayService.Enabled`](#swarmrelayserviceenabled) for becoming a
  limited relay for other peers

Default: Enabled

Type: `flag`

Listen Addresses:

- This transport is special. Any node that enables this transport can receive
  inbound connections on this transport, without specifying a listen address.

#### `Swarm.Transports.Network.WebTransport`

A new feature of [`go-libp2p`](https://github.com/libp2p/go-libp2p/releases/tag/v0.23.0)
is the [WebTransport](https://github.com/libp2p/go-libp2p/issues/1717) transport.

This is a spiritual descendant of WebSocket but over `HTTP/3`.
Since this runs on top of `HTTP/3` it uses `QUIC` under the hood.
We expect it to perform worst than `QUIC` because of the extra overhead,
this transport is really meant at agents that cannot do `TCP` or `QUIC` (like browsers).

WebTransport is a new transport protocol currently under development by the IETF and the W3C, and already implemented by Chrome.
Conceptually, it’s like WebSocket run over QUIC instead of TCP. Most importantly, it allows browsers to establish (secure!) connections to WebTransport servers without the need for CA-signed certificates,
thereby enabling any js-libp2p node running in a browser to connect to any kubo node, with zero manual configuration involved.

The previous alternative is websocket secure, which require installing a reverse proxy and TLS certificates manually.

Default: Enabled

Type: `flag`

Listen Addresses:

- `/ip4/0.0.0.0/udp/4001/quic-v1/webtransport` (default)
- `/ip6/::/udp/4001/quic-v1/webtransport` (default)

#### `Swarm.Transports.Network.WebRTCDirect`

[WebRTC Direct](https://github.com/libp2p/specs/blob/master/webrtc/webrtc-direct.md)
is a transport protocol that provides another way for browsers to
connect to the rest of the libp2p network. WebRTC Direct allows for browser
nodes to connect to other nodes without special configuration, such as TLS
certificates. This can be useful for browser nodes that do not yet support
[WebTransport](https://blog.libp2p.io/2022-12-19-libp2p-webtransport/),
which is still relatively new and has [known issues](https://github.com/libp2p/js-libp2p/issues/2572).

Enabling this transport allows Kubo node to act on `/udp/4001/webrtc-direct`
listeners defined in `Addresses.Swarm`, `Addresses.Announce` or
`Addresses.AppendAnnounce`.

> [!NOTE]
> WebRTC Direct is browser-to-node. It cannot be used to connect a browser
> node to a node that is behind a NAT or firewall (without UPnP port mapping).
> The browser-to-private requires using normal
> [WebRTC](https://github.com/libp2p/specs/blob/master/webrtc/webrtc.md),
> which is currently being worked on in
> [go-libp2p#2009](https://github.com/libp2p/go-libp2p/issues/2009).

Default: Enabled

Type: `flag`

Listen Addresses:

- `/ip4/0.0.0.0/udp/4001/webrtc-direct` (default)
- `/ip6/::/udp/4001/webrtc-direct` (default)

### `Swarm.Transports.Security`

Configuration section for libp2p _security_ transports. Transports enabled in
this section will be used to secure unencrypted connections.

This does not concern all the QUIC transports which use QUIC's builtin encryption.

Security transports are configured with the `priority` type.

When establishing an _outbound_ connection, Kubo will try each security
transport in priority order (lower first), until it finds a protocol that the
receiver supports. When establishing an _inbound_ connection, Kubo will let
the initiator choose the protocol, but will refuse to use any of the disabled
transports.

Supported transports are: TLS (priority 100) and Noise (priority 200).

No default priority will ever be less than 100. Lower values have precedence.

#### `Swarm.Transports.Security.TLS`

[TLS](https://github.com/libp2p/specs/tree/master/tls) (1.3) is the default
security transport as of Kubo 0.5.0. It's also the most scrutinized and
trusted security transport.

Default: `100`

Type: `priority`

#### `Swarm.Transports.Security.SECIO`

**REMOVED**:  support for SECIO has been removed. Please remove this option from your config.

#### `Swarm.Transports.Security.Noise`

[Noise](https://github.com/libp2p/specs/tree/master/noise) is slated to replace
TLS as the cross-platform, default libp2p protocol due to ease of
implementation. It is currently enabled by default but with low priority as it's
not yet widely supported.

Default: `200`

Type: `priority`

### `Swarm.Transports.Multiplexers`

Configuration section for libp2p _multiplexer_ transports. Transports enabled in
this section will be used to multiplex duplex connections.

This does not concern all the QUIC transports which use QUIC's builtin muxing.

Multiplexer transports are configured the same way security transports are, with
the `priority` type. Like with security transports, the initiator gets their
first choice.

Supported transport is only: Yamux (priority 100)

No default priority will ever be less than 100.

### `Swarm.Transports.Multiplexers.Yamux`

Yamux is the default multiplexer used when communicating between Kubo nodes.

Default: `100`

Type: `priority`

### `Swarm.Transports.Multiplexers.Mplex`

**REMOVED**: See <https://github.com/ipfs/kubo/issues/9958>

Support for Mplex has been [removed from Kubo and go-libp2p](https://github.com/libp2p/specs/issues/553).
Please remove this option from your config.

## `DNS`

Options for configuring DNS resolution for [DNSLink](https://docs.ipfs.tech/concepts/dnslink/) and `/dns*` [Multiaddrs][libp2p-multiaddrs].

### `DNS.Resolvers`

Map of [FQDNs](https://en.wikipedia.org/wiki/Fully_qualified_domain_name) to custom resolver URLs.

This allows for overriding the default DNS resolver provided by the operating system,
and using different resolvers per domain or TLD (including ones from alternative, non-ICANN naming systems).

Example:

```json
{
  "DNS": {
    "Resolvers": {
      "eth.": "https://dns.eth.limo/dns-query",
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
- Out-of-the-box support for selected non-ICANN TLDs relies on third-party centralized services provided by respective communities on best-effort basis.
- The special value `"auto"` uses DNS resolvers from [AutoConf](#autoconf) when enabled. For example: `{".": "auto"}` uses any custom DoH resolver (global or per TLD) provided by AutoConf system.

Default: `{".": "auto"}`

Type: `object[string -> string]`

### `DNS.MaxCacheTTL`

Maximum duration for which entries are valid in the DoH cache.

This allows you to cap the Time-To-Live suggested by the DNS response ([RFC2181](https://datatracker.ietf.org/doc/html/rfc2181#section-8)).
If present, the upper bound is applied to DoH resolvers in [`DNS.Resolvers`](#dnsresolvers).

Note: this does NOT work with Go's default DNS resolver. To make this a global setting, add a `.` entry to `DNS.Resolvers` first.

**Examples:**

- `"1m"` DNS entries are kept for 1 minute or less.
- `"0s"` DNS entries expire as soon as they are retrieved.

Default: Respect DNS Response TTL

Type: `optionalDuration`

## `HTTPRetrieval`

`HTTPRetrieval` is configuration for pure HTTP retrieval based on Trustless HTTP Gateways'
[Block Responses (`application/vnd.ipld.raw`)](https://specs.ipfs.tech/http-gateways/trustless-gateway/#block-responses-application-vnd-ipld-raw)
which can be used in addition to or instead of retrieving blocks with [Bitswap over Libp2p](#bitswap).

Default: `{}`

Type: `object`

### `HTTPRetrieval.Enabled`

Controls whether HTTP-based block retrieval is enabled.

When enabled, Kubo will act on `/tls/http` (HTTP/2) providers ([Trustless HTTP Gateways](https://specs.ipfs.tech/http-gateways/trustless-gateway/)) returned by the [`Routing.DelegatedRouters`](#routingdelegatedrouters)
to perform pure HTTP [block retrievals](https://specs.ipfs.tech/http-gateways/trustless-gateway/#block-responses-application-vnd-ipld-raw)
(`/ipfs/cid?format=raw`, `Accept: application/vnd.ipld.raw`)
alongside [Bitswap over Libp2p](#bitswap).

HTTP requests for `application/vnd.ipld.raw` will be made instead of Bitswap when a peer has a `/tls/http` multiaddr
and the HTTPS server returns HTTP 200 for the [probe path](https://specs.ipfs.tech/http-gateways/trustless-gateway/#dedicated-probe-paths).

> [!IMPORTANT]
> This feature is relatively new. Please report any issues via [Github](https://github.com/ipfs/kubo/issues/new).
>
> Important notes:
>
> - TLS and HTTP/2 are required. For privacy reasons, and to maintain feature-parity with browsers, unencrypted `http://` providers are ignored and not used.
> - This feature works in the same way as Bitswap: connected HTTP-peers receive optimistic block requests even for content that they are not announcing.
> - For performance reasons, and to avoid loops, the HTTP client does not follow redirects. Providers should keep announcements up to date.
> - IPFS ecosystem is working towards [supporting HTTP providers on Amino DHT](https://github.com/ipfs/specs/issues/496). Currently, HTTP providers are mostly limited to results from [`Routing.DelegatedRouters`](#routingdelegatedrouters) endpoints and requires `Routing.Type=auto|autoclient`.

Default: `true`

Type: `flag`

### `HTTPRetrieval.Allowlist`

Optional list of hostnames for which HTTP retrieval is allowed for.
If this list is not empty, only hosts matching these entries will be allowed for HTTP retrieval.

> [!TIP]
> To limit HTTP retrieval to a provider at `/dns4/example.com/tcp/443/tls/http` (which would serve `HEAD|GET https://example.com/ipfs/cid?format=raw`), set this to `["example.com"]`

Default: `[]`

Type: `array[string]`

### `HTTPRetrieval.Denylist`

Optional list of hostnames for which HTTP retrieval is not allowed.
Denylist entries take precedence over Allowlist entries.

> [!TIP]
> This denylist operates on HTTP endpoint hostnames.
> To deny specific PeerID, use [`Routing.IgnoreProviders`](#routingignoreproviders) instead.

Default: `[]`

Type: `array[string]`

### `HTTPRetrieval.NumWorkers`

The number of worker goroutines to use for concurrent HTTP retrieval operations.
This setting controls the level of parallelism for HTTP-based block retrieval operations.
Higher values can improve performance when retrieving many blocks but may increase resource usage.

Default: `16`

Type: `optionalInteger`

### `HTTPRetrieval.MaxBlockSize`

Sets the maximum size of a block that the HTTP retrieval client will accept.

> [!NOTE]
> This setting is a security feature designed to protect Kubo from malicious providers who might send excessively large or invalid data.
> Increasing this value allows Kubo to retrieve larger blocks from compatible HTTP providers, but doing so reduces interoperability with Bitswap, and increases potential security risks.
>
> Learn more: [Supporting Large IPLD Blocks: Why block limits?](https://discuss.ipfs.tech/t/supporting-large-ipld-blocks/15093#why-block-limits-5)

Default: `2MiB` (matching [Bitswap size limit](https://specs.ipfs.tech/bitswap-protocol/#block-sizes))

Type: `optionalString`

### `HTTPRetrieval.TLSInsecureSkipVerify`

Disables TLS certificate validation.
Allows making HTTPS connections to HTTP/2 test servers with self-signed TLS certificates.
Only for testing, do not use in production.

Default: `false`

Type: `flag`

## `Import`

Options to configure the default options used for ingesting data, in commands such as `ipfs add` or `ipfs block put`. All affected commands are detailed per option.

Note that using flags will override the options defined here.

### `Import.CidVersion`

The default CID version. Commands affected: `ipfs add`.

Must be either 0 or 1. CIDv0 uses SHA2-256 only, while CIDv1 supports multiple hash functions.

Default: `0`

Type: `optionalInteger`

### `Import.UnixFSRawLeaves`

The default UnixFS raw leaves option. Commands affected: `ipfs add`, `ipfs files write`.

Default: `false` if `CidVersion=0`; `true` if `CidVersion=1`

Type: `flag`

### `Import.UnixFSChunker`

The default UnixFS chunker. Commands affected: `ipfs add`.

Valid formats:

- `size-<bytes>` - fixed size chunker
- `rabin-<min>-<avg>-<max>` - rabin fingerprint chunker  
- `buzhash` - buzhash chunker

Default: `size-262144`

Type: `optionalString`

### `Import.HashFunction`

The default hash function. Commands affected: `ipfs add`, `ipfs block put`, `ipfs dag put`.

Must be a valid multihash name (e.g., `sha2-256`, `blake3`) and must be allowed for use in IPFS according to security constraints.

Run `ipfs cid hashes --supported` to see the full list of allowed hash functions.

Default: `sha2-256`

Type: `optionalString`

### `Import.BatchMaxNodes`

The maximum number of nodes in a write-batch. The total size of the batch is limited by `BatchMaxnodes` and `BatchMaxSize`.

Increasing this will batch more items together when importing data with `ipfs dag import`, which can speed things up.

Must be positive (> 0). Setting to 0 would cause immediate batching after each node, which is inefficient.

Default: `128`

Type: `optionalInteger`

### `Import.BatchMaxSize`

The maximum size of a single write-batch (computed as the sum of the sizes of the blocks). The total size of the batch is limited by `BatchMaxnodes` and `BatchMaxSize`.

Increasing this will batch more items together when importing data with `ipfs dag import`, which can speed things up.

Must be positive (> 0). Setting to 0 would cause immediate batching after any data, which is inefficient.

Default: `20971520` (20MiB)

Type: `optionalInteger`

### `Import.UnixFSFileMaxLinks`

The maximum number of links that a node part of a UnixFS File can have
when building the DAG while importing.

This setting controls both the fanout in files that are chunked into several
blocks and grouped as a Unixfs (dag-pb) DAG.

Must be positive (> 0). Zero or negative values would break file DAG construction.

Default: `174`

Type: `optionalInteger`

### `Import.UnixFSDirectoryMaxLinks`

The maximum number of links that a node part of a UnixFS basic directory can
have when building the DAG while importing.

This setting controls both the fanout for basic, non-HAMT folder nodes. It
sets a limit after which directories are converted to a HAMT-based structure.

When unset (0), no limit exists for children. Directories will be converted to
HAMTs based on their estimated size only.

This setting will cause basic directories to be converted to HAMTs when they
exceed the maximum number of children. This happens transparently during the
add process. The fanout of HAMT nodes is controlled by `MaxHAMTFanout`.

Must be non-negative (>= 0). Zero means no limit, negative values are invalid.

Commands affected: `ipfs add`

Default: `0` (no limit, because [`Import.UnixFSHAMTDirectorySizeThreshold`](#importunixfshamtdirectorysizethreshold) triggers controls when to switch to HAMT sharding when a directory grows too big)

Type: `optionalInteger`

### `Import.UnixFSHAMTDirectoryMaxFanout`

The maximum number of children that a node part of a UnixFS HAMT directory
(aka sharded directory) can have.

HAMT directories have unlimited children and are used when basic directories
become too big or reach `MaxLinks`. A HAMT is a structure made of UnixFS
nodes that store the list of elements in the folder. This option controls the
maximum number of children that the HAMT nodes can have.

According to the [UnixFS specification](https://specs.ipfs.tech/unixfs/#hamt-structure-and-parameters), this value must be a power of 2, a multiple of 8 (for byte-aligned bitfields), and not exceed 1024 (to prevent denial-of-service attacks).

Commands affected: `ipfs add`, `ipfs daemon` (globally overrides [`boxo/ipld/unixfs/io.DefaultShardWidth`](https://github.com/ipfs/boxo/blob/6c5a07602aed248acc86598f30ab61923a54a83e/ipld/unixfs/io/directory.go#L30C5-L30C22))

Default: `256`

Type: `optionalInteger`

### `Import.UnixFSHAMTDirectorySizeThreshold`

The sharding threshold to decide whether a basic UnixFS directory
should be sharded (converted into HAMT Directory) or not.

This value is not strictly related to the size of the UnixFS directory block
and any increases in the threshold should come with being careful that block
sizes stay under 2MiB in order for them to be reliably transferable through the
networking stack. At the time of writing this, IPFS peers on the public swarm
tend to ignore requests for blocks bigger than 2MiB.

Uses implementation from `boxo/ipld/unixfs/io/directory`, where the size is not
the _exact_ block size of the encoded directory but just the estimated size
based byte length of DAG-PB Links names and CIDs.

Setting to `1B` is functionally equivalent to always using HAMT (useful in testing).

Commands affected: `ipfs add`, `ipfs daemon` (globally overrides [`boxo/ipld/unixfs/io.HAMTShardingSize`](https://github.com/ipfs/boxo/blob/6c5a07602aed248acc86598f30ab61923a54a83e/ipld/unixfs/io/directory.go#L26))

Default: `256KiB` (may change, inspect `DefaultUnixFSHAMTDirectorySizeThreshold` to confirm)

Type: `optionalBytes`

## `Version`

Options to configure agent version announced to the swarm, and leveraging
other peers version for detecting when there is time to update.

### `Version.AgentSuffix`

Optional suffix to the AgentVersion presented by `ipfs id` and exposed via [libp2p identify protocol](https://github.com/libp2p/specs/blob/master/identify/README.md#agentversion).

The value from config takes precedence over value passed via `ipfs daemon --agent-version-suffix`.

> [!NOTE]
> Setting a custom version suffix helps with ecosystem analysis, such as Amino DHT reports published at <https://stats.ipfs.network>

Default: `""` (no suffix, or value from `ipfs daemon --agent-version-suffix=`)

Type: `optionalString`

### `Version.SwarmCheckEnabled`

Observe the AgentVersion of swarm peers and log warning when
`SwarmCheckPercentThreshold` of peers runs version higher than this node.

Default: `true`

Type: `flag`

### `Version.SwarmCheckPercentThreshold`

Control the percentage of `kubo/` peers running new version required to
trigger update warning.

Default: `5`

Type: `optionalInteger` (1-100)

## Profiles

Configuration profiles allow to tweak configuration quickly. Profiles can be
applied with the `--profile` flag to `ipfs init` or with the `ipfs config profile
apply` command. When a profile is applied a backup of the configuration file
will be created in `$IPFS_PATH`.

Configuration profiles can be applied additively. For example, both the `test-cid-v1` and `lowpower` profiles can be applied one after the other.
The available configuration profiles are listed below. You can also find them
documented in `ipfs config profile --help`.

### `server` profile

Disables local [`Discovery.MDNS`](#discoverymdns), [turns off uPnP NAT port mapping](#swarmdisablenatportmap),  and blocks connections to
IPv4 and IPv6 prefixes that are [private, local only, or unrouteable](https://github.com/ipfs/kubo/blob/b71cf0d15904bdef21fe2eee5f1118a274309a4d/config/profile.go#L24-L43).

Recommended when running IPFS on machines with public IPv4 addresses (no NAT, no uPnP)
at providers that interpret local IPFS discovery and traffic as netscan abuse ([example](https://github.com/ipfs/kubo/issues/10327)).

### `randomports` profile

Use a random port number for the incoming swarm connections.
Used for testing.

### `default-datastore` profile

Configures the node to use the default datastore (flatfs).

Read the "flatfs" profile description for more information on this datastore.

This profile may only be applied when first initializing the node.

### `local-discovery` profile

Enables local [`Discovery.MDNS`](#discoverymdns) (enabled by default).

Useful to re-enable local discovery after it's disabled by another profile
(e.g., the server profile).

`test` profile

Reduces external interference of IPFS daemon, this
is useful when using the daemon in test environments.

### `default-networking` profile

Restores default network settings.
Inverse profile of the test profile.

### `autoconf-on` profile

Safe default for joining the public IPFS Mainnet swarm with automatic configuration.
Can also be used with custom AutoConf.URL for other networks.

### `autoconf-off` profile

Disables AutoConf and clears all networking fields for manual configuration.
Use this for private networks or when you want explicit control over all endpoints.

### `flatfs` profile

Configures the node to use the flatfs datastore.
Flatfs is the default, most battle-tested and reliable datastore.

You should use this datastore if:

- You need a very simple and very reliable datastore, and you trust your
  filesystem. This datastore stores each block as a separate file in the
  underlying filesystem so it's unlikely to lose data unless there's an issue
  with the underlying file system.
- You need to run garbage collection in a way that reclaims free space as soon as possible.
- You want to minimize memory usage.
- You are ok with the default speed of data import, or prefer to use `--nocopy`.

> [!WARNING]
> This profile may only be applied when first initializing the node via `ipfs init --profile flatfs`

> [!NOTE]
> See caveats and configuration options at [`datastores.md#flatfs`](datastores.md#flatfs)

### `flatfs-measure` profile

Configures the node to use the flatfs datastore with metrics. This is the same as [`flatfs` profile](#flatfs-profile) with the addition of the `measure` datastore wrapper.

### `pebbleds` profile

Configures the node to use the pebble high-performance datastore.

Pebble is a LevelDB/RocksDB inspired key-value store focused on performance and internal usage by CockroachDB.
You should use this datastore if:

- You need a datastore that is focused on performance.
- You need a datastore that is good for multi-terabyte data sets.
- You need reliability by default, but may choose to disable WAL for maximum performance when reliability is not critical.
- You want a datastore that does not need GC cycles and does not use more space than necessary
- You want a datastore that does not take several minutes to start with large repositories
- You want a datastore that performs well even with default settings, but can optimized by setting configuration to tune it for your specific needs.

> [!WARNING]
> This profile may only be applied when first initializing the node via `ipfs init --profile pebbleds`

> [!NOTE]
> See other caveats and configuration options at [`datastores.md#pebbleds`](datastores.md#pebbleds)

### `pebbleds-measure` profile

Configures the node to use the pebble datastore with metrics. This is the same as [`pebbleds` profile](#pebble-profile) with the addition of the `measure` datastore wrapper.

### `badgerds` profile

Configures the node to use the **legacy** badgerv1 datastore.

> [!CAUTION]
> This is based on very old badger 1.x, which has known bugs and is no longer supported by the upstream team.
> It is provided here only for pre-existing users, allowing them to migrate away to more modern datastore.
> Do not use it for new deployments, unless you really, really know what you are doing.

Also, be aware that:

- This datastore will not properly reclaim space when your datastore is
  smaller than several gigabytes. If you run IPFS with `--enable-gc`, you plan on storing very little data in
  your IPFS node, and disk usage is more critical than performance, consider using
  `flatfs`.
- This datastore uses up to several gigabytes of memory.
- Good for medium-size datastores, but may run into performance issues if your dataset is bigger than a terabyte.
- The current implementation is based on old badger 1.x which is no longer supported by the upstream team.

> [!WARNING]
> This profile may only be applied when first initializing the node via `ipfs init --profile badgerds`

> [!NOTE]
> See other caveats and configuration options at [`datastores.md#pebbleds`](datastores.md#pebbleds)

### `badgerds-measure` profile

Configures the node to use the **legacy** badgerv1 datastore with metrics. This is the same as [`badgerds` profile](#badger-profile) with the addition of the `measure` datastore wrapper.

### `lowpower` profile

Reduces daemon overhead on the system by disabling optional swarm services.

- [`Routing.Type`](#routingtype) set to `autoclient` (no DHT server, only client).
- `Swarm.ConnMgr` set to maintain minimum number of p2p connections at a time.
- Disables [`AutoNAT`](#autonat).
- Disables [`Swam.RelayService`](#swarmrelayservice).

> [!NOTE]
> This profile is provided for legacy reasons.
> With modern Kubo setting the above should not be necessary.

### `announce-off` profile

Disables [Provide](#provide) system (and announcing to Amino DHT).

> [!CAUTION]
> The main use case for this is setups with manual Peering.Peers config.
> Data from this node will not be announced on the DHT. This will make
> DHT-based routing an data retrieval impossible if this node is the only
> one hosting it, and other peers are not already connected to it.

### `announce-on` profile

(Re-)enables [Provide](#provide) system (reverts [`announce-off` profile](#announce-off-profile)).

### `legacy-cid-v0` profile

Makes UnixFS import (`ipfs add`) produce legacy CIDv0 with no raw leaves, sha2-256 and 256 KiB chunks.

See <https://github.com/ipfs/kubo/blob/master/config/profile.go> for exact [`Import.*`](#import) settings.

> [!NOTE]
> This profile is provided for legacy users and should not be used for new projects.

### `test-cid-v1` profile

Makes UnixFS import (`ipfs add`) produce modern CIDv1 with raw leaves, sha2-256
and 1 MiB chunks (max 174 links per file, 256 per HAMT node, switch dir to HAMT
above 256KiB).

See <https://github.com/ipfs/kubo/blob/master/config/profile.go> for exact [`Import.*`](#import) settings.

> [!NOTE]
> [`Import.*`](#import) settings applied by this profile MAY change in future release. Provided for testing purposes.
>
> Follow [kubo#4143](https://github.com/ipfs/kubo/issues/4143) for more details,
> and provide feedback in [discuss.ipfs.tech/t/should-we-profile-cids](https://discuss.ipfs.tech/t/should-we-profile-cids/18507) or [ipfs/specs#499](https://github.com/ipfs/specs/pull/499).

### `test-cid-v1-wide` profile

Makes UnixFS import (`ipfs add`) produce modern CIDv1 with raw leaves, sha2-256
and 1 MiB chunks and wider file DAGs (max 1024 links per every node type,
switch dir to HAMT above 1MiB).

See <https://github.com/ipfs/kubo/blob/master/config/profile.go> for exact [`Import.*`](#import) settings.

> [!NOTE]
> [`Import.*`](#import) settings applied by this profile MAY change in future release. Provided for testing purposes.
>
> Follow [kubo#4143](https://github.com/ipfs/kubo/issues/4143) for more details,
> and provide feedback in [discuss.ipfs.tech/t/should-we-profile-cids](https://discuss.ipfs.tech/t/should-we-profile-cids/18507) or [ipfs/specs#499](https://github.com/ipfs/specs/pull/499).

## Security

This section provides an overview of security considerations for configurations that expose network services.

### Port and Network Exposure

Several configuration options expose TCP or UDP ports that can make your Kubo node accessible from the network:

- **[`Addresses.API`](#addressesapi)** - Exposes the admin RPC API (default: localhost:5001)
- **[`Addresses.Gateway`](#addressesgateway)** - Exposes the HTTP gateway (default: localhost:8080)
- **[`Addresses.Swarm`](#addressesswarm)** - Exposes P2P connectivity (default: 0.0.0.0:4001, both UDP and TCP)
- **[`Swarm.Transports.Network`](#swarmtransportsnetwork)** - Controls which P2P transport protocols are enabled over TCP and UDP

### Security Best Practices

- Keep admin services ([`Addresses.API`](#addressesapi)) bound to localhost unless authentication ([`API.Authorizations`](#apiauthorizations)) is configured
- Use [`Gateway.NoFetch`](#gatewaynofetch) to prevent arbitrary CID retrieval if Kubo is acting as a public gateway available to anyone
- Configure firewall rules to restrict access to exposed ports. Note that [`Addresses.Swarm`](#addressesswarm) is special - all incoming traffic to swarm ports should be allowed to ensure proper P2P connectivity
- Control which public-facing addresses are announced to other peers using [`Addresses.NoAnnounce`](#addressesnoannounce), [`Addresses.Announce`](#addressesannounce), and [`Addresses.AppendAnnounce`](#addressesappendannounce)
- Consider using the [`server` profile](#server-profile) for production deployments

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

----

[multiaddr]: https://docs.ipfs.tech/concepts/glossary/#multiaddr
