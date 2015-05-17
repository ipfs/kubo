/*
Package namesys implements resolvers and publishers for the IPFS
naming system (IPNS).

The core of IPFS is an immutable, content-addressable Merkle graph.
That works well for many use cases, but doesn't allow you to answer
questions like "what is Alice's current homepage?".  The mutable name
system allows Alice to publish information like:

  The current homepage for alice.example.com is
  /ipfs/Qmcqtw8FfrVSBaRmbWwHxt3AuySBhJLcvmFYi3Lbc4xnwj

or:

  The current homepage for node
  QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy
  is
  /ipfs/Qmcqtw8FfrVSBaRmbWwHxt3AuySBhJLcvmFYi3Lbc4xnwj

The mutable name system also allows users to resolve those references
to find the immutable IPFS object currently referenced by a given
mutable name.

For command-line bindings to this functionality, see:

  ipfs name
  ipfs dns
  ipfs resolve
*/
package namesys

import (
	"errors"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	ci "github.com/ipfs/go-ipfs/p2p/crypto"
	path "github.com/ipfs/go-ipfs/path"
)

// ErrResolveFailed signals an error when attempting to resolve.
var ErrResolveFailed = errors.New("could not resolve name.")

// ErrPublishFailed signals an error when attempting to publish.
var ErrPublishFailed = errors.New("could not publish name.")

// Namesys represents a cohesive name publishing and resolving system.
//
// Publishing a name is the process of establishing a mapping, a key-value
// pair, according to naming rules and databases.
//
// Resolving a name is the process of looking up the value associated with the
// key (name).
type NameSystem interface {
	Resolver
	Publisher
}

// Resolver is an object capable of resolving names.
type Resolver interface {

	// Resolve looks up a name, and returns the value previously published.
	Resolve(ctx context.Context, name string) (value path.Path, err error)

	// CanResolve checks whether this Resolver can resolve a name
	CanResolve(name string) bool
}

// Publisher is an object capable of publishing particular names.
type Publisher interface {

	// Publish establishes a name-value mapping.
	// TODO make this not PrivKey specific.
	Publish(ctx context.Context, name ci.PrivKey, value path.Path) error
}
