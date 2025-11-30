# Provide Stats

The `ipfs provide stat` command gives you statistics about your local provide
system. This file provides a detailed explanation of the metrics reported by
this command.

## Understanding the Metrics

The statistics are organized into three types of measurements:

### Per-worker rates

Metrics like "CIDs reprovided/min/worker" measure the throughput of a single
worker processing one region. To estimate total system throughput, multiply by
the number of active workers of that type (see [Workers stats](#workers-stats)).

Example: If "CIDs reprovided/min/worker" shows 100 and you have 10 active
periodic workers, your total reprovide throughput is approximately 1,000
CIDs/min.

### Per-region averages

Metrics like "Avg CIDs/reprovide" measure properties of the work units (keyspace
regions). These represent the average size or characteristics of a region, not a
rate. Do NOT multiply these by worker count.

Example: "Avg CIDs/reprovide: 250,000" means each region contains an average of
250,000 CIDs that get reprovided together as a batch.

### System totals

Metrics like "Total CIDs provided" are cumulative counts since node startup.
These aggregate all work across all workers over time.

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
prefix length across all regions in the schedule. Longer prefixes indicate the
keyspace is divided into more regions (because there are more DHT servers in the
swarm to distribute records across).

### Next region prefix

Keyspace prefix of the next region to be reprovided.

### Next region reprovide

When the next region is scheduled to be reprovided.

## Timings

### Uptime

How long the provide system has been running since Kubo started, along with the
start timestamp.

### Current time offset

Elapsed time in the current reprovide cycle, showing cycle progress (e.g., '11h'
means 11 hours into a 22-hour cycle, roughly halfway through).

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

Note: this counts successful sends; some DHT servers may have gone offline
afterward, so actual availability may be lower.

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

### CIDs provided/min/worker

Average rate of initial provides per minute per worker during the last
reprovide cycle (excludes reprovides). Each worker handles one keyspace region
at a time, providing all CIDs in that region. This measures the throughput of a
single worker only.

To estimate total system provide throughput, multiply by the number of active
burst workers shown in [Workers stats](#workers-stats) (Burst > Active).

Note: This rate only counts active time when initial provides are being
processed. If workers are idle, actual throughput may be lower.

### CIDs reprovided/min/worker

Average rate of reprovides per minute per worker during the last reprovide
cycle (excludes initial provides). Each worker handles one keyspace region at a
time, reproviding all CIDs in that region. This measures the throughput of a
single worker only.

To estimate total system reprovide throughput, multiply by the number of active
periodic workers shown in [Workers stats](#workers-stats) (Periodic > Active).

Example: If this shows 100 CIDs/min and you have 10 active periodic workers,
your total reprovide throughput is approximately 1,000 CIDs/min.

Note: This rate only counts active time when regions are being reprovided. If
workers are idle due to network issues or queue exhaustion, actual throughput
may be lower.

### Region reprovide duration

Average time to reprovide all CIDs in a region during the last cycle.

### Avg CIDs/reprovide

Average number of CIDs per region during the last reprovide cycle.

This measures the average size of a region (how many CIDs are batched together),
not a throughput rate. Do NOT multiply this by worker count.

Combined with [Region reprovide duration](#region-reprovide-duration), this
helps estimate per-worker throughput: dividing Avg CIDs/reprovide by Region
reprovide duration gives CIDs/min/worker.

### Regions reprovided (last cycle)

Number of regions reprovided in the last cycle.

> [!NOTE]
> (⚠️ 0.39 limitation) If this shows 1 region while using
> [`Routing.AcceleratedDHTClient`](./config.md#routingaccelerateddhtclient), sweep mode lost
> efficiency gains. Consider disabling the accelerated client. See [caveat 4](./config.md#routingaccelerateddhtclient).

## Workers

### Active workers

Number of workers currently processing provide or reprovide operations.

### Free workers

Number of idle workers not reserved for periodic or burst tasks.

### Workers stats

Breakdown of worker status by type (periodic for scheduled reprovides, burst for
initial provides). For each type:

- **Active**: Currently processing operations (use this count when calculating total throughput from per-worker rates)
- **Dedicated**: Reserved for this type
- **Available**: Idle dedicated workers + [free workers](#free-workers)
- **Queued**: 0 or 1 (workers acquired only when needed)

The number of active workers determines your total system throughput. For
example, if you have 10 active periodic workers, multiply
[CIDs reprovided/min/worker](#cids-reprovidedminworker) by 10 to estimate total
reprovide throughput.

See [provide queue](#provide-queue) and [reprovide queue](#reprovide-queue) for
regions waiting to be processed.

### Max connections/worker

Maximum concurrent DHT server connections per worker when sending provider
records for a region.

## Capacity Planning

### Estimating if your system can keep up with the reprovide schedule

To check if your provide system has sufficient capacity:

1. Calculate required throughput:
   - Required CIDs/min = [CIDs scheduled](#cids-scheduled) / ([Reprovide interval](#reprovide-interval) in minutes)
   - Example: 67M CIDs / (22 hours × 60 min) = 50,758 CIDs/min needed

2. Calculate actual throughput:
   - Actual CIDs/min = [CIDs reprovided/min/worker](#cids-reprovidedminworker) × Active periodic workers
   - Example: 100 CIDs/min/worker × 256 active workers = 25,600 CIDs/min

3. Compare:
   - If actual < required: System is underprovisioned, increase [MaxWorkers](./config.md#providedhtmaxworkers) or [DedicatedPeriodicWorkers](./config.md#providedhtdedicatedperiodicworkers)
   - If actual > required: System has excess capacity
   - If [Reprovide queue](#reprovide-queue) is growing: System is falling behind

### Understanding worker utilization

- High active workers with growing reprovide queue: Need more workers or network connectivity is limiting throughput
- Low active workers with non-empty reprovide queue: Workers may be waiting for network or DHT operations
- Check [Reachable peers](#reachable-peers) to diagnose network connectivity issues
- (⚠️ 0.39 limitation) If [Regions scheduled](#regions-scheduled) shows 1 while using
  [`Routing.AcceleratedDHTClient`](./config.md#routingaccelerateddhtclient), consider disabling
  the accelerated client to restore sweep efficiency. See [caveat 4](./config.md#routingaccelerateddhtclient).

## See Also

- [Provide configuration reference](./config.md#provide)
- [Provide metrics for Prometheus](./metrics.md#provide)
