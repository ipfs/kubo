# Telemetry

The telemetry plugin can send anonymized usage data about a Kubo node to an HTTP endpoint. It helps operators and Kubo developers understand how Kubo is used in aggregate.

Telemetry is opt-in and disabled by default.

Kubo ships with no built-in endpoint and does not phone home: nothing is sent unless you enable telemetry and point it at a collector you control. The data never includes personally identifiable information.

**Table of Contents**

- [Enabling telemetry](#enabling-telemetry)
  - [Modes](#modes)
  - [Configuration](#configuration)
- [Endpoint API](#endpoint-api)
  - [Payload](#payload)
  - [Running your own collector](#running-your-own-collector)
- [Data collected](#data-collected)
- [Privacy](#privacy)
- [Testing locally](#testing-locally)
- [See also](#see-also)

## Enabling telemetry

Telemetry needs two settings:

- the mode, set with the [`IPFS_TELEMETRY`](environment-variables.md#ipfs_telemetry) environment variable or `Plugins.Plugins.telemetry.Config.Mode`
- the destination, set with `Plugins.Plugins.telemetry.Config.Endpoint`

`IPFS_TELEMETRY` overrides the config `Mode` when both are set. When telemetry is enabled without an `Endpoint`, the plugin logs a warning and sends nothing.

To turn telemetry on, set the mode to `on`, point the endpoint at a collector you run, and restart the daemon:

```json
{
  "Plugins": {
    "Plugins": {
      "telemetry": {
        "Config": {
          "Mode": "on",
          "Endpoint": "https://telemetry.example.com"
        }
      }
    }
  }
}
```

To turn it back off, set the mode to `off`, which also removes the stored node identifier:

```bash
export IPFS_TELEMETRY="off"
```

The same applies through `Plugins.Plugins.telemetry.Config.Mode` in the config file.

### Modes

| Mode  | Description                                                                                                   |
|-------|---------------------------------------------------------------------------------------------------------------|
| `off` | Default. Telemetry is disabled and nothing is sent. Any stored telemetry identifier is removed.               |
| `on`  | Telemetry is enabled and sent to the configured `Endpoint`. The startup notice is logged once, on first run.  |

`auto` is accepted as a legacy value and behaves like `off`.

### Configuration

| Key        | Type   | Default | Description                                                                                |
|------------|--------|---------|--------------------------------------------------------------------------------------------|
| `Mode`     | string | `off`   | `off` or `on`. See [Modes](#modes).                                                        |
| `Endpoint` | string | none    | URL the node sends telemetry to. Required when telemetry is enabled.                       |
| `Delay`    | string | `15m`   | How long to wait after daemon start before the first send. Accepts a Go duration string.   |

## Endpoint API

When enabled, the node sends one request per collection to the configured `Endpoint`:

- Method: `POST`
- Headers: `Content-Type: application/json`, plus a `User-Agent` carrying the Kubo version
- Body: a single JSON object (see [Payload](#payload))
- Response: any `2xx` is success. A status of `400` or higher is a failure; the node logs it and retries on the next interval.

The first request is sent `Delay` after the daemon starts (15 minutes by default), then once every 24 hours while the daemon runs.

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

### Running your own collector

A collector is any HTTP service that accepts the `POST` above and replies with a `2xx`. Parse the JSON body and keep the fields you need. Point one or more nodes at it through their `Endpoint`, and use the per-node `uuid` to group repeated reports from the same node.

## Data collected

Each report carries the fields shown in the [Payload example](#payload): the Kubo version, coarse buckets for repository size and uptime, booleans and enums describing the node's routing, providing, network, and discovery configuration, and basic platform facts such as OS and architecture. No file names, content identifiers, or peer addresses are included.

To see the exact data your node would send, set `GOLOG_LOG_LEVEL="telemetry=debug"`.

## Privacy

- **Anonymized**: no personally identifiable information is sent. Sizes and uptimes are reported as coarse buckets, not exact values.
- **Opt-in**: nothing is sent unless you enable telemetry and configure an endpoint.
- **Operator-controlled**: data goes only to the `Endpoint` you set, over whatever transport that URL uses.

The telemetry identifier (`uuid`) is stored in the IPFS repo directory and identifies the node across runs while telemetry is enabled. It holds no personal information. Setting the mode to `off` removes it.

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
