//go:build tools

package tools

import (
	_ "github.com/Kubuxu/gocovmerge"
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "github.com/ipfs/go-cidutil/cid-fmt"
	_ "github.com/ipfs/go-test/cli/random-data"
	_ "github.com/ipfs/go-test/cli/random-files"
	_ "github.com/ipfs/hang-fds"
	_ "github.com/multiformats/go-multihash/multihash"
	_ "gotest.tools/gotestsum"
)
