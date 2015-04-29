package unixfs

import (
	"io"

	core "github.com/ipfs/go-ipfs/core"
	importer "github.com/ipfs/go-ipfs/importer"
	chunk "github.com/ipfs/go-ipfs/importer/chunk"
	dag "github.com/ipfs/go-ipfs/merkledag"
)

// Add builds a merkledag from a reader, pinning all objects to the
// local datastore.  Returns the root node.
func AddFromReader(node *core.IpfsNode, reader io.Reader) (*dag.Node, error) {
	fileNode, err := importer.BuildDagFromReader(
		reader,
		node.DAG,
		node.Pinning.GetManual(),
		chunk.DefaultSplitter,
	)
	if err != nil {
		return nil, err
	}
	if err := node.Pinning.Flush(); err != nil {
		return nil, err
	}
	return fileNode, nil
}
