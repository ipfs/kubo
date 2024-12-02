package iface

import (
	"context"

	"github.com/ipfs/boxo/path"
	"github.com/ipfs/kubo/core/coreiface/options"
)

// ChangeType denotes type of change in ObjectChange
type ChangeType int

const (
	// DiffAdd is set when a link was added to the graph
	DiffAdd ChangeType = iota

	// DiffRemove is set when a link was removed from the graph
	DiffRemove

	// DiffMod is set when a link was changed in the graph
	DiffMod
)

// ObjectChange represents a change ia a graph
type ObjectChange struct {
	// Type of the change, either:
	// * DiffAdd - Added a link
	// * DiffRemove - Removed a link
	// * DiffMod - Modified a link
	Type ChangeType

	// Path to the changed link
	Path string

	// Before holds the link path before the change. Note that when a link is
	// added, this will be nil.
	Before path.ImmutablePath

	// After holds the link path after the change. Note that when a link is
	// removed, this will be nil.
	After path.ImmutablePath
}

// ObjectAPI specifies the interface to MerkleDAG and contains useful utilities
// for manipulating MerkleDAG data structures.
type ObjectAPI interface {
	// AddLink adds a link under the specified path. child path can point to a
	// subdirectory within the patent which must be present (can be overridden
	// with WithCreate option).
	AddLink(ctx context.Context, base path.Path, name string, child path.Path, opts ...options.ObjectAddLinkOption) (path.ImmutablePath, error)

	// RmLink removes a link from the node
	RmLink(ctx context.Context, base path.Path, link string) (path.ImmutablePath, error)

	// Diff returns a set of changes needed to transform the first object into the
	// second.
	Diff(context.Context, path.Path, path.Path) ([]ObjectChange, error)
}
