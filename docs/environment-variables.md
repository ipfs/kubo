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

Default: `config.DefaultHTTPRouters`

## `IPFS_HTTP_ROUTERS_FILTER_PROTOCOLS`

Overrides values passed with `filter-protocols` parameter defined in IPIP-484.
Value is space-separated.

```console
$ IPFS_HTTP_ROUTERS_FILTER_PROTOCOLS="unknown transport-bitswap transport-foo" ipfs daemon
```

Default: `config.DefaultHTTPRoutersFilterProtocols`

## `IPFS_CONTENT_BLOCKING_DISABLE`

Disables the content-blocking subsystem. No denylists will be watched and no
content will be blocked.

## `LIBP2P_TCP_REUSEPORT`

Kubo tries to reuse the same source port for all connections to improve NAT
traversal. If this is an issue, you can disable it by setting
`LIBP2P_TCP_REUSEPORT` to false.

Default: `true`

## `LIBP2P_TCP_MUX`

By default Kubo tries to reuse the same listener port for raw TCP and WebSockets transports via experimental `libp2p.ShareTCPListener()` feature introduced in [go-libp2p#2984](https://github.com/libp2p/go-libp2p/pull/2984).
If this is an issue, you can disable it by setting `LIBP2P_TCP_MUX` to `false` and use separate ports for each TCP transport.

> [!CAUTION]
> This configuration option may be removed once `libp2p.ShareTCPListener()`  becomes default in go-libp2p.

Default: `true`

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

For tracing configuration, please check: https://github.com/ipfs/boxo/blob/main/docs/tracing.md
