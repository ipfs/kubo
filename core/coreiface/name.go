package iface

import (
	"context"
	"time"

	options "github.com/ipfs/go-ipfs/core/coreapi/interface/options"
)

// IpnsEntry specifies the interface to IpnsEntries
type IpnsEntry interface {
	// Name returns IpnsEntry name
	Name() string
	// Value returns IpnsEntry value
	Value() Path
}

// NameAPI specifies the interface to IPNS.
//
// IPNS is a PKI namespace, where names are the hashes of public keys, and the
// private key enables publishing new (signed) values. In both publish and
// resolve, the default name used is the node's own PeerID, which is the hash of
// its public key.
//
// You can use .Key API to list and generate more names and their respective keys.
type NameAPI interface {
	// Publish announces new IPNS name
	Publish(ctx context.Context, path Path, opts ...options.NamePublishOption) (IpnsEntry, error)

	// WithValidTime is an option for Publish which specifies for how long the
	// entry will remain valid. Default value is 24h
	WithValidTime(validTime time.Duration) options.NamePublishOption

	// WithKey is an option for Publish which specifies the key to use for
	// publishing. Default value is "self" which is the node's own PeerID.
	// The key parameter must be either PeerID or keystore key alias.
	//
	// You can use KeyAPI to list and generate more names and their respective keys.
	WithKey(key string) options.NamePublishOption

	// Resolve attempts to resolve the newest version of the specified name
	Resolve(ctx context.Context, name string, opts ...options.NameResolveOption) (Path, error)

	// WithRecursive is an option for Resolve which specifies whether to perform a
	// recursive lookup. Default value is false
	WithRecursive(recursive bool) options.NameResolveOption

	// WithLocal is an option for Resolve which specifies if the lookup should be
	// offline. Default value is false
	WithLocal(local bool) options.NameResolveOption

	// WithCache is an option for Resolve which specifies if cache should be used.
	// Default value is true
	WithCache(cache bool) options.NameResolveOption
}
