package corefuse

import (
	"errors"

	fuseversion "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-fuse-version"
	eventlog "github.com/jbenet/go-ipfs/thirdparty/eventlog"
)

var errNotOnline = errors.New("This command must be run in online mode. Try running 'ipfs daemon' first.")

var log = eventlog.Logger("core/fuse")

// this file is only here to prevent go src tools (like godep) from
// thinking fuseversion is not a required package by non-darwin archs.
var _ = fuseversion.LocalFuseSystems
