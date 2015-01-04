package namesys

import (
	"errors"

	ci "github.com/jbenet/go-ipfs/p2p/crypto"
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
	Resolve(name string) (value string, err error)

	// CanResolve checks whether this Resolver can resolve a name
	CanResolve(name string) bool
}

// Publisher is an object capable of publishing particular names.
type Publisher interface {

	// Publish establishes a name-value mapping.
	// TODO make this not PrivKey specific.
	Publish(name ci.PrivKey, value string) error
}
