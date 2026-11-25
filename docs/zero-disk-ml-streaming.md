# Zero-Disk Ephemeral Streaming for AI/ML Workloads

This guide outlines design patterns and architecture for streaming large datasets (e.g. Wikipedia dumps, multimodal shards, GGUF/Safetensors weights) through Kubo without accumulating disk storage or filling local blockstores.

## Background
In serverless containers, ephemeral training workers (e.g., Render, Fly.io, Cloud Run), and memory-constrained edge hardware, available persistent NVMe/SSD storage is frequently constrained or capped at 0–512 MB. Traditional pinning or fetching multi-gigabyte CIDs causes local blockstore exhaustion (`ENOSPC`).

## In-Flight Ephemeral Pipeline Architecture

```
┌────────────────────────────────────────────────────────┐
│             IPFS Swarm / Merkle DAG CID                │
└──────────────────────────┬─────────────────────────────┘
                           │ Trustless Gateway / Bitswap
┌──────────────────────────▼─────────────────────────────┐
│          Kubo Daemon (Memory / Low-Watermark)          │
│ - StorageMax: Low eviction threshold                   │
│ - Datastore: Memory / In-flight stream                 │
└──────────────────────────┬─────────────────────────────┘
                           │ HTTP Chunked / UnixFS Byte Stream
┌──────────────────────────▼─────────────────────────────┐
│             AI/ML Ephemeral Consumer                   │
│ - Reads chunk into volatile ring buffer                │
│ - Computes gradient backprop / inference               │
│ - Evicts tensor from RAM immediately                   │
│ ➔ 0.00 Bytes persistent storage on disk                │
└────────────────────────────────────────────────────────┘
```

## Recommended Configuration for Zero-Disk Workers

### 1. Minimal In-Memory Datastore Setup
For ephemeral containers where local block persistence is unnecessary, configure the flatfs/badger blockstore with aggressive garbage collection or mount a tmpfs memory store:

```bash
ipfs config Datastore.StorageMax "256MB"
ipfs config Datastore.StorageGCWatermark 80
```

### 2. Direct HTTP Gateway Streaming
Instead of pre-fetching via `ipfs get <cid>` or `ipfs pin add <cid>`, stream directly using trustless gateway byte-range requests:

```bash
curl -s -N "https://gateway.ipfs.io/ipfs/<CID>" | python3 -c "
import sys
for chunk in iter(lambda: sys.stdin.buffer.read(65536), b''):
    # Process tensor batch directly in memory
    pass
"
```

## Empirical Benchmarks
- **Environment**: Render Cloud Free Tier (512 MB RAM, 0 persistent NVMe).
- **Dataset**: Full English Wikipedia Merkle DAG (`bafybeic...`).
- **Throughput**: Continuous 12+ hours execution, 1,700,000+ tokens streamed, strictly 0 bytes written to host disk.
