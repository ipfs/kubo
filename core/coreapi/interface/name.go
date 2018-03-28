package iface

import (
	"context"

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

	// Resolve attempts to resolve the newest version of the specified name
	Resolve(ctx context.Context, name string, opts ...options.NameResolveOption) (Path, error)
}
