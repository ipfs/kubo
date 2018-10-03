package iface

import (
	"context"

	options "github.com/ipfs/go-ipfs/core/coreapi/interface/options"

	files "gx/ipfs/QmSP88ryZkHSRn1fnngAaV2Vcn63WUJzAavnRM9CVdU1Ky/go-ipfs-cmdkit/files"
	ipld "gx/ipfs/QmdDXJs4axxefSPgK6Y1QhpJWKuDPnGJiqgq4uncb4rFHL/go-ipld-format"
)

// UnixfsAPI is the basic interface to immutable files in IPFS
// NOTE: This API is heavily WIP, things are guaranteed to break frequently
type UnixfsAPI interface {
	// Add imports the data from the reader into merkledag file
	//
	// TODO: a long useful comment on how to use this for many different scenarios
	Add(context.Context, files.File, ...options.UnixfsAddOption) (ResolvedPath, error)

	// Get returns a read-only handle to a file tree referenced by a path
	//
	// Note that some implementations of this API may apply the specified context
	// to operations performed on the returned file
	Get(context.Context, Path) (files.File, error)

	// Cat returns a reader for the file
	Cat(context.Context, Path) (Reader, error)

	// Ls returns the list of links in a directory
	Ls(context.Context, Path) ([]*ipld.Link, error)
}
