# go-ipfs environment variables

## `LIBP2P_TCP_REUSEPORT`

go-ipfs tries to reuse the same source port for all connections to improve NAT
traversal. If this is an issue, you can disable it by setting
`LIBP2P_TCP_REUSEPORT` to false.

Default: true

## `IPFS_PATH`

Sets the location of the IPFS repo (where the config, blocks, etc.
are stored).

Default: ~/.ipfs

## `IPFS_LOGGING`

Specifies the log level for go-ipfs.

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

Sets the file to which go-ipfs logs. By default, go-ipfs logs to standard error.

## `GOLOG_TRACING_FILE`

Sets the file to which go-ipfs sends tracing events. By default, tracing is
disabled.

This log can be read at runtime (without writing it to a file) using the `ipfs
log tail` command.

Warning: Enabling tracing will likely affect performance.

## `IPFS_FUSE_DEBUG`

Enables fuse debug logging.

Default: false

## `YAMUX_DEBUG`

Enables debug logging for the yamux stream muxer.

Default: false

## `IPFS_FD_MAX`

Sets the file descriptor limit for go-ipfs. If go-ipfs fails to set the file
descriptor limit, it will log an error.

Defaults: 2048

## `IPFS_DIST_PATH`

IPFS Content Path from which go-ipfs fetches repo migrations (when the daemon
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

## `LIBP2P_MUX_PREFS`

Deprecated: Use the `Swarm.Transports.Multiplexers` config field.

Tells go-ipfs which multiplexers to use in which order.

Default: "/yamux/1.0.0 /mplex/6.7.0"
