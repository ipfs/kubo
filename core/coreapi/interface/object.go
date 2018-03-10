package iface

import (
	"context"
	"io"

	options "github.com/ipfs/go-ipfs/core/coreapi/interface/options"

	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"
)

// ObjectStat provides information about dag nodes
type ObjectStat struct {
	// Cid is the CID of the node
	Cid *cid.Cid

	// NumLinks is number of links the node contains
	NumLinks int

	// BlockSize is size of the raw serialized node
	BlockSize int

	// LinksSize is size of the links block section
	LinksSize int

	// DataSize is the size of data block section
	DataSize int

	// CumulativeSize is size of the tree (BlockSize + link sizes)
	CumulativeSize int
}

// ObjectAPI specifies the interface to MerkleDAG and contains useful utilities
// for manipulating MerkleDAG data structures.
type ObjectAPI interface {
	// New creates new, empty (by default) dag-node.
	New(context.Context, ...options.ObjectNewOption) (ipld.Node, error)

	// WithType is an option for New which allows to change the type of created
	// dag node.
	//
	// Supported types:
	// * 'empty' - Empty node
	// * 'unixfs-dir' - Empty UnixFS directory
	WithType(string) options.ObjectNewOption

	// Put imports the data into merkledag
	Put(context.Context, io.Reader, ...options.ObjectPutOption) (Path, error)

	// WithInputEnc is an option for Put which specifies the input encoding of the
	// data. Default is "json".
	//
	// Supported encodings:
	// * "protobuf"
	// * "json"
	WithInputEnc(e string) options.ObjectPutOption

	// WithDataType specifies the encoding of data field when using Josn or XML
	// input encoding.
	//
	// Supported types:
	// * "text" (default)
	// * "base64"
	WithDataType(t string) options.ObjectPutOption

	// Get returns the node for the path
	Get(context.Context, Path) (ipld.Node, error)

	// Data returns reader for data of the node
	Data(context.Context, Path) (io.Reader, error)

	// Links returns lint or links the node contains
	Links(context.Context, Path) ([]*ipld.Link, error)

	// Stat returns information about the node
	Stat(context.Context, Path) (*ObjectStat, error)

	// AddLink adds a link under the specified path. child path can point to a
	// subdirectory within the patent which must be present (can be overridden
	// with WithCreate option).
	AddLink(ctx context.Context, base Path, name string, child Path, opts ...options.ObjectAddLinkOption) (Path, error)

	// WithCreate is an option for AddLink which specifies whether create required
	// directories for the child
	WithCreate(create bool) options.ObjectAddLinkOption

	// RmLink removes a link from the node
	RmLink(ctx context.Context, base Path, link string) (Path, error)

	// AppendData appends data to the node
	AppendData(context.Context, Path, io.Reader) (Path, error)

	// SetData sets the data contained in the node
	SetData(context.Context, Path, io.Reader) (Path, error)
}
