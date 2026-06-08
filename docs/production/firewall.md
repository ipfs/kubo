# Firewall Setup for Kubo

By default, kubo's libp2p swarm listens on **port 4001** over both TCP and
UDP. Open both so peers can reach you:

- **TCP/4001** carries the plain TCP transport (and the optional WebSocket `/ws`).
- **UDP/4001** carries QUIC, WebTransport, and WebRTC-Direct.

Block either one and kubo falls back to slower relayed or hole-punched
connections.

The defaults come from [`Addresses.Swarm`](../config.md#addressesswarm):

```json
[
  "/ip4/0.0.0.0/tcp/4001",
  "/ip6/::/tcp/4001",
  "/ip4/0.0.0.0/udp/4001/webrtc-direct",
  "/ip4/0.0.0.0/udp/4001/quic-v1",
  "/ip4/0.0.0.0/udp/4001/quic-v1/webtransport",
  "/ip6/::/udp/4001/webrtc-direct",
  "/ip6/::/udp/4001/quic-v1",
  "/ip6/::/udp/4001/quic-v1/webtransport"
]
```

The examples below use [`ufw`](https://help.ubuntu.com/community/UFW), the
default firewall tool on Debian and Ubuntu. The same rules translate to
`firewalld`, `nftables`, or cloud security groups.

## Check what rules you have

List active rules:

```bash
sudo ufw status verbose
```

Or with line numbers, useful when deleting one later:

```bash
sudo ufw status numbered
```

A typical SSH-only host looks like this:

```
Status: active
Logging: off
Default: deny (incoming), allow (outgoing), disabled (routed)

To                         Action      From
--                         ------      ----
22/tcp                     ALLOW IN    Anywhere
22/tcp (v6)                ALLOW IN    Anywhere (v6)
```

You want 4001 in that list, on both TCP and UDP.

## Open port 4001

The short way opens both TCP and UDP at once:

```bash
sudo ufw allow 4001 comment 'ipfs/libp2p swarm'
```

One rule per protocol reads more clearly later:

```bash
sudo ufw allow 4001/tcp comment 'ipfs/libp2p tcp+http+ws'
sudo ufw allow 4001/udp comment 'ipfs/libp2p quic+webtransport+webrtc'
```

`ufw` covers IPv4 and IPv6 together when `IPV6=yes` is set in
`/etc/default/ufw` (the default on Ubuntu).

To limit a rule to one interface or source range:

```bash
sudo ufw allow in on eth0 to any port 4001 proto tcp
sudo ufw allow in on eth0 to any port 4001 proto udp
sudo ufw allow from 203.0.113.0/24 to any port 4001
```

> [!NOTE]
> A public IPFS node needs to be reachable by anyone. Restrict by source IP
> only on private deployments.

Check the result:

```bash
sudo ufw status verbose
```

You should see `4001/tcp` and `4001/udp` (and the matching `(v6)` lines).

## Optional: a `Kubo` application profile

When you run kubo across many hosts, a `ufw` "application profile" lets you
allow it by name. Create `/etc/ufw/applications.d/kubo`:

```ini
[Kubo]
title=Kubo
description=ipfs kubo swarm ports
ports=4001/tcp|4001/udp
```

Allow it by name:

```bash
sudo ufw allow Kubo
```

Inspect the profile:

```bash
sudo ufw app info Kubo
```

If you later edit the `ports=` line in the profile, push the new ports
into the existing rule with:

```bash
sudo ufw app update Kubo
```

## Different ports?

If you changed [`Addresses.Swarm`](../config.md#addressesswarm) (for example,
when running several kubo nodes on one host), open the port you chose. Open
both TCP and UDP unless you explicitly disabled a transport in
[`Swarm.Transports.Network`](../config.md#swarmtransportsnetwork).

## Remove a rule

Find the rule number:

```bash
sudo ufw status numbered
```

Numbers shift after each delete, so list again between deletes:

```bash
sudo ufw delete <number>
```

Or delete by spec:

```bash
sudo ufw delete allow 4001/tcp
sudo ufw delete allow 4001/udp
```

## Is the daemon healthy?

To confirm kubo is running and the local block pipeline works:

```bash
ipfs diag healthy
```

It exits 0 when the daemon is up. Use it for container healthchecks. It
only checks local state; for reachability from outside, see the next
section.

## Can peers reach you?

`ipfs id` shows the addresses your node advertises. To test them from
outside, ask AutoNAT V2:

```bash
ipfs swarm addrs autonat
```

Look for `Reachability: Public`. The `Reachable` and `Unreachable` lists
break things down by address, so you can see at a glance which protocol is
blocked upstream.

If you stay `Private` even with `ufw` open, something upstream is blocking
you. Common next steps:

- **Behind a home or office router (NAT):** let kubo ask the router to
  forward the port. Keep
  [`Swarm.DisableNatPortMap`](../config.md#swarmdisablenatportmap) at `false`
  (the default; this is UPnP / NAT-PMP). The `server` profile disables it,
  so if you applied that profile but you are behind a router, set it back
  to `false`.
- **No control over the upstream NAT (CGNAT, mobile, locked-down corporate
  networks):** keep
  [`Swarm.EnableHolePunching`](../config.md#swarmenableholepunching) on
  (the default). Peers will then reach you through a relay using DCUtR
  (direct connection upgrade through relay).

More background: the
[libp2p AutoNAT V2 spec](https://github.com/libp2p/specs/blob/master/autonat/autonat-v2.md).

## Related

- [`Addresses.Swarm`](../config.md#addressesswarm): the addresses kubo
  listens on.
- [`Swarm.Transports.Network`](../config.md#swarmtransportsnetwork): which
  transports are enabled.
- [Security section in `config.md`](../config.md#security): port and
  exposure guidance for the API, Gateway, and swarm.
