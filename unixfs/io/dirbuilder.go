package io

import (
	"golang.org/x/net/context"

	mdag "github.com/ipfs/go-ipfs/merkledag"
	format "github.com/ipfs/go-ipfs/unixfs"
	key "github.com/ipfs/go-key"
)

type directoryBuilder struct {
	dserv   mdag.DAGService
	dirnode *mdag.Node
}

// NewEmptyDirectory returns an empty merkledag Node with a folder Data chunk
func NewEmptyDirectory() *mdag.Node {
	nd := new(mdag.Node)
	nd.SetData(format.FolderPBData())
	return nd
}

// NewDirectory returns a directoryBuilder. It needs a DAGService to add the Children
func NewDirectory(dserv mdag.DAGService) *directoryBuilder {
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
func (d *directoryBuilder) GetNode() *mdag.Node {
	return d.dirnode
}
