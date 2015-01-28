// Package incfusever is only here to prevent go src tools (like godep)
// from thinking fuseversion is not a required package. Though we do not
// actually use github.com/jbenet/go-fuse-version as a library, we
// _may_ need its binary. We avoid it as much as possible as compiling
// it _requires_ fuse headers. Users must be able to install go-ipfs
// without also installing fuse.
package incfusever

import (
	fuseversion "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-fuse-version"
)

var _ = fuseversion.LocalFuseSystems
