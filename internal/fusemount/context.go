// Package fusemount provides internal helpers shared between the FUSE
// mount layer and the core API.  It lives under internal/ so that
// external consumers of kubo cannot bypass publish guards.
package fusemount

import "context"

// publishKey is a context key that lets the IPNS FUSE mount's
// internal MFS republisher bypass the "cannot manually publish while
// IPNS is mounted" guard in the Name API.  Without this bypass the
// guard blocks the mount's own publishes and silently drops IPNS
// updates, causing data written through the FUSE mount to be lost
// on daemon restart (see https://github.com/ipfs/kubo/issues/2168).
//
// TODO: the /ipns/ FUSE mount does not detect changes when a
// locally-owned key is published via `ipfs name publish` (RPC/CLI).
// A larger refactor is needed so the mountpoint's MFS representation
// is updated to reflect external publishes to locally-owned keys,
// rather than silently overwriting them on the next MFS flush.
type publishKey struct{}

// ContextWithPublish marks ctx as originating from the FUSE mount's
// internal publish path.
func ContextWithPublish(ctx context.Context) context.Context {
	return context.WithValue(ctx, publishKey{}, true)
}

// IsPublish reports whether ctx was marked by [ContextWithPublish].
func IsPublish(ctx context.Context) bool {
	return ctx.Value(publishKey{}) != nil
}
