package iface

import (
	"context"
	"errors"

	"github.com/ipfs/boxo/ipns"
	"github.com/ipfs/boxo/path"
	"github.com/ipfs/kubo/core/coreiface/options"
)

var ErrResolveFailed = errors.New("could not resolve name")

// mountPublishKey is a context key that lets the IPNS FUSE mount's
// internal MFS republisher bypass the "cannot manually publish while
// IPNS is mounted" guard in the Name API. Without this bypass the
// guard blocks the mount's own publishes and silently drops IPNS
// updates, causing data written through the FUSE mount to be lost
// on daemon restart (see https://github.com/ipfs/kubo/issues/2168).
//
// TODO: the /ipns/ FUSE mount does not detect changes when a
// locally-owned key is published via `ipfs name publish` (RPC/CLI).
// A larger refactor is needed so the mountpoint's MFS representation
// is updated to reflect external publishes to locally-owned keys,
// rather than silently overwriting them on the next MFS flush.
type mountPublishKey struct{}

// ContextWithMountPublish marks ctx as originating from the FUSE
// mount's internal publish path. Name.Publish implementations should
// skip the "cannot publish while IPNS is mounted" guard for such
// contexts.
func ContextWithMountPublish(ctx context.Context) context.Context {
	return context.WithValue(ctx, mountPublishKey{}, true)
}

// IsMountPublish reports whether ctx was marked by
// [ContextWithMountPublish].
func IsMountPublish(ctx context.Context) bool {
	return ctx.Value(mountPublishKey{}) != nil
}

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
