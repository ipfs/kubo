## Kubo metrics

By default, a Prometheus endpoint is exposed by Kubo at `http://127.0.0.1:5001/debug/metrics/prometheus`.

It includes default [Prometheus Go client metrics](https://prometheus.io/docs/guides/go-application/) + Kubo-specific metrics listed below.

### Table of Contents

- [DHT RPC](#dht-rpc)
  - [Inbound RPC metrics](#inbound-rpc-metrics)
  - [Outbound RPC metrics](#outbound-rpc-metrics)
- [Provide](#provide)
  - [Legacy Provider](#legacy-provider)
  - [DHT Provider](#dht-provider)
- [Gateway (`boxo/gateway`)](#gateway-boxogateway)
  - [HTTP metrics](#http-metrics)
  - [Blockstore cache metrics](#blockstore-cache-metrics)
  - [Backend metrics](#backend-metrics)

> [!WARNING]
> This documentation is incomplete. For an up-to-date list of metrics available at daemon startup, see [test/sharness/t0119-prometheus-data/prometheus_metrics_added_by_measure_profile](https://github.com/ipfs/kubo/blob/master/test/sharness/t0119-prometheus-data/prometheus_metrics_added_by_measure_profile).
>
> Additional metrics may appear during runtime as some components (like boxo/gateway) register metrics only after their first event occurs (e.g., HTTP request/response).

## DHT RPC

Metrics from `go-libp2p-kad-dht` for DHT RPC operations:

### Inbound RPC metrics

- `rpc_inbound_messages_total` - Counter: total messages received per RPC
- `rpc_inbound_message_errors_total` - Counter: total errors for received messages
- `rpc_inbound_bytes_[bucket|sum|count]` - Histogram: distribution of received bytes per RPC
- `rpc_inbound_request_latency_[bucket|sum|count]` - Histogram: latency distribution for inbound RPCs

### Outbound RPC metrics

- `rpc_outbound_messages_total` - Counter: total messages sent per RPC
- `rpc_outbound_message_errors_total` - Counter: total errors for sent messages
- `rpc_outbound_requests_total` - Counter: total requests sent
- `rpc_outbound_request_errors_total` - Counter: total errors for sent requests
- `rpc_outbound_bytes_[bucket|sum|count]` - Histogram: distribution of sent bytes per RPC
- `rpc_outbound_request_latency_[bucket|sum|count]` - Histogram: latency distribution for outbound RPCs

## Provide

### Legacy Provider

Metrics for the legacy provider system when `Provide.DHT.SweepEnabled=false`:

- `provider_reprovider_provide_count` - Counter: total successful provide operations since node startup
- `provider_reprovider_reprovide_count` - Counter: total reprovide sweep operations since node startup

### DHT Provider

Metrics for the DHT provider system when `Provide.DHT.SweepEnabled=true`:

- `total_provide_count_total` - Counter: total successful provide operations since node startup

## Gateway (`boxo/gateway`)

Gateway metrics appear after the first HTTP request is processed:

### HTTP metrics

- `ipfs_http_gw_responses_total{code}` - Counter: total HTTP responses by status code
- `ipfs_http_gw_retrieval_timeouts_total{code,truncated}` - Counter: requests that timed out during content retrieval
- `ipfs_http_gw_concurrent_requests` - Gauge: number of requests currently being processed

### Blockstore cache metrics

- `ipfs_http_blockstore_cache_hit` - Counter: global block cache hits
- `ipfs_http_blockstore_cache_requests` - Counter: global block cache requests

### Backend metrics

- `ipfs_gw_backend_api_call_duration_seconds_[bucket|sum|count]{backend_method}` - Histogram: time spent in IPFSBackend API calls