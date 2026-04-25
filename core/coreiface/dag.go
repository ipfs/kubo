package iface

import (
	"context"

	"github.com/ipfs/boxo/path"
	ipld "github.com/ipfs/go-ipld-format"
)

// DagStatResult is the result of DAG Stat: size and block count for a single root.
type DagStatResult struct {
	NumBlocks int64
	Size      uint64
}

// APIDagService extends ipld.DAGService
type APIDagService interface {
	ipld.DAGService

	// Pinning returns special NodeAdder which recursively pins added nodes
	Pinning() ipld.NodeAdder

	// Stat walks the DAG from the given path and returns total size and block count.
	Stat(ctx context.Context, p path.Path) (*DagStatResult, error)
}
