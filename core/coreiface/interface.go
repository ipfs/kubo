// Package iface defines IPFS Core API which is a set of interfaces used to
// interact with IPFS nodes.
package iface

import (
	"context"
	"errors"
	"io"

	options "github.com/ipfs/go-ipfs/core/coreapi/interface/options"

	ipld "gx/ipfs/QmNwUEK7QbwSqyKBu3mMtToo8SUc6wQJ7gdZq4gGGJqfnf/go-ipld-format"
	cid "gx/ipfs/QmeSrf6pzut73u6zLQkRFQ3ygt3k6XFT2kjdYP8Tnkwwyg/go-cid"
)

// Path is a generic wrapper for paths used in the API. A path can be resolved
// to a CID using one of Resolve functions in the API.
type Path interface {
	String() string
	Cid() *cid.Cid
	Root() *cid.Cid
	Resolved() bool
}

// TODO: should we really copy these?
//       if we didn't, godoc would generate nice links straight to go-ipld-format
type Node ipld.Node
type Link ipld.Link

type Reader interface {
	io.ReadSeeker
	io.Closer
}

// CoreAPI defines an unified interface to IPFS for Go programs.
type CoreAPI interface {
	// Unixfs returns an implementation of Unixfs API
	Unixfs() UnixfsAPI
	Dag() DagAPI

	// ResolvePath resolves the path using Unixfs resolver
	ResolvePath(context.Context, Path) (Path, error)

	// ResolveNode resolves the path (if not resolved already) using Unixfs
	// resolver, gets and returns the resolved Node
	ResolveNode(context.Context, Path) (Node, error)
}

// UnixfsAPI is the basic interface to immutable files in IPFS
type UnixfsAPI interface {
	// Add imports the data from the reader into merkledag file
	Add(context.Context, io.Reader) (Path, error)

	// Cat returns a reader for the file
	Cat(context.Context, Path) (Reader, error)

	// Ls returns the list of links in a directory
	Ls(context.Context, Path) ([]*Link, error)
}

// DagAPI specifies the interface to IPLD
type DagAPI interface {
	// Put inserts data using specified format and input encoding.
	// If format is not specified (nil), default dag-cbor/sha256 is used
	Put(ctx context.Context, src io.Reader, opts ...options.DagPutOption) ([]Node, error)

	// WithInputEnc is an option for Put which specifies the input encoding of the
	// data. Default is "json", most formats/codecs support "raw"
	WithInputEnc(enc string) options.DagPutOption

	// WithCodec is an option for Put which specifies the multicodec to use to
	// serialize the object. Default is cid.DagCBOR (0x71)
	WithCodec(codec uint64) options.DagPutOption

	// WithHash is an option for Put which specifies the multihash settings to use
	// when hashing the object. Default is based on the codec used
	// (mh.SHA2_256 (0x12) for DagCBOR). If mhLen is set to -1, default length for
	// the hash will be used
	WithHash(mhType uint64, mhLen int) options.DagPutOption

	// Get attempts to resolve and get the node specified by the path
	Get(ctx context.Context, path Path) (Node, error)

	// Tree returns list of paths within a node specified by the path.
	Tree(ctx context.Context, path Path, opts ...options.DagTreeOption) ([]Path, error)

	// WithDepth is an option for Tree which specifies maximum depth of the
	// returned tree. Default is -1 (no depth limit)
	WithDepth(depth int) options.DagTreeOption
}

// type ObjectAPI interface {
// 	New() (cid.Cid, Object)
// 	Get(string) (Object, error)
// 	Links(string) ([]*Link, error)
// 	Data(string) (Reader, error)
// 	Stat(string) (ObjectStat, error)
// 	Put(Object) (cid.Cid, error)
// 	SetData(string, Reader) (cid.Cid, error)
// 	AppendData(string, Data) (cid.Cid, error)
// 	AddLink(string, string, string) (cid.Cid, error)
// 	RmLink(string, string) (cid.Cid, error)
// }

// type ObjectStat struct {
// 	Cid            cid.Cid
// 	NumLinks       int
// 	BlockSize      int
// 	LinksSize      int
// 	DataSize       int
// 	CumulativeSize int
// }

var ErrIsDir = errors.New("object is a directory")
var ErrOffline = errors.New("can't resolve, ipfs node is offline")
