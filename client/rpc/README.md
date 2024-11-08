# `coreiface.CoreAPI` over http `rpc`

> IPFS CoreAPI implementation using HTTP API

This package implements [`coreiface.CoreAPI`](https://pkg.go.dev/github.com/ipfs/kubo/core/coreiface#CoreAPI) over the HTTP API.

## Documentation

https://pkg.go.dev/github.com/ipfs/kubo/client/rpc

### Example

Pin file on your local IPFS node based on its CID:

```go
package main

import (
	"context"
	"fmt"

	"github.com/ipfs/boxo/path"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/kubo/client/rpc"
)

func main() {
	// "Connect" to local node
	node, err := rpc.NewLocalApi()
	if err != nil {
		fmt.Println(err)
		return
	}
	// Pin a given file by its CID
	ctx := context.Background()
	c, err := cid.Decode("bafkreidtuosuw37f5xmn65b3ksdiikajy7pwjjslzj2lxxz2vc4wdy3zku")
	if err != nil {
		fmt.Println(err)
		return
	}
	p := path.FromCid(c)
	err = node.Pin().Add(ctx, p)
	if err != nil {
		fmt.Println(err)
		return
	}
}
```
