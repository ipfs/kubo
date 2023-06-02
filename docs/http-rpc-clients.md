# HTTP/RPC Clients

Kubo provides official HTTP RPC  (`/api/v0`) clients for selected languages:

- [`js-kubo-rpc-client`](https://github.com/ipfs/js-kubo-rpc-client) - Official JS client for talking to Kubo RPC over HTTP
- [`go-ipfs-api`](https://github.com/ipfs/go-ipfs-api) - The go interface to ipfs's HTTP RPC - Follow https://github.com/ipfs/kubo/issues/9124 for coming changes.
- [`httpapi`](./client/rpc) (previously `go-ipfs-http-client`) - [`coreiface.CoreAPI`](https://pkg.go.dev/github.com/ipfs/boxo/coreiface#CoreAPI) implementation using HTTP RPC

## Recommended clients

| Language |     Package Name    | Github Repository                          |
|:--------:|:-------------------:|--------------------------------------------|
| JS       | kubo-rpc-client     | https://github.com/ipfs/js-kubo-rpc-client |
| Go       | `rpc`               | [`./client/rpc`](./client/rpc)             |
