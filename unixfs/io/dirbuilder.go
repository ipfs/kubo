package io

import (
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	key "github.com/ipfs/go-ipfs/blocks/key"
	dag "github.com/ipfs/go-ipfs/merkledag"
	format "github.com/ipfs/go-ipfs/unixfs"
)

type directoryBuilder struct {
	dserv   dag.DAGService
	dirnode *dag.Node
}

// NewEmptyDirectory returns an empty merkledag Node with a folder Data chunk
func NewEmptyDirectory() *dag.Node {
	return &dag.Node{Data: format.FolderPBData()}
}

// NewDirectory returns a directoryBuilder. It needs a DAGService to add the Children
func NewDirectory(dserv dag.DAGService) *directoryBuilder {
	db := new(directoryBuilder)
	db.dserv = dserv
	db.dirnode = NewEmptyDirectory()
	return db
}

// AddChild adds a (name, key)-pair to the root node.
func (d *directoryBuilder) AddChild(ctx context.Context, name string, k key.Key) error {
	cnode, err := d.dserv.Get(ctx, k)
	if err != nil {
		return err
	}

	return d.dirnode.AddNodeLinkClean(name, cnode)
}

// GetNode returns the root of this directoryBuilder
func (d *directoryBuilder) GetNode() *dag.Node {
	return d.dirnode
}
