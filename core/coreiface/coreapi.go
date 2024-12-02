// Package iface defines IPFS Core API which is a set of interfaces used to
// interact with IPFS nodes.
package iface

import (
	"context"

	"github.com/ipfs/boxo/path"
	"github.com/ipfs/kubo/core/coreiface/options"

	ipld "github.com/ipfs/go-ipld-format"
)

// CoreAPI defines an unified interface to IPFS for Go programs
type CoreAPI interface {
	// Unixfs returns an implementation of Unixfs API
	Unixfs() UnixfsAPI

	// Block returns an implementation of Block API
	Block() BlockAPI

	// Dag returns an implementation of Dag API
	Dag() APIDagService

	// Name returns an implementation of Name API
	Name() NameAPI

	// Key returns an implementation of Key API
	Key() KeyAPI

	// Pin returns an implementation of Pin API
	Pin() PinAPI

	// Object returns an implementation of Object API
	Object() ObjectAPI

	// Swarm returns an implementation of Swarm API
	Swarm() SwarmAPI

	// PubSub returns an implementation of PubSub API
	PubSub() PubSubAPI

	// Routing returns an implementation of Routing API
	Routing() RoutingAPI

	// ResolvePath resolves the path using UnixFS resolver, and returns the resolved
	// immutable path, and the remainder of the path segments that cannot be resolved
	// within UnixFS.
	ResolvePath(context.Context, path.Path) (path.ImmutablePath, []string, error)

	// ResolveNode resolves the path (if not resolved already) using Unixfs
	// resolver, gets and returns the resolved Node
	ResolveNode(context.Context, path.Path) (ipld.Node, error)

	// WithOptions creates new instance of CoreAPI based on this instance with
	// a set of options applied
	WithOptions(...options.ApiOption) (CoreAPI, error)
}
