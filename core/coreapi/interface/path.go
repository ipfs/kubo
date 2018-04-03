package iface

import (
	cid "gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
)

// Path is a generic wrapper for paths used in the API. A path can be resolved
// to a CID using one of Resolve functions in the API.
//
// Paths must be prefixed with a valid prefix:
//
// * /ipfs - Immutable unixfs path (files)
// * /ipld - Immutable ipld path (data)
// * /ipns - Mutable names. Usually resolves to one of the immutable paths
//TODO: /local (MFS)
type Path interface {
	// String returns the path as a string.
	String() string

	// Namespace returns the first component of the path
	Namespace() string

	// Mutable returns false if the data pointed to by this path in guaranteed
	// to not change.
	//
	// Note that resolved mutable path can be immutable.
	Mutable() bool
}

// ResolvedPath is a resolved Path
type ResolvedPath interface {
	// Cid returns the CID referred to by path
	Cid() *cid.Cid

	// Root returns the CID of root path
	Root() *cid.Cid

	// Remainder returns unresolved part of the path
	Remainder() string

	Path
}
