# Kubo environment variables

## `IPFS_PATH`

Sets the location of the IPFS repo (where the config, blocks, etc.
are stored).

Default: ~/.ipfs

## `IPFS_LOGGING`

Specifies the log level for Kubo.

`IPFS_LOGGING` is a deprecated alias for the `GOLOG_LOG_LEVEL` environment variable.  See below.

## `IPFS_LOGGING_FMT`

Specifies the log message format.

`IPFS_LOGGING_FMT` is a deprecated alias for the `GOLOG_LOG_FMT` environment variable.  See below.

## `GOLOG_LOG_LEVEL`

Specifies the log-level, both globally and on a per-subsystem basis.  Level can be one of:

* `debug`
* `info`
* `warn`
* `error`
* `dpanic`
* `panic`
* `fatal`

Per-subsystem levels can be specified with `subsystem=level`.  One global level and one or more per-subsystem levels
can be specified by separating them with commas.

Default: `error`

Example:

```console
GOLOG_LOG_LEVEL="error,core/server=debug" ipfs daemon
```

Logging can also be configured at runtime, both globally and on a per-subsystem basis, with the `ipfs log` command.

## `GOLOG_LOG_FMT`

Specifies the log message format.  It supports the following values:

- `color` -- human readable, colorized (ANSI) output
- `nocolor` -- human readable, plain-text output.
- `json` -- structured JSON.

For example, to log structured JSON (for easier parsing):

```bash
export GOLOG_LOG_FMT="json"
```
The logging format defaults to `color` when the output is a terminal, and `nocolor` otherwise.

## `GOLOG_FILE`

Sets the file to which Kubo logs. By default, Kubo logs to standard error.

## `GOLOG_TRACING_FILE`

Sets the file to which Kubo sends tracing events. By default, tracing is
disabled.

This log can be read at runtime (without writing it to a file) using the `ipfs
log tail` command.

Warning: Enabling tracing will likely affect performance.

## `IPFS_FUSE_DEBUG`

If SET, enables fuse debug logging.

Default: false

## `YAMUX_DEBUG`

If SET, enables debug logging for the yamux stream muxer.

Default: false

## `IPFS_FD_MAX`

Sets the file descriptor limit for Kubo. If Kubo fails to set the file
descriptor limit, it will log an error.

Defaults: 2048

## `IPFS_DIST_PATH`

IPFS Content Path from which Kubo fetches repo migrations (when the daemon
is launched with the `--migrate` flag).

Default: `/ipfs/<cid>` (the exact path is hardcoded in
`migrations.CurrentIpfsDist`, depends on the IPFS version)

## `IPFS_NS_MAP`

Adds static namesys records for deterministic tests and debugging.
Useful for testing things like DNSLink without real DNS lookup.

Example:

```console
$ IPFS_NS_MAP="dnslink-test1.example.com:/ipfs/bafkreicysg23kiwv34eg2d7qweipxwosdo2py4ldv42nbauguluen5v6am,dnslink-test2.example.com:/ipns/dnslink-test1.example.com" ipfs daemon
...
$ ipfs resolve -r /ipns/dnslink-test2.example.com
/ipfs/bafkreicysg23kiwv34eg2d7qweipxwosdo2py4ldv42nbauguluen5v6am
```

## `IPFS_HTTP_ROUTERS`

Overrides all implicit HTTP routers enabled when `Routing.Type=auto` with
the space-separated list of URLs provided in this variable.
Useful for testing and debugging in offline contexts.

Example:

```console
$ ipfs config Routing.Type auto
$ IPFS_HTTP_ROUTERS="http://127.0.0.1:7423" ipfs daemon
```

The above will replace implicit HTTP routers with single one, allowing for
inspection/debug of HTTP requests sent by Kubo via `while true ; do nc -l 7423; done`
or more advanced tools like [mitmproxy](https://docs.mitmproxy.org/stable/#mitmproxy).

## `LIBP2P_TCP_REUSEPORT`

Kubo tries to reuse the same source port for all connections to improve NAT
traversal. If this is an issue, you can disable it by setting
`LIBP2P_TCP_REUSEPORT` to false.

Default: true

## `LIBP2P_MUX_PREFS`

Deprecated: Use the `Swarm.Transports.Multiplexers` config field.

Tells Kubo which multiplexers to use in which order.

Default: "/yamux/1.0.0 /mplex/6.7.0"

## `LIBP2P_RCMGR`

Forces [libp2p Network Resource Manager](https://github.com/libp2p/go-libp2p-resource-manager#readme)
to be enabled (`1`) or disabled (`0`).
When set, overrides [`Swarm.ResourceMgr.Enabled`](https://github.com/ipfs/kubo/blob/master/docs/config.md#swarmresourcemgrenabled) from the config.

Default: use config (not set)

## `LIBP2P_DEBUG_RCMGR`

Enables tracing of [libp2p Network Resource Manager](https://github.com/libp2p/go-libp2p-resource-manager#readme)
and outputs it to `rcmgr.json.gz`


Default: disabled (not set)

# Tracing
For advanced configuration (e.g. ratio-based sampling), see also: https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/sdk-environment-variables.md

## `OTEL_TRACES_EXPORTER`
Specifies the exporters to use as a comma-separated string. Each exporter has a set of additional environment variables used to configure it. The following values are supported:

- `otlp`
- `jaeger`
- `zipkin`
- `file` -- appends traces to a JSON file on the filesystem

Setting this enables OpenTelemetry tracing.

**NOTE** Tracing support is experimental: releases may contain tracing-related breaking changes.

Default: "" (no exporters)

## `OTLP Exporter`
Unless specified in this section, the OTLP exporter uses the environment variables documented here: https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/protocol/exporter.md

### `OTEL_EXPORTER_OTLP_PROTOCOL`
Specifies the OTLP protocol to use, which is one of:

- `grpc`
- `http/protobuf`

Default: "grpc"

## `Jaeger Exporter`

See: https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/sdk-environment-variables.md#jaeger-exporter

## `Zipkin Exporter`
See: https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/sdk-environment-variables.md#zipkin-exporter

## `File Exporter`
### `OTEL_EXPORTER_FILE_PATH`
Specifies the filesystem path for the JSON file.

Default: "$PWD/traces.json"

### How to use Jaeger UI

One can use the `jaegertracing/all-in-one` Docker image to run a full Jaeger
stack and configure Kubo to publish traces to it (here, in an ephemeral
container):

```console
$ docker run --rm -it --name jaeger \
    -e COLLECTOR_ZIPKIN_HOST_PORT=:9411 \
    -p 5775:5775/udp \
    -p 6831:6831/udp \
    -p 6832:6832/udp \
    -p 5778:5778 \
    -p 16686:16686 \
    -p 14268:14268 \
    -p 14268:14269 \
    -p 14250:14250 \
    -p 9411:9411 \
    jaegertracing/all-in-one
```

Then, in other terminal, start Kubo with Jaeger tracing enabled:
```
$ OTEL_TRACES_EXPORTER=jaeger ipfs daemon
```

Finally, the [Jaeger UI](https://github.com/jaegertracing/jaeger-ui#readme) is available at http://localhost:16686
