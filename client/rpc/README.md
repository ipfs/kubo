# `coreiface.CoreAPI` over http `rpc`

> IPFS CoreAPI implementation using HTTP API

This packages implements [`coreiface.CoreAPI`](https://pkg.go.dev/github.com/ipfs/boxo/coreiface#CoreAPI) over the HTTP API.

## Documentation

https://pkg.go.dev/github.com/ipfs/kubo/client/rpc

### Example

Pin file on your local IPFS node based on its CID:

```go
package main

import (
    "context"
    "fmt"

    "github.com/ipfs/kubo/client/rpc"
    path "github.com/ipfs/boxo/coreiface/path"
)

func main() {
    // "Connect" to local node
    node, err := rpc.NewLocalApi()
    if err != nil {
        fmt.Printf(err)
        return
    }
    // Pin a given file by its CID
    ctx := context.Background()
    cid := "bafkreidtuosuw37f5xmn65b3ksdiikajy7pwjjslzj2lxxz2vc4wdy3zku"
    p := path.New(cid)
    err = node.Pin().Add(ctx, p)
    if err != nil {
    	fmt.Printf(err)
        return
    }
    return
}
```
