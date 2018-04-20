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

	// Namespace returns the first component of the path.
	//
	// For example path "/ipfs/QmHash", calling Namespace() will return "ipfs"
	Namespace() string

	// Mutable returns false if the data pointed to by this path in guaranteed
	// to not change.
	//
	// Note that resolved mutable path can be immutable.
	Mutable() bool
}

// ResolvedPath is a resolved Path
type ResolvedPath interface {
	// Cid returns the CID of the object referenced by the path.
	//
	// Example:
	// If you have 3 linked objects: QmRoot -> A -> B, and resolve path
	// "/ipfs/QmRoot/A/B", the Cid method will return the CID of object B
	Cid() *cid.Cid

	// Root returns the CID of the root object of the path
	//
	// Example:
	// If you have 3 linked objects: QmRoot -> A -> B, and resolve path
	// "/ipfs/QmRoot/A/B", the Root method will return the CID of object QmRoot
	Root() *cid.Cid

	// Remainder returns unresolved part of the path
	//
	// Example:
	// If you have 2 linked objects: QmRoot -> A, where A is a CBOR node
	// containing the following data:
	//
	// {"foo": {"bar": 42}}
	//
	// When resolving "/ipld/QmRoot/A/foo/bar", Remainder will return "foo/bar"
	Remainder() string

	Path
}
