// +build tools

package tools

import (
	_ "github.com/Kubuxu/gocovmerge"
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "github.com/ipfs/go-cidutil/cid-fmt"
	_ "github.com/ipfs/hang-fds"
	_ "github.com/jbenet/go-random-files/random-files"
	_ "github.com/jbenet/go-random/random"
	_ "github.com/multiformats/go-multihash/multihash"
	_ "gotest.tools/gotestsum"
)
