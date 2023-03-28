# go-ipfs-http-api

[![](https://img.shields.io/badge/made%20by-Protocol%20Labs-blue.svg?style=flat-square)](https://protocol.ai)
[![](https://img.shields.io/badge/project-IPFS-blue.svg?style=flat-square)](https://ipfs.io/)
[![](https://img.shields.io/badge/matrix-%23ipfs-blue.svg?style=flat-square)](https://app.element.io/#/room/#ipfs:matrix.org)
[![standard-readme compliant](https://img.shields.io/badge/standard--readme-OK-green.svg?style=flat-square)](https://github.com/RichardLitt/standard-readme)
[![GoDoc](https://godoc.org/github.com/ipfs/go-ipfs-http-api?status.svg)](https://godoc.org/github.com/ipfs/go-ipfs-http-api)

> IPFS CoreAPI implementation using HTTP API

This package is experimental and subject to change. If you need to depend on
something less likely to change, please use
[go-ipfs-api](https://github.com/ipfs/go-ipfs-api). If you'd like the latest and
greatest features, please use _this_ package.

## Documentation

https://godoc.org/github.com/ipfs/go-ipfs-http-api

### Example

Pin file on your local IPFS node based on its CID:

```go
package main

import (
    "context"
    "fmt"

    ipfsClient "github.com/ipfs/go-ipfs-http-client"
    path "github.com/ipfs/interface-go-ipfs-core/path"
)

func main() {
    // "Connect" to local node
    node, err := ipfsClient.NewLocalApi()
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

## Contribute

Feel free to join in. All welcome. Open an [issue](https://github.com/ipfs/go-ipfs-http-api/issues)!

This repository falls under the IPFS [Code of Conduct](https://github.com/ipfs/community/blob/master/code-of-conduct.md).

### Want to hack on IPFS?

[![](https://cdn.rawgit.com/jbenet/contribute-ipfs-gif/master/img/contribute.gif)](https://github.com/ipfs/community/blob/master/CONTRIBUTING.md)

## License

MIT
