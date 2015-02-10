// package namesys implements various functionality for the ipns naming system.
package namesys

import (
	"errors"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ci "github.com/jbenet/go-ipfs/p2p/crypto"
	u "github.com/jbenet/go-ipfs/util"
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
	Resolve(ctx context.Context, name string) (value u.Key, err error)

	// CanResolve checks whether this Resolver can resolve a name
	CanResolve(name string) bool
}

// Publisher is an object capable of publishing particular names.
type Publisher interface {

	// Publish establishes a name-value mapping.
	// TODO make this not PrivKey specific.
	Publish(ctx context.Context, name ci.PrivKey, value u.Key) error
}
