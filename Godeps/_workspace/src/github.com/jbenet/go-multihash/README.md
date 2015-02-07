# go-multihash

![travis](https://travis-ci.org/jbenet/go-multihash.svg)

[multihash](//github.com/jbenet/multihash) implementation in Go.

## Example

```go
package main

import (
  "encoding/hex"
  "fmt"
  "github.com/jbenet/go-multihash"
)

func main() {
  // ignores errors for simplicity.
  // don't do that at home.

  buf, _ := hex.DecodeString("0beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a33")
  mhbuf, _ := multihash.EncodeName(buf, "sha1");
  mhhex := hex.EncodeToString(mhbuf)
  fmt.Printf("hex: %v\n", mhhex);

  o, _ := multihash.Decode(mhbuf);
  mhhex = hex.EncodeToString(o.Digest);
  fmt.Printf("obj: %v 0x%x %d %s\n", o.Name, o.Code, o.Length, mhhex);
}
```

Run [test/foo.go](test/foo.go)

```
> cd test/
> go build
> ./test
hex: 11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a33
obj: sha1 0x11 20 0beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a33
```

## License

MIT
