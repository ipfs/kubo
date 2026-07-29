# Telemetry

The telemetry plugin sends anonymized usage data about a Kubo node to an HTTP endpoint. It tells Kubo maintainers which features are actually used, so work goes where it helps.

Telemetry is enabled by default and reports to `https://telemetry.ipshipyard.dev`. Each report carries a random identifier, coarse buckets, and configuration flags. It never carries personal data, file names, content identifiers, or peer addresses. The first report is sent 15 minutes after the daemon starts, then once a day.

You can turn it off at any time, and a build of Kubo can ship with the endpoint removed. See [Disabling telemetry](#disabling-telemetry).

**Table of Contents**

- [Disabling telemetry](#disabling-telemetry)
  - [For one daemon run](#for-one-daemon-run)
  - [For a node](#for-a-node)
  - [For every tool on the machine](#for-every-tool-on-the-machine)
  - [When building Kubo yourself](#when-building-kubo-yourself)
- [Sending to your own collector](#sending-to-your-own-collector)
  - [Modes](#modes)
  - [Configuration](#configuration)
- [Endpoint API](#endpoint-api)
  - [Payload](#payload)
  - [Retiring an endpoint](#retiring-an-endpoint)
  - [Running your own collector](#running-your-own-collector)
- [Data collected](#data-collected)
- [Privacy](#privacy)
- [Testing locally](#testing-locally)
- [See also](#see-also)

## Disabling telemetry

Any of the options below stops the node from sending. Each takes effect on the next daemon start.

### For one daemon run

Set [`IPFS_TELEMETRY`](environment-variables.md#ipfs_telemetry) to `off`:

```bash
export IPFS_TELEMETRY=off
ipfs daemon
```

### For a node

Set the mode in the config file, then restart the daemon:

```bash
ipfs config Plugins.Plugins.telemetry.Config.Mode off
```

To go further and keep the plugin from loading at all:

```bash
ipfs config --json Plugins.Plugins.telemetry.Disabled true
```

### For every tool on the machine

Kubo honors [`DO_NOT_TRACK`](environment-variables.md#do_not_track), the shared opt-out convention other command line tools read as well:

```bash
export DO_NOT_TRACK=1
```

`IPFS_TELEMETRY` wins over `DO_NOT_TRACK`, so a machine that opts out globally can still keep telemetry on for one node with `IPFS_TELEMETRY=on`.

### When building Kubo yourself

The built-in endpoint lives in one linker-settable variable, so you can build a Kubo that has no telemetry destination at all. Blank it at build time:

```bash
go build -ldflags "-X github.com/ipfs/kubo/plugin/plugins/telemetry.defaultEndpoint=" -o ipfs ./cmd/ipfs
```

A binary built this way collects nothing, writes no telemetry identifier, and needs no configuration from the people running it. Distributors and packagers who do not want their users reporting to the Kubo maintainers should use this rather than patching source. The operator of such a build can still point it at their own collector with `Endpoint`.

## Sending to your own collector

Set `Endpoint` to your own URL to report there instead of the built-in destination:

```json
{
  "Plugins": {
    "Plugins": {
      "telemetry": {
        "Config": {
          "Endpoint": "https://telemetry.example.com"
        }
      }
    }
  }
}
```

### Modes

The mode comes from [`IPFS_TELEMETRY`](environment-variables.md#ipfs_telemetry) first, then [`DO_NOT_TRACK`](environment-variables.md#do_not_track), then `Plugins.Plugins.telemetry.Config.Mode`.

| Mode   | Description                                                                                             |
|--------|---------------------------------------------------------------------------------------------------------|
| `auto` | Default when the mode is unset. Telemetry is enabled, and a node shows the startup notice on its first run. |
| `on`   | Telemetry is enabled.                                                                                    |
| `off`  | Telemetry is disabled. Nothing is sent, and the stored identifier is removed.                            |

### Configuration

| Key        | Type   | Default                            | Description                                                                              |
|------------|--------|------------------------------------|--------------------------------------------------------------------------------------------|
| `Mode`     | string | `auto`                             | `auto`, `on` or `off`. See [Modes](#modes).                                              |
| `Endpoint` | string | `https://telemetry.ipshipyard.dev` | URL the node sends telemetry to.                                                          |
| `Delay`    | string | `15m`                              | How long to wait after daemon start before the first send. Accepts a Go duration string. |

## Endpoint API

When enabled, the node sends one request per collection to the `Endpoint`:

- Method: `POST`
- Headers: `Content-Type: application/json`, plus a `User-Agent` carrying the Kubo version
- Body: a single JSON object (see [Payload](#payload))
- Response: any `2xx` is success. A status of `400` or higher is a failure; the node logs it and retries on the next interval. `410` is special, see [Retiring an endpoint](#retiring-an-endpoint).

The first request is sent `Delay` after the daemon starts (15 minutes by default), then once every 24 hours while the daemon runs. Requests go through `HTTP_PROXY`, `HTTPS_PROXY` and `NO_PROXY` when those are set.

### Payload

The body is the `LogEvent` struct defined in [`plugin/plugins/telemetry/telemetry.go`](https://github.com/ipfs/kubo/blob/master/plugin/plugins/telemetry/telemetry.go). That struct is the source of truth for the fields; this page can fall behind it, so check the code for the current set.

Example:

```json
{
  "uuid": "f81d4fae-7dec-11d0-a765-00a0c91e6bf6",
  "agent_version": "kubo/0.43.0/",
  "private_network": false,
  "bootstrappers_custom": false,
  "repo_size_bucket": 5368709120,
  "uptime_bucket": 86400000000000,
  "reprovider_strategy": "all",
  "provide_dht_sweep_enabled": false,
  "provide_dht_interval_custom": false,
  "provide_dht_max_workers_custom": false,
  "routing_type": "auto",
  "routing_accelerated_dht_client": false,
  "routing_delegated_count": 0,
  "autonat_service_mode": "enabled",
  "autonat_reachability": "Public",
  "swarm_enable_hole_punching": true,
  "swarm_circuit_addresses": true,
  "swarm_ipv4_public_addresses": true,
  "swarm_ipv6_public_addresses": false,
  "auto_tls_auto_wss": true,
  "auto_tls_domain_suffix_custom": false,
  "autoconf": true,
  "autoconf_custom": false,
  "discovery_mdns_enabled": true,
  "platform_os": "linux",
  "platform_arch": "amd64",
  "platform_containerized": false,
  "platform_vm": false
}
```

`repo_size_bucket` is an upper bound in bytes and `uptime_bucket` is an upper bound in nanoseconds. Both are coarse buckets rather than exact values (see [Privacy](#privacy)).

### Retiring an endpoint

A collector that answers `410 Gone` is telling nodes to stop.[^rfc9110] A node that gets a `410` writes a `telemetry_retired` file in its repo, removes its `telemetry_uuid`, and sends nothing further to that endpoint, on this run or any later one.

This is how telemetry gets switched off across nodes that are already running, without waiting for anyone to upgrade. It works the same for third-party collectors: answer `410` when you retire yours, so the nodes pointed at it stop rather than retry forever.

The marker holds the endpoint it applies to. Pointing the node at a different `Endpoint`, or deleting the file, starts reporting again.

[^rfc9110]: [RFC 9110, section 15.5.11](https://httpwg.org/specs/rfc9110.html#status.410): a `410` means the resource is intentionally unavailable, the condition is likely permanent, and the server owners want remote references removed.

### Running your own collector

A collector is any HTTP service that accepts the `POST` above and replies with a `2xx`. Parse the JSON body and keep the fields you need. Point one or more nodes at it through their `Endpoint`, and use the per-node `uuid` to group repeated reports from the same node.

## Data collected

Each report carries the fields shown in the [Payload example](#payload): the Kubo version, coarse buckets for repository size and uptime, booleans and enums describing the node's routing, providing, network, and discovery configuration, and basic platform facts such as OS and architecture. No file names, content identifiers, or peer addresses are included.

To see the exact data your node would send, set `GOLOG_LOG_LEVEL="telemetry=debug"`.

## Privacy

- **Anonymized**: no personally identifiable information is sent. Sizes and uptimes are reported as coarse buckets, not exact values.
- **Announced**: the first run of a node prints a notice naming the endpoint and the ways to opt out, 15 minutes before anything is sent.
- **Yours to turn off**: see [Disabling telemetry](#disabling-telemetry). Opting out removes the stored identifier.

The telemetry identifier (`uuid`) is stored in the IPFS repo directory and identifies the node across runs while telemetry is enabled. It holds no personal information.

## Testing locally

To capture and inspect telemetry on your own machine, run a small HTTP server and point the endpoint at it:

```json
{
  "Plugins": {
    "Plugins": {
      "telemetry": {
        "Config": {
          "Mode": "on",
          "Endpoint": "http://localhost:9099",
          "Delay": "5s"
        }
      }
    }
  }
}
```

The short `Delay` sends the first report a few seconds after startup instead of after the default 15 minutes.

## See also

- [Kubo environment variables](environment-variables.md)
- [Plugins](plugins.md)
- [Kubo configuration](config.md)
