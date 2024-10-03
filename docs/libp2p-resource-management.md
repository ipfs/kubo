<!-- omit in toc -->
# libp2p Network Resource Manager <small>(`Swarm.ResourceMgr`)</small>

## Purpose
The purpose of this document is to provide more information about the [libp2p Network Resource Manager](https://github.com/libp2p/go-libp2p/tree/master/p2p/host/resource-manager#readme) and how it's integrated into Kubo so that Kubo users can understand and configure it appropriately.

## 🙋 Help!  The resource manager is protecting my node but I want to understand more
The resource manager is generally a *feature* to bound libp2p's resources, whether from bugs, unintentionally misbehaving peers, or intentional Denial of Service attacks.

Good places to start are:
1. Understand [how the resource manager is configured](#levels-of-configuration).
2. Understand [how to read the log message](#what-do-these-protected-from-exceeding-resource-limits-log-messages-mean)
3. Understand [how to inspect and change limits](#user-supplied-override-limits)

## Table of Contents

- [Purpose](#purpose)
- [🙋 Help!  The resource manager is protecting my node but I want to understand more](#-help--the-resource-manager-is-protecting-my-node-but-i-want-to-understand-more)
- [Table of Contents](#table-of-contents)
- [Levels of Configuration](#levels-of-configuration)
  - [Approach](#approach)
  - [Computed Default Limits](#computed-default-limits)
  - [User Supplied Override Limits](#user-supplied-override-limits)
- [FAQ](#faq)
  - [What do these "Protected from exceeding resource limits" log messages mean?](#what-do-these-protected-from-exceeding-resource-limits-log-messages-mean)
  - [How does one see the Active Limits?](#how-does-one-see-the-active-limits)
  - [How does one see the Computed Default Limits?](#how-does-one-see-the-computed-default-limits)
  - [How does one monitor libp2p resource usage?](#how-does-one-monitor-libp2p-resource-usage)
  - [How does the resource manager (ResourceMgr) relate to the connection manager (ConnMgr)?](#how-does-the-resource-manager-resourcemgr-relate-to-the-connection-manager-connmgr)
  - [What are the "Application error 0x0 (remote) ... cannot reserve ..." messages?](#what-are-the-application-error-0x0-remote--cannot-reserve--messages)
- [History](#history)

## Levels of Configuration

See also the [`Swarm.ResourceMgr` config docs](./config.md#swarmresourcemgr).

### Approach
libp2p's resource manager provides tremendous flexibility but also adds complexity.  There are these levels of limit configuration for resource management protection:

1. "The user who does nothing" - In this case Kubo attempts to give some sane defaults discussed below
   based on the amount of memory and file descriptors their system has.
   This should protect the node from many attacks.

2. "Slightly more advanced user" - Where the defaults aren't good enough, a good set of higher-level "knobs" are exposed to satisfy most use cases
   without requiring users to wade into all the intricacies of libp2p's resource manager.
   The "knobs"/inputs are `Swarm.ResourceMgr.MaxMemory` and `Swarm.ResourceMgr.MaxFileDescriptors` as described below.

3. "Power user" - They [specify override limits](#user-supplied-override-limits) and own their own destiny without Kubo getting in the way.

### Computed Default Limits
With the `Swarm.ResourceMgr.MaxMemory` and `Swarm.ResourceMgr.MaxFileDescriptors` inputs defined,
[resource manager limits](https://github.com/libp2p/go-libp2p/tree/master/p2p/host/resource-manager#limits) are created at the
[system](https://github.com/libp2p/go-libp2p/tree/master/p2p/host/resource-manager#the-system-scope),
[transient](https://github.com/libp2p/go-libp2p/tree/master/p2p/host/resource-manager#the-transient-scope),
and [peer](https://github.com/libp2p/go-libp2p/tree/master/p2p/host/resource-manager#peer-scopes) scopes.
Other scopes are ignored (by being set to "unlimited").

The reason these scopes are chosen is because:
- `system` - This gives us the coarse-grained control we want so we can reason about the system as a whole.
  It is the backstop, and allows us to reason about resource consumption more easily
  since don't have think about the interaction of many other scopes.
- `transient` - Limiting connections that are in process of being established provides backpressure so not too much work queues up.
- `peer` - The peer scope doesn't protect us against intentional DoS attacks.
  It's just as easy for an attacker to send 100 requests/second with 1 peerId vs. 10 requests/second with 10 peers.
  We are reliant on the system scope for protection here in the malicious case.
  The reason for having a peer scope is to protect against unintentional DoS attacks
  (e.g., bug in a peer which is causing it to "misbehave").
  In the unintentional case, we want to make sure a "misbehaving" node doesn't consume more resources than necessary.

Within these scopes, limits are set on:
1. [memory](https://github.com/libp2p/go-libp2p/tree/master/p2p/host/resource-manager#memory)
2. [file descriptors (FD)](https://github.com/libp2p/go-libp2p/tree/master/p2p/host/resource-manager#file-descriptors)
3. [*inbound* connections](https://github.com/libp2p/go-libp2p/tree/master/p2p/host/resource-manager#connections).
Limits are set based on the `Swarm.ResourceMgr.MaxMemory` and `Swarm.ResourceMgr.MaxFileDescriptors` inputs above.

There are also some special cases where minimum values are enforced.
For example, Kubo maintainers have found in practice that it's a footgun to have too low of a value for `System.ConnsInbound` and a default minimum is used. (See [core/node/libp2p/rcmgr_defaults.go](https://github.com/ipfs/kubo/blob/master/core/node/libp2p/rcmgr_defaults.go) for specifics.)

We trust this node to behave properly and thus don't limit *outbound* connection/stream limits.
We apply any limits that libp2p has for its protocols/services
since we assume libp2p knows best here.

Source: [core/node/libp2p/rcmgr_defaults.go](https://github.com/ipfs/kubo/blob/master/core/node/libp2p/rcmgr_defaults.go)

### User Supplied Override Limits
A user who wants fine control over the limits used by the go-libp2p resource manager can specify overrides to the [computed default limits](#computed-default-limits).
This is done by defining limits in ``$IPFS_PATH/libp2p-resource-limit-overrides.json``.
These values trump anything else and are parsed directly by go-libp2p.
(See the [go-libp2p Resource Manager README](https://github.com/libp2p/go-libp2p/blob/master/p2p/host/resource-manager/README.md) for formatting.) 

## FAQ

### What do these "Protected from exceeding resource limits" log messages mean?
"Protected from exceeding resource limits" log messages denote that the resource manager is working and that it prevented additional resources from being used beyond the set limits.  Per [libp2p code](https://github.com/libp2p/go-libp2p/blob/master/p2p/host/resource-manager/scope.go), these messages take the form of "$scope: cannot reserve $limitKey".  

As an example:

> Protected from exceeding resource limits 2 times: "system: cannot reserve inbound connection: resource limit exceeded"

This means that there were 2 recent occurrences where the libp2p resource manager prevented an inbound connection at the "system" [scope](https://github.com/libp2p/go-libp2p/tree/master/p2p/host/resource-manager#resource-scopes).  
Specifically the ``System.ConnsInbound`` limit was hit.  

This can be analyzed by viewing the limit and current usage with `ipfs swarm resources`.
`System.ConnsInbound` is likely close or at the limit value.

The simplest way to identify all resources across all scopes that are close to exceeding their limit (>90% usage) is with a command like `ipfs swarm resources | egrep "9.\..%"` 

Sources:
* [kubo resource manager logging](https://github.com/ipfs/kubo/blob/master/core/node/libp2p/rcmgr_logging.go)
* [libp2p resource manager messages](https://github.com/libp2p/go-libp2p/blob/master/p2p/host/resource-manager/scope.go)

### How does one see the Active Limits?
A dump of what limits are actually being used by the resource manager ([Computed Default Limits](#computed-default-limits) + [User Supplied Override Limits](#user-supplied-override-limits))
can be obtained by `ipfs swarm resources`.

### How does one see the Computed Default Limits?
This can be observed [seeing the active limits](#how-does-one-see-the-active-limits) assuming one hasn't detoured into "power user" mode with [User Supplied Override Limits](#user-supplied-override-limits).

### How does one monitor libp2p resource usage?

For [monitoring libp2p resource usage](https://github.com/libp2p/go-libp2p/tree/master/p2p/host/resource-manager#monitoring), 
various `*rcmgr_*` metrics can be accessed as the Prometheus endpoint at `{Addresses.API}/debug/metrics/prometheus` (default: `http://127.0.0.1:5001/debug/metrics/prometheus`).  
There are also [pre-built Grafana dashboards](https://github.com/libp2p/go-libp2p/tree/master/p2p/host/resource-manager/obs/grafana-dashboards) that can be added to a Grafana instance. 

A textual view of current resource usage and a list of services, protocols, and peers can be
obtained via `ipfs swarm stats --help`

### How does the resource manager (ResourceMgr) relate to the connection manager (ConnMgr)?
As discussed [here](https://github.com/libp2p/go-libp2p/tree/master/p2p/host/resource-manager#connmanager-vs-resource-manager)
these are separate systems in go-libp2p.
Kubo performs sanity checks to ensure that some of the hard limits of the ResourceMgr are sufficiently greater than the soft limits of the ConnMgr.

The soft limit of `Swarm.ConnMgr.HighWater` needs to be less than the resource manager hard limit `System.ConnsInbound` for the configuration to make sense.
This ensures the ConnMgr cleans up connections based on connection priorities before the hard limits of the ResourceMgr are applied.
If `Swarm.ConnMgr.HighWater` is greater than resource manager's `System.ConnsInbound`,
existing low-priority idle connections can prevent new high-priority connections from being established.
The ResourceMgr doesn't know that the new connection is high priority and simply blocks it because of the limit its enforcing.

To ensure the ConnMgr and ResourceMgr are congruent, the ResourceMgr [computed default limits](#computed-default-limits) are adjusted such that:
1. `System.ConnsInbound` >= `max(Swarm.ConnMgr.HighWater * 2, DefaultResourceMgrMinInboundConns)` AND
2. `System.StreamsInbound` is greater than any new/adjusted `Swarm.ResourceMgr.Limits.System.ConnsInbound` value so that there's enough streams per connection.

Source: [core/node/libp2p/rcmgr_defaults.go](https://github.com/ipfs/kubo/blob/master/core/node/libp2p/rcmgr_defaults.go)

### What are the "Application error 0x0 (remote) ... cannot reserve ..." messages?
These are messages coming from old (pre go-libp2p 0.26) *remote* go-libp2p peers (likely another older Kubo node) with the resource manager enabled on why it failed to establish a connection.  

This can be confusing, but these `Application error 0x0 (remote) ... cannot reserve ...` messages can occur even if your local node has the resource manager disabled.

You can distinguish resource manager messages originating from your local node if they're from the `resourcemanager` / `libp2p/rcmgr_logging.go` logger
or you see the string that is unique to Kubo (and not in go-libp2p): "Protected from exceeding resource limits".

See more info in this go-libp2p issue ([#1928](https://github.com/libp2p/go-libp2p/issues/1928)).  go-libp2p 0.26 / Kubo 0.19 onwards this confusing error message was removed.


## History
Kubo first [exposed this functionality in Kubo 0.13](./changelogs/v0.13.md#-libp2p-network-resource-manager-swarmresourcemgr), but it was disabled by default.  It was then enabled by default in [Kubo 0.17](./changelogs/v0.17.md#libp2p-resource-management-enabled-by-default).  Until that point, Kubo was vulnerable to unbound resource usage which could bring down nodes.  Introducing limits like this by default after the fact is tricky, which is why there have been changes and improvements afterwards.  The general trend since 0.17 with (0.18)[./changeloges/v0.18.md#improving-libp2p-resource-management-integration] and 0.19 has been to simplify and provide less options (and footguns!) for users and better documentation.
