// +build tools

package tools

import (
	_ "github.com/multiformats/go-multihash/multihash"
	_ "github.com/jbenet/go-random-files/random-files"
	_ "github.com/ipfs/hang-fds"
	_ "github.com/ipfs/go-cidutil/cid-fmt"
	_ "go-build,github.com/jbenet/go-random/random"
)

