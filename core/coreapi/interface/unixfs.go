package iface

import (
	"context"
	"github.com/ipfs/go-ipfs/core/coreapi/interface/options"

	"gx/ipfs/QmQ1JnYpnzkaurjW1yxkQxC2w3K1PorNE1nv1vaP5Le7sq/go-unixfs/pb"
	ipld "gx/ipfs/QmRL22E4paat7ky7vx9MLpR97JHHbFPrg3ytFQw6qp1y1s/go-ipld-format"
	"gx/ipfs/QmaXvvAVAQ5ABqM5xtjYmV85xmN5MkWAZsX9H9Fwo4FVXp/go-ipfs-files"
)

type AddEvent struct {
	Name  string
	Path  ResolvedPath `json:",omitempty"`
	Bytes int64        `json:",omitempty"`
	Size  string       `json:",omitempty"`
}

type LsLink struct {
	Link *ipld.Link
	Size uint64
	Type unixfs_pb.Data_DataType

	Err error
}

// UnixfsAPI is the basic interface to immutable files in IPFS
// NOTE: This API is heavily WIP, things are guaranteed to break frequently
type UnixfsAPI interface {
	// Add imports the data from the reader into merkledag file
	//
	// TODO: a long useful comment on how to use this for many different scenarios
	Add(context.Context, files.Node, ...options.UnixfsAddOption) (ResolvedPath, error)

	// Get returns a read-only handle to a file tree referenced by a path
	//
	// Note that some implementations of this API may apply the specified context
	// to operations performed on the returned file
	Get(context.Context, Path) (files.Node, error)

	// Ls returns the list of links in a directory. Links aren't guaranteed to be
	// returned in order
	Ls(context.Context, Path, ...options.UnixfsLsOption) (<-chan LsLink, error)
}
