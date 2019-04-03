package node

import (
	"github.com/ipfs/go-ipfs/repo"
)

type BuildCfg struct {
	// If online is set, the node will have networking enabled
	Online bool

	// ExtraOpts is a map of extra options used to configure the ipfs nodes creation
	ExtraOpts map[string]bool

	// If permanent then node should run more expensive processes
	// that will improve performance in long run
	Permanent bool

	// DisableEncryptedConnections disables connection encryption *entirely*.
	// DO NOT SET THIS UNLESS YOU'RE TESTING.
	DisableEncryptedConnections bool

	// If NilRepo is set, a Repo backed by a nil datastore will be constructed
	NilRepo bool

	Routing RoutingOption
	Host    HostOption
	Repo    repo.Repo
}

func (cfg *BuildCfg) getOpt(key string) bool {
	if cfg.ExtraOpts == nil {
		return false
	}

	return cfg.ExtraOpts[key]
}
