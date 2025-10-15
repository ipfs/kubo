# Provide Stats

The `ipfs provide stat` command gives you statistics about your local provide
system. This file provides a detailed explanation of the metrics reported by
this command.

## Connectivity

### Status

Provides the node's current connectivity status: `online`, `disconnected`, or
`offline`. A node is considered `disconnected`, if it has been recently online
in the last
[`Provide.DHT.OfflineDelay`](https://github.com/ipfs/kubo/blob/master/docs/config.md#providedhtofflinedelay).

It also contains the timestamp of the last status change.

## Queues

### Provide queue

Display the number of CIDs in the provide queue. In the queue, CIDs are grouped
by keyspace region. Also displays the number of keyspace regions in the queue.

### Reprovide queue

Shows the number of regions that are waiting to be reprovided. The regions
sitting in the reprovide queue are late to reprovide, and will be reprovided as
soon as possible.

A high number of regions in the reprovide queue may indicate a few things:

1. The node is currently `disconnected` or `offline`, and the reprovides that
   should have executed are queued for when the node goes back `online`.
2. The node is currently processing the backlog of late reprovides after a
   restart or a period of being `disconnected` or `offline`.
3. The provide system cannot keep up with the rate of late reprovides, if the
   queue size keeps inscresing or doesn't decrease over time. This is usually
   due to a high number of CIDs to be reprovided and a too low number of
   (periodic) workers. Consider increasing the [`Provide.DHT.MaxWorkers`]() and
   [`Provide.DHT.DedicatedPeriodicWorkers`]().

## Schedule

### CIDs scheduled

Number of CIDs scheduled to be reprovided.

### Regions scheduled

Number of keyspace regions scheduled to be reprovided. Each CID is mapped to a
region, and reprovided as a batch with the other CIDs in the same region.

### Avg prefix length

Average prefix length of the scheduled regions. This is an indicator of the
number of DHT servers in the swarm.

### Next reprovide at

Timestamp of the next scheduled region reprovide.

### Next prefix

Next region prefix to be reprovided.

## Timings

### Uptime

Uptime of the provide system, since Kubo was started. Also includes the
timestamp at which the provide system was started.

### Current time offset

Time offset in the current reprovide cycle. This metrics shows the progression
of the provide cycle.

### Cycle started

Timestamp of when the current reprovide cycle started.

### Reprovide interval

Duration of the reprovide cycle. This is the interval at which all CIDs are
reprovided.

## Network

### Avg record holders

Each CID is sent to multiple DHT servers in the swarm. This metric shows the
average number of DHT servers that have been sent each CID. If this number
matches the [Replication factor](#replication-factor), it means that all CIDs
have been sent to the desired number of DHT servers.

A lower number indicates that a proportion of the DHT network either isn't
reachable, timed out during the `ADD_PROVIDER` RPC, or doesn't support storing
provider records.

Note that this metric only displays the number of replicas that were sent
successfully. It is possible that some of the records holders have gone
offline, and the actual number of nodes storing the provider records may be
lower.

### Peers swept

Number of DHT servers that were contacted during the last reprovide sweep
cycle. This doesn't include peers that were contacted during the initial
provide of a CID, not during a DHT lookup. It only includes peers that we tried
to send a provider record to.

If providing CIDs to all keyspace regions (very likely beyond a certain number
of CIDs), this number is expected to grow during the initial reprovide cycle
(up to [`reprovide interval`](#reprovide-interval) after the node started).
After that, the number is expected to stabilize and show the actual size of the
DHT swarm.

If providing a small number of CIDs, this number will be lower than the network
size, since it only considers the peers to which we sent provider records.

### Full keyspace coverage

Boolean value indicating whether the reprovide sweep has covered all the DHT
servers in the swarm or not. It `true` it means that the node has sent provider
records to all DHT servers in the swarm during the last reprovide cycle.

It means that [`Peers swept`](#peers-swept) is an approximation of the DHT
swarm size over the last [`Reprovide Interval`](#reprovide-interval).

### Reachable peers

Number of reachable peers among the [`Peers swept`](#peers-swept). A reachable
peer is a peer that successfully responded to all the `ADD_PROVIDER` RPC that
we sent during the last reprovide cycle.

Also includes the percentage of reachable peers among the [`Peers swept`](#peers-swept).

### Avg region size

Average number of DHT servers in each keyspace region.

### Replication factor

Number of DHT servers to which we send a provider record for each CID.

## Operations

### Ongoing provides

Number of CIDs that are currently being provided for the first time. Also shows
the number of keyspace regions to which these CIDs belong.

Having a higher number of CIDs than number of regions indicates that regions
contain multiple CIDs, which is a sign of efficient batching.

Each keyspace region corresponds to a [burst worker]().

### Ongoing reprovides

Number of CIDs and keyspace regions that are currently being reprovided.

Each region corresponds to a [periodic worker]().

### Total CIDs provided

Number of (non-distinct) CIDs that have been provided since the node started.
Also includes reprovides.

### Total records provided

Number of provider records that have successfully been sent to DHT servers
since the node started. Also includes reprovides.

### Total provide errors

Number of regions that have failed to be provided since the node started. Also
includes reprovides. Upon failure, the provide system will retry providing the
failed region, unless it is `disconnected` or `offline`, in which case the
retry happens when the node goes back `online`.

### CIDs provided/min

Average number of CIDs provided (excluding reprovides) per minute of provide
operation running in the last reprovide cycle.

### CIDs reprovided/min

Average number of CIDs reprovided (excluding initial provide) per minute of
reprovide operation running in the last reprovide cycle.

### Region reprovide duration

Average duration it took to reprovide all the CIDs in a keyspace region during
the last reprovide cycle.

### Avg CIDs/reprovide

Average number of CIDs that were reprovided in each keyspace region during the
last reprovide cycle.

### Regions reprovided (last cycle)

Number of keyspace regions that were reprovided during the last reprovide cycle.

## Workers

### Active workers

Number of workers that are currently active, either processing a provide or a
reprovide operation.

### Free workers

Number of workers that are currently idle, and not reserved for a specific task
(periodic or burst).

### Workers stats

For each kind of worker (periodic and burst), shows the number of active,
dedicated, available and queued workers. The number of available workers is the
sum of idle dedicated workers and [`free workers`](#free-workers). The number
of queued workers can be either `0` or `1` since we don't try to take a new
worker until we successfully can take one. You can look at the [`Provide
queue`](#provide-queue) and [`Reprovide queue`](#reprovide-queue) to see how
many regions are waiting to be processed, which corresponds to queued workers.

### Max connections/worker

Maximum number of connections to DHT servers per worker when sending out
proivder records for a region.
