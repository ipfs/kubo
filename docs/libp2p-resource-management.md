The [libp2p Network Resource Manager](https://github.com/libp2p/go-libp2p/tree/master/p2p/host/resource-manager#readme) allows setting limits per [Resource Scope](https://github.com/libp2p/go-libp2p/tree/master/p2p/host/resource-manager#resource-scopes)

##### Levels of Configuration

libp2p's resource manager provides tremendous flexibility but also adds a lot of complexity.
There are these levels of limit configuration for resource management protection:
1. "The user who does nothing" - In this case they get some sane defaults discussed below
   based on the amount of memory and file descriptors their system has.
   This should protect the node from many attacks.
2. "Slightly more advanced user" - They can tweak the default limits discussed below.  
   Where the defaults aren't good enough, a good set of higher-level "knobs" are exposed to satisfy most use cases
   without requiring users to wade into all the intricacies of libp2p's resource manager.
   The "knobs"/inputs are `Swarm.ResourceMgr.MaxMemory` and `Swarm.ResourceMgr.MaxFileDescriptors` as described below. 
3. "Power user" - They specify all the default limits from below they want override via `Swarm.ResourceMgr.Limits`;

##### Default Limits

With these inputs defined, [resource manager limits](https://github.com/libp2p/go-libp2p/tree/master/p2p/host/resource-manager#limits) are created at the 
[system](https://github.com/libp2p/go-libp2p/tree/master/p2p/host/resource-manager#the-system-scope), 
[transient](https://github.com/libp2p/go-libp2p/tree/master/p2p/host/resource-manager#the-transient-scope), 
and [peer](https://github.com/libp2p/go-libp2p/tree/master/p2p/host/resource-manager#peer-scopes) scopes.
Other scopes are ignored (by being set to "~infinity".

The reason these scopes are chosen is because:
- system - This gives us the coarse-grained control we want so we can reason about the system as a whole.
  It is the backstop, and allows us to reason about resource consumption more easily
  since don't have think about the interaction of many other scopes.
- transient - Limiting connections that are in process of being established provides backpressure so not too much work queues up.
- peer - The peer scope doesn't protect us against intentional DoS attacks.
  It's just as easy for an attacker to send 100 requests/second with 1 peerId vs. 10 requests/second with 10 peers.
  We are reliant on the system scope for protection here in the malicious case.
  The reason for having a peer scope is to protect against unintentional DoS attacks
  (e.g., bug in a peer which is causing it to "misbehave").
  In the unintional case, we want to make sure a "misbehaving" node doesn't consume more resources than necessary.

Within these scopes, limits are just set on 
[memory](https://github.com/libp2p/go-libp2p/tree/master/p2p/host/resource-manager#memory), 
[file descriptors (FD)](https://github.com/libp2p/go-libp2p/tree/master/p2p/host/resource-manager#file-descriptors), [*inbound* connections](https://github.com/libp2p/go-libp2p/tree/master/p2p/host/resource-manager#connections),
and [*inbound* streams](https://github.com/libp2p/go-libp2p/tree/master/p2p/host/resource-manager#streams).
Limits are set based on the inputs above.
We trust this node to behave properly and thus don't limit *outbound* connection/stream limits.
We apply any limits that libp2p has for its protocols/services
since we assume libp2p knows best here.

##### Active Limits
A dump of what limits were computed and are actually being used by the resource manager
can be obtained by `ipfs swarm limit all`.

##### libp2p resource monitoring
For [monitoring libp2p resource usage](https://github.com/libp2p/go-libp2p/tree/master/p2p/host/resource-manager#monitoring), 
various `*rcmgr_*` metrics can be accessed as the prometheus endpoint at `{Addresses.API}/debug/metrics/prometheus` (default: `http://127.0.0.1:5001/debug/metrics/prometheus`).  
There are also [pre-built Grafana dashboards](https://github.com/libp2p/go-libp2p/tree/master/p2p/host/resource-manager/obs/grafana-dashboards) that can be added to a Grafana instance. 

A textual view of current resource usage and a list of services, protocols, and peers can be
obtained via `ipfs swarm stats --help`

#### `Swarm.ResourceMgr.Enabled`

Enables the libp2p Resource Manager using limits based on the defaults and/or other configuration as discussed above.

Default: `true`
Type: `flag`

#### `Swarm.ResourceMgr.MaxMemory`

This is the max amount of memory to allow libp2p to use.
libp2p's resource manager will prevent additional resource creation while this limit is reached.
This value is also used to scale the limit on various resources at various scopes 
when the default limits (discuseed above) are used.
For example, increasing this value will increase the default limit for incoming connections.

Default: `[TOTAL_SYSTEM_MEMORY]/8`
Type: `optionalBytes`

#### `Swarm.ResourceMgr.MaxFileDescriptors`

This is the maximum number of file descriptors to allow libp2p to use.
libp2p's resource manager will prevent additional file descriptor consumption while this limit is reached.

This param is ignored on Windows.

Default `[TOTAL_SYSTEM_FILE_DESCRIPTORS]/2`
Type: `optionalInteger`

#### `Swarm.ResourceMgr.Limits`

Map of resource limits [per scope](https://github.com/libp2p/go-libp2p/tree/master/p2p/host/resource-manager#resource-scopes).

The map supports fields from the [`LimitConfig` struct](https://github.com/libp2p/go-libp2p/blob/master/p2p/host/resource-manager/limit_defaults.go#L111).

[`BaseLimit`s](https://github.com/libp2p/go-libp2p/blob/master/p2p/host/resource-manager/limit.go#L89) can be set for any scope, and within the `BaseLimit`, all limit <key,value>s are optional.

The `Swarm.ResourceMgr.Limits` override the default limits described above. 
Any override `BaseLimits` or limit <key,value>s from `Swarm.ResourceMgr.Limits`
that aren't specified will use the default limits.

Example #1: setting limits for a specific scope
```json
{
  "Swarm": {
    "ResourceMgr": {
      "Limits": {
        "System": {
          "Memory": 1073741824,
          "FD": 512,
          "Conns": 1024,
          "ConnsInbound": 256,
          "ConnsOutbound": 1024,
          "Streams": 16384,
          "StreamsInbound": 4096,
          "StreamsOutbound": 16384
        }
      }
    }
  }
}
```

Example #2: setting a specific <key,value> limit
```json
{
  "Swarm": {
    "ResourceMgr": {
      "Limits": {
        "Transient": {
          "ConnsOutbound": 256,
        }
      }
    }
  }
}
```

It is also possible to adjust some runtime limits via `ipfs swarm limit --help`.
Changes made via `ipfs swarm limit` are persisted in `Swarm.ResourceMgr.Limits`.

Default: `{}` (use the safe implicit defaults described above)

Type: `object[string->object]`

#### `Swarm.ResourceMgr.Allowlist`

A list of multiaddrs that can bypass normal system limits (but are still limited by the allowlist scope).
Convenience config around [go-libp2p-resource-manager#Allowlist.Add](https://pkg.go.dev/github.com/libp2p/go-libp2p/p2p/host/resource-manager#Allowlist.Add).

Default: `[]`

Type: `array[string]` (multiaddrs)