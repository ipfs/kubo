package commands

import (
	fuseversion "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-fuse-version"
)

// this file is only here to prevent go src tools (like godep) from
// thinking fuseversion is not a required package by non-darwin archs.
var _ = fuseversion.LocalFuseSystems
