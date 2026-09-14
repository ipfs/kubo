# Kubo on low-memory devices

This guide is for running the Kubo daemon on devices with limited RAM, such as a Raspberry Pi or Orange Pi with 8 GiB, managed by systemd. It covers three layers that work together: the Go runtime, the systemd unit, and Kubo configuration. Set all three. Each layer catches what the previous one cannot. The same principles apply to other orchestration: for example, in Docker set `GOMEMLIMIT` via `--env` and replace the systemd limits with `--memory` and `--memory-reservation`.

## Quick start (8 GiB device)

Add memory limits to the service:

```console
$ sudo systemctl edit ipfs
```

```ini
[Service]
Environment=GOMEMLIMIT=4GiB
MemoryHigh=6G
MemoryMax=7G
Restart=on-failure
```

Cap concurrent DHT announcement work, and turn off the services that spend resources on other peers:

```console
$ ipfs config --json Provide.DHT.MaxWorkers 4
$ ipfs config Routing.Type autoclient
$ ipfs config AutoNAT.ServiceMode disabled
$ ipfs config --json Swarm.RelayService.Enabled false
```

Then reload and restart:

```console
$ sudo systemctl daemon-reload
$ sudo systemctl restart ipfs
```

If the node provides millions of CIDs, also read [Announcements on a large pinset](#announcements-on-a-large-pinset) below.

## How the layers fit together

Keep the values in this order, with room between each pair:

```
GOMEMLIMIT  <  MemoryHigh  <  MemoryMax  <  total RAM
```

### Go runtime: `GOMEMLIMIT`

`GOMEMLIMIT` is a soft ceiling for the memory managed by the Go runtime. As usage approaches the limit, the garbage collector runs more often and returns free memory to the OS sooner. Set it to about half of total RAM.

The limit is soft: memory that is still in use cannot be freed, so the process can exceed `GOMEMLIMIT` under load. That is why the systemd limits below are still needed. See the [Go garbage collector guide](https://go.dev/doc/gc-guide) for how the limit works.

For a systemd service, set it with `Environment=GOMEMLIMIT=4GiB` in the unit (or a `systemctl edit` drop-in, as in the quick start). For manual runs, export it in the shell before starting the daemon.

### systemd: `MemoryHigh`, `MemoryMax`, swap

`MemoryHigh` is the throttling threshold: above it, the kernel slows the service down and reclaims memory from it. `MemoryMax` is the hard limit: above it, the kernel kills the service.

Two rules for picking the values:

- Leave a gap between `GOMEMLIMIT` and `MemoryHigh`. The service uses memory the Go runtime does not manage: thread stacks, and the OS page cache for repo files, which the kernel accounts to the service. If `MemoryHigh` sits at or below `GOMEMLIMIT`, the kernel throttles the daemon during normal operation.
- On a device without swap, keep `MemoryMax` close above `MemoryHigh`. Without swap the kernel cannot reclaim the daemon's working memory, so a process stuck above `MemoryHigh` is throttled to a crawl and can stay there indefinitely: it drops peers and stops responding, but never recovers. Crossing `MemoryMax` instead gets a clean kill, and `Restart=on-failure` brings the daemon back within seconds. A fast restart beats a slow zombie.

If the device has swap, add `MemorySwapMax=0` to keep the daemon from pushing the rest of the system into swap.

### Kubo configuration

Run the node as a DHT client. With the default [`Routing.Type`](../config.md#routingtype) of `auto`, a publicly reachable node becomes a DHT server: it stores other peers' provider records in its datastore and answers their lookups, which adds network, CPU, and storage load unrelated to your own content. Setting `Routing.Type` to `autoclient` (or `dhtclient`) removes that duty; the node still announces its own content and retrieves normally.

Two more services exist to help other peers and can go on constrained hardware: [`AutoNAT.ServiceMode`](../config.md#autonatservicemode) set to `disabled` stops answering other peers' reachability dial-back requests, and [`Swarm.RelayService.Enabled`](../config.md#swarmrelayserviceenabled) set to `false` stops relaying traffic between third parties. Neither affects this node's own connectivity: it still uses AutoNAT as a client and public relays when it needs them.

On a node that announces a lot of content, the DHT announcement subsystem is the main memory consumer that Kubo configuration controls. [`Provide.DHT.MaxWorkers`](../config.md#providedhtmaxworkers) caps its concurrency; the next section explains why this matters on low-memory devices.

Leave [`Swarm.ResourceMgr.MaxMemory`](../config.md#swarmresourcemgrmaxmemory) unset unless you know what you are doing. It does not limit the Kubo process; it scales the [libp2p resource manager](../libp2p-resource-management.md) limits, including how many inbound connections the node accepts. Setting it too low cripples connectivity: the daemon keeps running and looks online, but the resource manager refuses new connections with `cannot reserve inbound connection: resource limit exceeded` errors, logged as "Protected from exceeding resource limits". Inspect current usage against the limits with `ipfs swarm resources`.

## Announcements on a large pinset

Keep [`Provide.DHT.SweepEnabled`](../config.md#providedhtsweepenabled) at its default `true` on low-power devices. Sweep mode spreads announcement work smoothly across the reprovide cycle instead of completing it in bursts, which avoids the resource spikes of the legacy provider.

With sweep mode, the daemon announces content region by region across the DHT keyspace, and each active worker holds one region's keys in memory while it works. After downtime, many regions are due at once and the daemon puts every worker to work, so peak memory scales with [`Provide.DHT.MaxWorkers`](../config.md#providedhtmaxworkers) times the region size. On a node providing millions of CIDs, a high worker count can consume several GiB during this catch-up phase; `4` workers keep the peak small on an 8 GiB device.

Size the workload with two rates. The sustained rate is what your node actually announces: read `Total CIDs provided` from `ipfs stats provide` twice, 24 hours apart. As a reference, an 8 GiB board with NVMe storage and 4 workers sustains roughly 300k records per hour. The required rate is your CID count divided by [`Provide.DHT.Interval`](../config.md#providedhtinterval) (default 22h). Keep the required rate below the sustained rate, with margin. The hard floor is record expiry: provider records on the Amino DHT expire after 48 hours ([`amino.DefaultProvideValidity`](https://github.com/libp2p/go-libp2p-kad-dht/blob/v0.34.0/amino/defaults.go#L40-L43)), so a full pass over the pinset must always complete faster than that, or some content periodically becomes undiscoverable.

Worked example, 10 million CIDs on an 8 GiB device: the default 22h interval requires ~455k records per hour, above the reference rate, so the cycle slips. Raising `Provide.DHT.MaxWorkers` to `6` and `Provide.DHT.Interval` to `32h` lowers the requirement to ~313k per hour, within reach of the sustained rate, and a full pass stays well inside the 48h expiry (the floor for 10M CIDs is ~208k per hour). After a change, let a full cycle finish and use the [Verify](#verify) checks to confirm the reprovide queue is not growing.

Do not enable [`Routing.AcceleratedDHTClient`](../config.md#routingaccelerateddhtclient) to speed announcements up on a low-memory device. It crawls the entire DHT on an hourly schedule, and those crawls cause the memory and traffic spikes this guide exists to prevent. Adjust workers and interval as above instead.

The number of announced CIDs itself is the biggest lever. [`Provide.Strategy`](../config.md#providestrategy) selects what gets announced: `pinned+mfs+entities` announces file and directory roots instead of every block, which typically shrinks the workload by orders of magnitude. Read the trade-offs in the strategy documentation before changing it.

## Verify

Check where the service sits relative to its limits:

```console
$ systemctl status ipfs | grep Memory
```

Check memory pressure. The `avg10` values show the percentage of time the service was stalled waiting for memory over the last 10 seconds. Near-zero is healthy; sustained values above a few percent mean the limits are too tight or the workload is too big:

```console
$ cat /sys/fs/cgroup/system.slice/ipfs.service/memory.pressure
```

Check that announcements keep up with the reprovide cycle:

```console
$ ipfs stats provide
```

See [provide-stats.md](../provide-stats.md) for reading this output, and [libp2p resource management](../libp2p-resource-management.md) for the resource manager's own limits.
