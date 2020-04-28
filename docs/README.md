# Developer Documentation and Guides

If you are looking for User Documentation & Guides, please visit [docs.ipfs.io](https://docs.ipfs.io/).

If you’re experiencing an issue with IPFS, **please follow [our issue guide](github-issue-guide.md) when filing an issue!**

Otherwise, check out the following guides to using and developing IPFS:

## Developing `go-ipfs`

- First, please read the Contributing Guidelines [for IPFS projects](https://github.com/ipfs/community/blob/master/CONTRIBUTING.md) and then the Contributing Guidelines for [go-ipfs specifically](https://github.com/ipfs/community/blob/master/CONTRIBUTING_GO.md)
- Building on…
    - [Windows](windows.md)
- [Performance Debugging Guidelines](debug-guide.md)
- [Release Checklist](releases.md)

## Guides

- [How to Implement an API Client](implement-api-bindings.md)
- [Connecting with Websockets](transports.md) — if you want `js-ipfs` nodes in web browsers to connect to your `go-ipfs` node, you will need to turn on websocket support in your `go-ipfs` node.

## Advanced User Guides

- [Transferring a File Over IPFS](file-transfer.md)
- [Configuration reference](config.md)
    - [Datastore configuration](datastores.md)
    - [Experimental features](experimental-features.md)
- [Installing command completion](command-completion.md)
- [Mounting IPFS with FUSE](fuse.md)
- [Installing plugins](plugins.md)
- [Setting up an IPFS Gateway](https://github.com/ipfs/go-ipfs/blob/master/docs/gateway.md)

## Other

- [Thanks to all our contributors ❤️](AUTHORS) (We use the `generate-authors.sh` script to regenerate this list.)
- [How to file a GitHub Issue](github-issue-guide.md)
