# Kubo changelog vFUTURE

<a href="https://ipshipyard.com/"><img align="right" src="https://github.com/user-attachments/assets/39ed3504-bb71-47f6-9bf8-cb9a1698f272" /></a>

This release was brought to you by the [Shipyard](https://ipshipyard.com/) team.

- [vFUTURE](#vfuture)

## vFUTURE

- [Overview](#overview)
- [🔦 Highlights](#-highlights)
  - [Experimental on-demand pinning](#experimental-on-demand-pinning)
- [📝 Changelog](#-changelog)
- [👨‍👩‍👧‍👦 Contributors](#-contributors)

### Overview

### 🔦 Highlights

#### Experimental on-demand pinning

Automatically pins content when DHT provider counts fall below a configurable
min, and unpins once they stay above a max for a grace period. Helps keep
critical data around without storing overly replicated CIDs. Same rough idea as
ipfs-cluster replication factors, except nobody coordinates the pinset; each node reacts to DHT counts locally.

The feature is gated behind `Experimental.OnDemandPinningEnabled` and described
in [ipfs/specs#532](https://github.com/ipfs/specs/pull/532).

```console
$ ipfs config --json Experimental.OnDemandPinningEnabled true
```

New CLI commands under `ipfs pin ondemand`:

- `add` register CIDs for on-demand pinning
- `rm` deregister and unpin
- `ls` list registered CIDs (use `--live` for DHT provider counts; errors if routing is unavailable; timed-out lookups show as unknown)

Design highlights:

- **Pin partitioning**: the checker marks its pins with the reserved name
  `kubo:on-demand` so it never removes a user pin.
- **Storage budget**: skips pinning when repo usage exceeds
  `StorageMax * StorageGCWatermark`.
- **Idle timeout**: DAG fetches timeout after 2 minutes without receiving new
  blocks, allowing large downloads while skipping dead records.
- **Provide after pin**: the checker publishes a DHT provider record after
  pinning so other peers can discover the content on this node.
- **Sybil limitation**: provider counts come from DHT queries, which are
  susceptible to Sybil manipulation. Documented as a known limitation.

Configuration at [`OnDemandPinning`](https://github.com/ipfs/kubo/blob/master/docs/config.md#ondemandpinning):

| Option | Default | Description |
|---|---|---|
| `OnDemandPinning.ReplicationTargetMin` | `5` | Pin when fewer than this many providers (excluding self) |
| `OnDemandPinning.ReplicationTargetMax` | `7` | Start unpin grace only above this many providers |
| `OnDemandPinning.CheckInterval` | `"10m"` | Sweep period; failed CIDs back off up to 72h |
| `OnDemandPinning.UnpinGracePeriod` | `"72h"` | How long above max before unpinning (longer than 48h DHT record validity; plus up to `2 * CheckInterval` jitter) |
| `OnDemandPinning.DryRun` | `false` | Log/record pin/unpin decisions without changing the pinset |

See [experimental features](https://github.com/ipfs/kubo/blob/master/docs/experimental-features.md#on-demand-pinning)
for full documentation.

### 📝 Changelog

### 👨‍👩‍👧‍👦 Contributors
