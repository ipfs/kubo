package iface

import (
	"context"
	"github.com/ipfs/interface-go-ipfs-core/options"

	"github.com/ipfs/go-ipfs-files"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-unixfs"
)

type AddEvent struct {
	Name  string
	Path  ResolvedPath `json:",omitempty"`
	Bytes int64        `json:",omitempty"`
	Size  string       `json:",omitempty"`
}

type FileType int32

const (
	TRaw       = FileType(unixfs.TRaw)
	TFile      = FileType(unixfs.TFile)
	TDirectory = FileType(unixfs.TDirectory)
	TMetadata  = FileType(unixfs.TMetadata)
	TSymlink   = FileType(unixfs.TSymlink)
	THAMTShard = FileType(unixfs.THAMTShard)
)

type LsLink struct {
	Link *ipld.Link
	Size uint64
	Type FileType

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
