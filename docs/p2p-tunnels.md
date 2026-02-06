# P2P Tunnels

Kubo supports tunneling TCP connections through libp2p streams, similar to SSH
port forwarding (`ssh -L`). This allows exposing local services to remote peers
and forwarding remote services to local ports.

- [Why P2P Tunnels?](#why-p2p-tunnels)
- [Quick Start](#quick-start)
- [Background Mode](#background-mode)
- [Foreground Mode](#foreground-mode)
  - [systemd Integration](#systemd-integration)
- [Security Considerations](#security-considerations)
- [Troubleshooting](#troubleshooting)

## Why P2P Tunnels?

Unlike traditional SSH tunnels, libp2p-based tunnels do not require:

- **No public IP or open ports**: The server does not need a static IP address
  or port forwarding configured on the router. Connectivity to peers behind NAT
  is facilitated by [Direct Connection Upgrade through Relay (DCUtR)](https://github.com/libp2p/specs/blob/master/relay/DCUtR.md),
  which enables NAT hole-punching.

- **No DNS or IP address management**: All you need is the server's PeerID and
  an agreed-upon protocol name (e.g., `/x/ssh`). Kubo handles peer discovery
  and routing via the [Amino DHT](https://specs.ipfs.tech/routing/kad-dht/).

- **Simplified firewall rules**: Since connections are established through
  libp2p's existing swarm connections, no additional firewall configuration is
  needed beyond what Kubo already requires.

This makes p2p tunnels useful for connecting to machines on home networks,
behind corporate firewalls, or in environments where traditional port forwarding
is not available.

## Quick Start

Enable the experimental feature:

```console
$ ipfs config --json Experimental.Libp2pStreamMounting true
```

Test with netcat (`nc`) - no services required:

**On the server:**

```console
$ ipfs p2p listen /x/test /ip4/127.0.0.1/tcp/9999
$ nc -l -p 9999
```

**On the client:**

Replace `$SERVER_ID` with the server's peer ID (get it with `ipfs id -f "<id>\n"`
on the server).

```console
$ ipfs p2p forward /x/test /ip4/127.0.0.1/tcp/9998 /p2p/$SERVER_ID
$ nc 127.0.0.1 9998
```

Type in either terminal and the text appears in the other. Use Ctrl+C to exit.

## Background Mode

By default, `ipfs p2p listen` and `ipfs p2p forward` register the tunnel with
the daemon and return immediately. The tunnel persists until explicitly closed
with `ipfs p2p close` or the daemon shuts down.

This example exposes a local SSH server (listening on `localhost:22`) to a
remote peer. The same pattern works for any TCP service.

**On the server** (the machine running SSH):

Register a p2p listener that forwards incoming connections to the local SSH
server. The protocol name `/x/ssh` is an arbitrary identifier that both peers
must agree on (the `/x/` prefix is required for custom protocols).

```console
$ ipfs p2p listen /x/ssh /ip4/127.0.0.1/tcp/22
```

**On the client:**

Create a local port (`2222`) that tunnels through libp2p to the server's SSH
service.

```console
$ ipfs p2p forward /x/ssh /ip4/127.0.0.1/tcp/2222 /p2p/$SERVER_ID
```

Now connect to SSH through the tunnel:

```console
$ ssh user@127.0.0.1 -p 2222
```

**Other services:** To tunnel a different service, change the port and protocol
name. For example, to expose a web server on port 8080, use `/x/mywebapp` and
`/ip4/127.0.0.1/tcp/8080`.

## Foreground Mode

Use `--foreground` (`-f`) to block until interrupted. The tunnel is
automatically removed when the command exits:

```console
$ ipfs p2p listen /x/ssh /ip4/127.0.0.1/tcp/22 --foreground
Listening on /x/ssh, forwarding to /ip4/127.0.0.1/tcp/22, waiting for interrupt...
^C
Received interrupt, removing listener for /x/ssh
```

The listener/forwarder is automatically removed when:

- The command receives Ctrl+C or SIGTERM
- `ipfs p2p close` is called
- The daemon shuts down

This mode is useful for systemd services and scripts that need cleanup on exit.

### systemd Integration

The `--foreground` flag enables clean integration with systemd. The examples
below show how to run `ipfs p2p listen` as a user service that starts
automatically when the IPFS daemon is ready.

Ensure IPFS daemon runs as a systemd user service. See
[misc/README.md](https://github.com/ipfs/kubo/blob/master/misc/README.md#systemd)
for setup instructions and where to place unit files.

#### P2P listener with path-based activation

Use a `.path` unit to wait for the daemon's RPC API to be ready before starting
the p2p listener.

**`ipfs-p2p-tunnel.path`**:

```systemd
[Unit]
Description=Monitor for IPFS daemon startup
After=ipfs.service
Requires=ipfs.service

[Path]
PathExists=%h/.ipfs/api
Unit=ipfs-p2p-tunnel.service

[Install]
WantedBy=default.target
```

The `%h` specifier expands to the user's home directory. If you use a custom
`IPFS_PATH`, adjust accordingly.

**`ipfs-p2p-tunnel.service`**:

```systemd
[Unit]
Description=IPFS p2p tunnel
Requires=ipfs.service

[Service]
ExecStart=ipfs p2p listen /x/ssh /ip4/127.0.0.1/tcp/22 -f
Restart=on-failure
RestartSec=10

[Install]
WantedBy=default.target
```

#### Enabling the services

```console
$ systemctl --user enable ipfs.service
$ systemctl --user enable ipfs-p2p-tunnel.path
$ systemctl --user start ipfs.service
```

The path unit monitors `~/.ipfs/api` and starts `ipfs-p2p-tunnel.service`
once the file exists.

## Security Considerations

> [!WARNING]
> This feature provides CLI and HTTP RPC users with the ability to set up port
> forwarding for localhost and LAN ports. If you enable this and plan to expose
> CLI or HTTP RPC to other users or machines, secure the RPC API using
> [`API.Authorizations`](https://github.com/ipfs/kubo/blob/master/docs/config.md#apiauthorizations)
> or custom auth middleware.

## Troubleshooting

### Foreground listener stops when terminal closes

When using `--foreground`, the listener stops if the terminal closes. For
persistent foreground listeners, use a systemd service, `nohup`, `tmux`, or
`screen`. Without `--foreground`, the listener persists in the daemon regardless
of terminal state.

### Connection refused errors

Verify:

1. The experimental feature is enabled: `ipfs config Experimental.Libp2pStreamMounting`
2. The listener is active: `ipfs p2p ls`
3. Both peers can connect: `ipfs swarm connect /p2p/$PEER_ID`

### Persistent tunnel configuration

There is currently no way to define tunnels in the Kubo JSON config file. Use
`--foreground` mode with a systemd service for persistent tunnels. Support for
configuring tunnels via JSON config may be added in the future (see [kubo#5460](https://github.com/ipfs/kubo/issues/5460) - PRs welcome!).
