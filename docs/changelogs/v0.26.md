# Kubo changelog v0.26

- [v0.26.0](#v0260)

## v0.26.0

- [Overview](#overview)
- [🔦 Highlights](#-highlights)
  - [Several deprecated commands have been removed](#several-deprecated-commands-have-been-removed)
  - [Support optional pin names](#support-optional-pin-names)
  - [`jaeger` trace exporter has been removed](#jaeger-trace-exporter-has-been-removed)
- [📝 Changelog](#-changelog)
- [👨‍👩‍👧‍👦 Contributors](#-contributors)

### Overview

### 🔦 Highlights

#### Kubo binary imports

For users of [Kubo preloaded plugins](https://github.com/ipfs/kubo/blob/master/docs/plugins.md#preloaded-plugins) there is now a way to create a kubo instance with your plugins by depending on the `cmd/ipfs/kubo` package rather than rebuilding kubo with the included plugins.

See the [customization docs](https://github.com/ipfs/kubo/blob/master/docs/customizing.md) for more information.

#### Several deprecated commands have been removed

Several deprecated commands have been removed:

- `ipfs urlstore` deprecated in [April 2019, Kubo 0.4.21](https://github.com/ipfs/kubo/commit/8beaee63b3fa634c59b85179286ad3873921a535), use `ipfs add -q --nocopy --cid-version=1 {url}` instead.
- `ipfs repo fsck` deprecated in [July 2019, Kubo 0.5.0](https://github.com/ipfs/kubo/commit/288a83ce7dcbf4a2498e06e4a95245bbb5e30f45)
- `ipfs file` (and `ipfs file ls`) deprecated in [November 2020, Kubo  0.8.0](https://github.com/ipfs/kubo/commit/ec64dc5c396e7114590e15909384fabce0035482), use `ipfs ls` and `ipfs files ls` instead.
- `ipfs dns` deprecated in [April 2022, Kubo 0.13](https://github.com/ipfs/kubo/commit/76ae33a9f3f9abd166d1f6f23d6a8a0511510e3c), use `ipfs resolve /ipns/{name}` instead.
- `ipfs tar` deprecated [April 2022, Kubo 0.13](https://github.com/ipfs/kubo/pull/8849)

#### Support optional pin names

You can now add a name to a pin when pinning a CID. To do so, use `ipfs pin add --name "Some Name" bafy...`. You can list your pins, including their names, with `ipfs pin ls --names`.

#### `jaeger` trace exporter has been removed

`jaeger` exporter has been removed from upstream, you should use `otlp` exporter instead.
See the [boxo tracing docs](https://github.com/ipfs/boxo/blob/a391d02102875ee7075a692076154bec1fa871f3/docs/tracing.md) for an example.

### 📝 Changelog

- Export a `kubo.Start` function so users can programmatically start Kubo from within a go program.

### 👨‍👩‍👧‍👦 Contributors
