package main

import (
	"encoding/hex"
	"fmt"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
)

func main() {
	// ignores errors for simplicity.
	// don't do that at home.

	buf, _ := hex.DecodeString("0beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a33")
	mhbuf, _ := multihash.EncodeName(buf, "sha1")
	mhhex := hex.EncodeToString(mhbuf)
	fmt.Printf("hex: %v\n", mhhex)

	o, _ := multihash.Decode(mhbuf)
	mhhex = hex.EncodeToString(o.Digest)
	fmt.Printf("obj: %v 0x%x %d %s\n", o.Name, o.Code, o.Length, mhhex)
}
