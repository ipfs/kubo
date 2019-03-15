# go-ipfs environment variables

## `LIBP2P_TCP_REUSEPORT` (`IPFS_REUSEPORT`)

go-ipfs tries to reuse the same source port for all connections to improve NAT
traversal. If this is an issue, you can disable it by setting
`LIBP2P_TCP_REUSEPORT` to false.

This variable was previously `IPFS_REUSEPORT`.

Default: true

## `IPFS_PATH`

Sets the location of the IPFS repo (where the config, blocks, etc.
are stored).

Default: ~/.ipfs

## `IPFS_LOGGING`

Sets the log level for go-ipfs. It can be set to one of:

* `CRITICAL`
* `ERROR`
* `WARNING`
* `NOTICE`
* `INFO`
* `DEBUG`

Logging can also be configured (on a subsystem by subsystem basis) at runtime
with the `ipfs log` command.

Default: `ERROR`

## `IPFS_LOGGING_FMT`

Sets the log message format. Can be one of:

* `color`
* `nocolor`

Default: `color`

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

URL from which go-ipfs fetches repo migrations (when the daemon is launched with
the `--migrate` flag).

Default: https://ipfs.io/ipfs/$something (depends on the IPFS version)

## `IPFS_NS_MAP`

Adds static namesys records for deteministic tests and debugging.
Useful for testing things like DNSLink without real DNS lookup.

Example:

```console
$ IPFS_NS_MAP="dnslink-test1.example.com:/ipfs/bafkreicysg23kiwv34eg2d7qweipxwosdo2py4ldv42nbauguluen5v6am,dnslink-test2.example.com:/ipns/dnslink-test1.example.com" ipfs daemon
...
$ ipfs resolve -r /ipns/dnslink-test2.example.com
/ipfs/bafkreicysg23kiwv34eg2d7qweipxwosdo2py4ldv42nbauguluen5v6am
```

## `LIBP2P_MUX_PREFS`

Tells go-ipfs which multiplexers to use in which order.

Default: "/yamux/1.0.0 /mplex/6.7.0"
