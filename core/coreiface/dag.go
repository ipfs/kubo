package iface

import (
	"context"
	"io"

	"github.com/ipfs/go-ipfs/core/coreapi/interface/options"

	ipld "gx/ipfs/QmZtNq8dArGfnpCZfx2pUNY7UcjGhVp5qqwQ4hH6mpTMRQ/go-ipld-format"
)

// DagOps groups operations that can be batched together
type DagOps interface {
	// Put inserts data using specified format and input encoding.
	// Unless used with WithCodec or WithHash, the defaults "dag-cbor" and
	// "sha256" are used.
	Put(ctx context.Context, src io.Reader, opts ...options.DagPutOption) (ResolvedPath, error)
}

// DagBatch is the batching version of DagAPI. All implementations of DagBatch
// should be threadsafe
type DagBatch interface {
	DagOps

	Commit(ctx context.Context) error
}

// DagAPI specifies the interface to IPLD
type DagAPI interface {
	DagOps

	// Get attempts to resolve and get the node specified by the path
	Get(ctx context.Context, path Path) (ipld.Node, error)

	// Tree returns list of paths within a node specified by the path.
	Tree(ctx context.Context, path Path, opts ...options.DagTreeOption) ([]Path, error)

	Batch(ctx context.Context) DagBatch
}
