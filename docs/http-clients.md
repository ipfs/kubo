# HTTP Clients

To date, we have four different HTTP RPC clients:

- [kubo-rpc-client](https://github.com/ipfs/js-kubo-rpc-client) - Official JS client for talking to Kubo over HTTP
- [ipfs-http-client](https://github.com/ipfs/js-ipfs/tree/master/packages/ipfs-http-client) - http client for js-ipfs -- Use when talking to [`js-ipfs`](https://github.com/ipfs/js-ipfs/tree/master/packages/ipfs)
- [go-ipfs-api](https://github.com/ipfs/go-ipfs-api) - The go interface to ipfs's HTTP API
- [go-ipfs-http-client](https://github.com/ipfs/go-ipfs-http-client) - IPFS CoreAPI implementation using HTTP API
- [kubo/commands/http](https://github.com/ipfs/kubo/tree/916f987de2c35db71815b54bbb9a0a71df829838/commands/http) -
  generalized transport based on the [command definitions](https://github.com/ipfs/kubo/tree/916f987de2c35db71815b54bbb9a0a71df829838/core/commands)

## Recommended clients

| Language |     Package Name    | Github Repository                           |
|:--------:|:-------------------:|---------------------------------------------|
| JS       | kubo-rpc-client     | https://github.com/ipfs/js-kubo-rpc-client  |
| Go       | go-ipfs-http-client | https://github.com/ipfs/go-ipfs-http-client |
