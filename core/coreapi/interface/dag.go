package iface

import (
	"context"
	"io"

	options "github.com/ipfs/go-ipfs/core/coreapi/interface/options"

	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"
)

// DagAPI specifies the interface to IPLD
type DagAPI interface {
	// Put inserts data using specified format and input encoding.
	// Unless used with WithCodec or WithHash, the defaults "dag-cbor" and
	// "sha256" are used.
	Put(ctx context.Context, src io.Reader, opts ...options.DagPutOption) (Path, error)

	// Get attempts to resolve and get the node specified by the path
	Get(ctx context.Context, path Path) (ipld.Node, error)

	// Tree returns list of paths within a node specified by the path.
	Tree(ctx context.Context, path Path, opts ...options.DagTreeOption) ([]Path, error)
}
