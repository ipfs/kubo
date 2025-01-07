# New multi-router configuration system

- Start Date: 2022-08-15
- Related Issues:
  - https://github.com/ipfs/kubo/issues/9188
  - https://github.com/ipfs/kubo/issues/9079
  - https://github.com/ipfs/kubo/pull/9877

## Summary

Previously we only used the Amino DHT for content routing and content
providing.

Kubo 0.14 introduced experimental support for [delegated routing](https://github.com/ipfs/kubo/pull/8997),
which then got changed and standardized as [Routing V1 HTTP API](https://specs.ipfs.tech/routing/http-routing-v1/).

Kubo 0.23.0 release added support for [self-hosting Routing V1 HTTP API server](https://github.com/ipfs/kubo/blob/master/docs/changelogs/v0.23.md#self-hosting-routingv1-endpoint-for-delegated-routing-needs).

Now we need a better way to add different routers using different protocols
like [Routing V1](https://specs.ipfs.tech/routing/http-routing-v1/) or Amino
DHT, and be able to configure them (future routing systems to come) to cover different use cases.

## Motivation

The actual routing implementation is not enough. Some users need to have more options when configuring the routing system. The new implementations should be able to:

- [x] Be user-friendly and easy enough to configure, but also versatile
- [x] Configurable Router execution order
     - [x] Delay some of the Router methods execution when they will be executed on parallel
- [x] Configure which method of a giving router will be used
- [x] Mark some router methods as mandatory to make the execution fails if that method fails

## Detailed design

### Configuration file description

The `Routing` configuration section will contain the following keys:

#### Type

`Type` will be still in use to avoid complexity for the user that only wants to use Kubo with the default behavior. We are going to add a new type, `custom`, that will use the new router systems. `none` type will deactivate **all** routers, default dht and delegated ones.

#### Routers

`Routers` will be a key-value list of routers that will be available to use. The key is the router name and the value is all the needed configurations for that router. the `Type` will define the routing kind. The main router types will be `http` and `dht`, but we will implement two special routers used to execute a set of routers in parallel or sequentially: `parallel` router and `sequential` router.

Depending on the routing type, it will use different parameters:

##### HTTP

Params:

- `"Endpoint"`: URL of HTTP server with endpoints that implement [Delegated Routing V1 HTTP API](https://specs.ipfs.tech/routing/http-routing-v1/) protocol.

##### Amino DHT

Params:
- `"Mode"`: Mode used by the Amino DHT. Possible values: "server", "client", "auto"
- `"AcceleratedDHTClient"`: Set to `true` if you want to use the experimentalDHT.
- `"PublicIPNetwork"`: Set to `true` to create a `WAN` Amino DHT. Set to `false` to create a `LAN` DHT.

##### Parallel

Params:
- `Routers`: A list of routers that will be executed in parallel:
    - `Name:string`: Name of the router. It should be one of the previously added to `Routers` list.
    - `Timeout:duration`: Local timeout. It accepts strings compatible with Go `time.ParseDuration(string)`. Time will start counting when this specific router is called, and it will stop when the router returns, or we reach the specified timeout.
    - `ExecuteAfter:duration`: Providing this param will delay the execution of that router at the specified time. It accepts strings compatible with Go `time.ParseDuration(string)`.
    - `IgnoreErrors:bool`: It will specify if that router should be ignored if an error occurred.
- `Timeout:duration`: Global timeout.  It accepts strings compatible with Go `time.ParseDuration(string)`.
##### Sequential

Params:
- `Routers`: A list of routers that will be executed in order:
    - `Name:string`: Name of the router. It should be one of the previously added to `Routers` list.
    - `Timeout:duration`: Local timeout. It accepts strings compatible with Go `time.ParseDuration(string)`. Time will start counting when this specific router is called, and it will stop when the router returns, or we reach the specified timeout.
    - `IgnoreErrors:bool`: It will specify if that router should be ignored if an error occurred.
- `Timeout:duration`: Global timeout.  It accepts strings compatible with Go `time.ParseDuration(string)`.
#### Methods

`Methods:map` will define which routers will be executed per method. The key will be the name of the method: `"provide"`, `"find-providers"`, `"find-peers"`, `"put-ipns"`, `"get-ipns"`. All methods must be added to the list. This will make configuration discoverable giving good errors to the user if a method is missing.

The value will contain:
- `RouterName:string`: Name of the router. It should be one of the previously added to `Routers` list.

#### Configuration file example:

```json
"Routing": {
  "Type": "custom",
  "Routers": {
    "http-delegated": {
      "Type": "http",
      "Parameters": {
        "Endpoint": "https://delegated-ipfs.dev" // /routing/v1 (https://specs.ipfs.tech/routing/http-routing-v1/)
      }
    },
    "dht-lan": {
      "Type": "dht",
      "Parameters": {
        "Mode": "server",
        "PublicIPNetwork": false,
        "AcceleratedDHTClient": false
      }
    },
    "dht-wan": {
      "Type": "dht",
      "Parameters": {
        "Mode": "auto",
        "PublicIPNetwork": true,
        "AcceleratedDHTClient": false
      }
    },
    "find-providers-router": {
      "Type": "parallel",
      "Parameters": {
        "Routers": [
          {
            "RouterName": "dht-lan",
            "IgnoreErrors": true
          },
          {
            "RouterName": "dht-wan"
          },
          {
            "RouterName": "http-delegated"
          }
        ]
      }
    },
    "provide-router": {
      "Type": "parallel",
      "Parameters": {
        "Routers": [
          {
            "RouterName": "dht-lan",
            "IgnoreErrors": true
          },
          {
            "RouterName": "dht-wan",
            "ExecuteAfter": "100ms",
            "Timeout": "100ms"
          },
          {
            "RouterName": "http-delegated",
            "ExecuteAfter": "100ms"
          }
        ]
      }
    },
    "get-ipns-router": {
      "Type": "sequential",
      "Parameters": {
        "Routers": [
          {
            "RouterName": "dht-lan",
            "IgnoreErrors": true
          },
          {
            "RouterName": "dht-wan",
            "Timeout": "300ms"
          },
          {
            "RouterName": "http-delegated",
            "Timeout": "300ms"
          }
        ]
      }
    },
    "put-ipns-router": {
      "Type": "parallel",
      "Parameters": {
        "Routers": [
          {
            "RouterName": "dht-lan"
          },
          {
            "RouterName": "dht-wan"
          },
          {
            "RouterName": "http-delegated"
          }
        ]
      }
    }
  },
  "Methods": {
    "find-providers": {
      "RouterName": "find-providers-router"
    },
    "provide": {
      "RouterName": "provide-router"
    },
    "get-ipns": {
      "RouterName": "get-ipns-router"
    },
    "put-ipns": {
      "RouterName": "put-ipns-router"
    }
  }
}
```

### Error cases
 - If any of the routers fails, the output will be an error by default.
 - You can use `IgnoreErrors:true` to ignore errors for a specific router output
 - To avoid any error at the output, you must ignore all router errors.

### Implementation Details

#### Methods

All routers must implement the `routing.Routing` interface:

```go=
type Routing interface {
    ContentRouting
    PeerRouting
    ValueStore

    Bootstrap(context.Context) error
}
```

All methods involved:

```go=
type Routing interface {
    Provide(context.Context, cid.Cid, bool) error
    FindProvidersAsync(context.Context, cid.Cid, int) <-chan peer.AddrInfo
    
    FindPeer(context.Context, peer.ID) (peer.AddrInfo, error)

    PutValue(context.Context, string, []byte, ...Option) error
    GetValue(context.Context, string, ...Option) ([]byte, error)
    SearchValue(context.Context, string, ...Option) (<-chan []byte, error)

    Bootstrap(context.Context) error
}
```
We can configure which methods will be used per routing implementation. Methods names used in the configuration file will be:

- `Provide`: `"provide"`
- `FindProvidersAsync`: `"find-providers"`
- `FindPeer`: `"find-peers"`
- `PutValue`: `"put-ipns"`
- `GetValue`, `SearchValue`: `"get-ipns"`
- `Bootstrap`: It will be always executed when needed.

#### Routers

We need to implement the `parallel` and `sequential` routers and stop using `routinghelpers.Tiered` router implementation.

Add cycle detection to avoid to user some headaches.

Also we need to implement an internal router, that will define the router used per method.

#### Other considerations

- We need to refactor how DHT routers are created to be able to use and add any amount of custom DHT routers.
- We need to add a new `custom` router type to be able to use the new routing system.
- Bitswap WANT broadcasting is not included on this document, but it can be added in next iterations.
- This document will live in docs/design-notes for historical reasons and future reference.

## Test fixtures

As test fixtures we can add different use cases here and see how the configuration will look like.

### Mimic previous dual DHT config

```json
"Routing": {
  "Type": "custom",
  "Routers": {
    "dht-lan": {
      "Type": "dht",
      "Parameters": {
        "Mode": "server",
        "PublicIPNetwork": false
      }
    },
    "dht-wan": {
      "Type": "dht",
      "Parameters": {
        "Mode": "auto",
        "PublicIPNetwork": true
      }
    },
    "parallel-dht-strict": {
      "Type": "parallel",
      "Parameters": {
        "Routers": [
          {
            "RouterName": "dht-lan"
          },
          {
            "RouterName": "dht-wan"
          }
        ]
      }
    },
    "parallel-dht": {
      "Type": "parallel",
      "Parameters": {
        "Routers": [
          {
            "RouterName": "dht-lan",
            "IgnoreError": true
          },
          {
            "RouterName": "dht-wan"
          }
        ]
      }
    }
  },
  "Methods": {
    "provide": {
      "RouterName": "dht-wan"
    },
    "find-providers": {
      "RouterName": "parallel-dht-strict"
    },
    "find-peers": {
      "RouterName": "parallel-dht-strict"
    },
    "get-ipns": {
      "RouterName": "parallel-dht"
    },
    "put-ipns": {
      "RouterName": "parallel-dht"
    }
  }
}
```

### Compatibility

~~We need to create a config migration using [fs-repo-migrations](https://github.com/ipfs/fs-repo-migrations). We should remove the `Routing.Type` param and add the configuration specified [previously](#Mimic-previous-dual-DHT-config).~~

We don't need to create any config migration! To avoid to the users the hassle of understanding how the new routing system works, we are going to keep the old behavior. We will add the Type `custom` to make available the new Routing system.

### Security

No new security implications or considerations were found.

### Alternatives

I got ideas from all of the following links to create this design document:

- https://github.com/ipfs/kubo/issues/9079#issuecomment-1211288268
- https://github.com/ipfs/kubo/issues/9157
- https://github.com/ipfs/kubo/issues/9079#issuecomment-1205000253
- https://www.notion.so/pl-strflt/Delegated-Routing-Thoughts-very-very-WIP-0543bc51b1bd4d63a061b0f28e195d38
- https://gist.github.com/guseggert/effa027ff4cbadd7f67598efb6704d12

### Copyright

Copyright and related rights waived via [CC0](https://creativecommons.org/publicdomain/zero/1.0/).
