package iface

import (
	"context"

	"github.com/ipfs/boxo/files"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/kubo/core/coreiface/options"
)

// DagImportResult represents the result of importing roots or stats from CAR files.
// Each result has either Root or Stats set, never both.
type DagImportResult struct {
	Root  *DagImportRoot
	Stats *DagImportStats
}

// DagImportRoot represents a root CID from a CAR file header
type DagImportRoot struct {
	Cid         cid.Cid
	PinErrorMsg string
}

// DagImportStats contains statistics about the import operation
type DagImportStats struct {
	BlockCount      uint64
	BlockBytesCount uint64
}

// APIDagService extends ipld.DAGService
type APIDagService interface {
	ipld.DAGService

	// Pinning returns special NodeAdder which recursively pins added nodes
	Pinning() ipld.NodeAdder

	// Import imports data from CAR files.
	// Returns a channel that streams results for each root CID found in CAR headers,
	// and optionally stats at the end if requested via options.
	// Supports importing multiple CAR files, each with multiple roots.
	Import(context.Context, files.File, ...options.DagImportOption) (<-chan DagImportResult, error)
}
