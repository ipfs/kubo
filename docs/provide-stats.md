# Provide Stats

The `ipfs provide stat` command gives you statistics about your local provide
system. This file provides a detailed explanation of the metrics reported by
this command.

## Connectivity

### Status

Current connectivity status (`online`, `disconnected`, or `offline`) and when
it last changed (see [provide connectivity
status](./config.md#providedhtofflinedelay)).

## Queues

### Provide queue

Number of CIDs waiting for initial provide, and the number of keyspace regions
they're grouped into.

### Reprovide queue

Number of regions with overdue reprovides. These regions missed their scheduled
reprovide time and will be processed as soon as possible. If decreasing, the
node is recovering from downtime. If increasing, either the node is offline or
the provide system needs more workers (see
[`Provide.DHT.MaxWorkers`](./config.md#providedhtmaxworkers)
and
[`Provide.DHT.DedicatedPeriodicWorkers`](./config.md#providedhtdedicatedperiodicworkers)).

## Schedule

### CIDs scheduled

Total CIDs scheduled for reprovide.

### Regions scheduled

Number of keyspace regions scheduled for reprovide. Each CID is mapped to a
specific region, and all CIDs within the same region are reprovided together as
a batch for efficient processing.

### Avg prefix length

Average length of binary prefixes identifying the scheduled regions. Each
keyspace region is identified by a binary prefix, and this shows the average
prefix length across all regions in the schedule. Longer prefixes indicate more
DHT servers in the swarm.

### Next region prefix

Keyspace prefix of the next region to be reprovided.

### Next region reprovide

When the next region is scheduled to be reprovided.

## Timings

### Uptime

How long the provide system has been running since Kubo started, along with the
start timestamp.

### Current time offset

Elapsed time in the current reprovide cycle, showing cycle progress.

### Cycle started

When the current reprovide cycle began.

### Reprovide interval

How often each CID is reprovided (the complete cycle duration).

## Network

### Avg record holders

Average number of provider records successfully sent for each CID to distinct
DHT servers. In practice, this is often lower than the [replication
factor](#replication-factor) due to unreachable peers or timeouts. Matching the
replication factor would indicate all DHT servers are reachable.

Note: some holders may have gone offline since receiving the record.

### Peers swept

Number of DHT servers to which we tried to send provider records in the last
reprovide cycle (sweep). Excludes peers contacted during initial provides or
DHT lookups.

### Full keyspace coverage

Whether provider records were sent to all DHT servers in the swarm during the
last reprovide cycle. If true, [peers swept](#peers-swept) approximates the
total DHT swarm size over the last [reprovide interval](#reprovide-interval).

### Reachable peers

Number and percentage of peers to which we successfully sent all provider
records assigned to them during the last reprovide cycle.

### Avg region size

Average number of DHT servers per keyspace region.

### Replication factor

Target number of DHT servers to receive each provider record.

## Operations

### Ongoing provides

Number of CIDs and regions currently being provided for the first time. More
CIDs than regions indicates efficient batching. Each region provide uses a
[burst
worker](./config.md#providedhtdedicatedburstworkers).

### Ongoing reprovides

Number of CIDs and regions currently being reprovided. Each region reprovide
uses a [periodic
worker](./config.md#providedhtdedicatedperiodicworkers).

### Total CIDs provided

Total number of provide operations since node startup (includes both provides
and reprovides).

### Total records provided

Total provider records successfully sent to DHT servers since startup (includes
reprovides).

### Total provide errors

Number of failed region provide/reprovide operations since startup. Failed
regions are automatically retried unless the node is offline.

### CIDs provided/min

Average rate of initial provides per minute during the last reprovide cycle
(excludes reprovides).

### CIDs reprovided/min

Average rate of reprovides per minute during the last reprovide cycle (excludes
initial provides).

### Region reprovide duration

Average time to reprovide all CIDs in a region during the last cycle.

### Avg CIDs/reprovide

Average number of CIDs per region during the last reprovide cycle.

### Regions reprovided (last cycle)

Number of regions reprovided in the last cycle.

## Workers

### Active workers

Number of workers currently processing provide or reprovide operations.

### Free workers

Number of idle workers not reserved for periodic or burst tasks.

### Workers stats

Breakdown of worker status by type (periodic for scheduled reprovides, burst
for initial provides). For each: active (currently processing), dedicated
(reserved for this type), available (idle dedicated + [free
workers](#free-workers)), and queued (0 or 1, since we only acquire when
needed). See [provide queue](#provide-queue) and [reprovide
queue](#reprovide-queue) for regions waiting to be processed.

### Max connections/worker

Maximum concurrent DHT server connections per worker when sending provider
records for a region.
