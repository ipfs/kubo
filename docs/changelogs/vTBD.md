# Kubo changelog vTBD

- [vTBD](#vtbd)

## vTBD

- [Overview](#overview)
- [🔦 Highlights](#-highlights)
  - [🌐 Experimental `HTTPProvider`: serve local data over plain HTTP/2](#-experimental-httpprovider-serve-local-data-over-plain-http2)
  - [🔒 AutoTLS without a broker: certificates for your own IP](#-autotls-without-a-broker-certificates-for-your-own-ip)
- [📝 Changelog](#-changelog)
- [👨‍👩‍👧‍👦 Contributors](#-contributors)

### Overview

### 🔦 Highlights

#### 🌐 Experimental `HTTPProvider`: serve local data over plain HTTP/2

Kubo now ships an experimental, opt-in `HTTPProvider`: the **server** side of HTTP retrieval. Enable it on a publicly dialable node and the local trustless gateway becomes reachable over plain HTTP/2 alongside the existing libp2p path, on the same swarm port (`4001` by default). Kubo reuses the certificate it already obtains through [`AutoTLS`](https://github.com/ipfs/kubo/blob/master/docs/config.md#autotls), so no extra TLS setup is required.

`HTTPProvider` is read-only: it serves raw blocks (`?format=raw`) from the local blockstore and does not fetch missing data from the network. It is **not** the same as [`Gateway`](https://github.com/ipfs/kubo/blob/master/docs/config.md#gateway), which stays on loopback `127.0.0.1:8080` and is the recursive, deserializing interface for local browsing.

This is a meaningful step toward broader IPFS interoperability: a stock HTTP client, a browser, `curl`, or any HTTP library can fetch verifiable blocks from a Kubo node without a libp2p stack. It pairs with the existing [`HTTPRetrieval`](https://github.com/ipfs/kubo/blob/master/docs/config.md#httpretrieval) (the client side).

Off by default. See [`HTTPProvider`](https://github.com/ipfs/kubo/blob/master/docs/config.md#httpprovider) for the available settings and how to turn it on.

#### 🔒 AutoTLS without a broker: certificates for your own IP

If your node listens on TCP port 443, AutoTLS now gets its TLS certificate for your node's own IP address, directly from Let's Encrypt. No name is registered with the `libp2p.direct` broker, and no DNS lookup sits between a browser and your node.

The only change needed is a port 443 listener in [`Addresses.Swarm`](https://github.com/ipfs/kubo/blob/master/docs/config.md#addressesswarm), such as `/ip4/0.0.0.0/tcp/443`. Let's Encrypt checks the address by connecting back to it and speaking TLS, which your node answers on the same listener it already serves peers on, so nothing extra is exposed. Such a node announces `/ip4/<your-ip>/tcp/443/tls/ws` (and the matching `/tls/http` when [`HTTPProvider.AnnounceMultiaddrs`](https://github.com/ipfs/kubo/blob/master/docs/config.md#httpproviderannouncemultiaddrs) is on).

The listener picks the path and there is no crossing over. A node on port 443 asks only for a certificate for its own address; if it cannot get one, that is an error in the log and it keeps trying, rather than quietly registering a name you did not ask for. A node with no port 443 listener keeps using the broker exactly as before. A node that already holds a `libp2p.direct` certificate and listens on 443 moves to the direct path on upgrade.

Certificates covering an IP address are short-lived by policy, valid for about six days, so a node has to stay online to keep renewing. Requests are paced to stay well inside Let's Encrypt's rate limits, and the pacing survives daemon restarts. See [`AutoTLS.IPCerts`](https://github.com/ipfs/kubo/blob/master/docs/config.md#autotlsipcerts).

### 📝 Changelog

### 👨‍👩‍👧‍👦 Contributors
