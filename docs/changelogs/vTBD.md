# Kubo changelog vTBD

<a href="https://ipshipyard.com/"><img align="right" src="https://github.com/user-attachments/assets/39ed3504-bb71-47f6-9bf8-cb9a1698f272" /></a>

This release was brought to you by the [Shipyard](https://ipshipyard.com/) team.

- [vTBD](#vtbd)

## vTBD

- [Overview](#overview)
- [🔦 Highlights](#-highlights)
  - [🌐 Browsers can dial you again through delegated routers](#-browsers-can-dial-you-again-through-delegated-routers)
- [📝 Changelog](#-changelog)
- [👨‍👩‍👧‍👦 Contributors](#-contributors)

### Overview

### 🔦 Highlights

#### 🌐 Browsers can dial you again through delegated routers

A publicly reachable node with [AutoTLS](https://github.com/ipfs/kubo/blob/master/docs/config.md#autotls) or `webrtc-direct` enabled was leaving those addresses out of the provider records it sent to HTTP routers. The record carried only `tcp`, `quic-v1`, and `webtransport`, none of which a browser can dial, so browser and [Helia](https://helia.io/) clients that found the node through a delegated router had no way to connect to it, even though the node was listening and `ipfs id` showed the addresses.

Provider records sent to HTTP routers now carry the same addresses the node announces everywhere else, including the AutoTLS `/tls/ws` and `webrtc-direct` ones, matching what the DHT already published. Loopback and LAN addresses stay out of the record when the node has a public address. See [#11369](https://github.com/ipfs/kubo/issues/11369).

### 📝 Changelog

### 👨‍👩‍👧‍👦 Contributors
