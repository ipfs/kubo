package iface

import (
	"context"
	"errors"

	"github.com/ipfs/boxo/ipns"
	"github.com/ipfs/boxo/path"
	"github.com/ipfs/kubo/core/coreiface/options"
)

var ErrResolveFailed = errors.New("could not resolve name")

type IpnsResult struct {
	path.Path
	Err error
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
	Publish(ctx context.Context, path path.Path, opts ...options.NamePublishOption) (ipns.Name, error)

	// Resolve attempts to resolve the newest version of the specified name
	Resolve(ctx context.Context, name string, opts ...options.NameResolveOption) (path.Path, error)

	// Search is a version of Resolve which outputs paths as they are discovered,
	// reducing the time to first entry
	//
	// Note: by default, all paths read from the channel are considered unsafe,
	// except the latest (last path in channel read buffer).
	Search(ctx context.Context, name string, opts ...options.NameResolveOption) (<-chan IpnsResult, error)
}
