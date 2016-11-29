package io

import (
	"context"

	mdag "github.com/ipfs/go-ipfs/merkledag"
	format "github.com/ipfs/go-ipfs/unixfs"
	cid "gx/ipfs/QmcTcsTvfaeEBRFo1TkFgT8sRmgi1n1LTZpecfVP8fzpGD/go-cid"
)

type directoryBuilder struct {
	dserv   mdag.DAGService
	dirnode *mdag.ProtoNode
}

// NewEmptyDirectory returns an empty merkledag Node with a folder Data chunk
func NewEmptyDirectory() *mdag.ProtoNode {
	nd := new(mdag.ProtoNode)
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
func (d *directoryBuilder) AddChild(ctx context.Context, name string, c *cid.Cid) error {
	cnode, err := d.dserv.Get(ctx, c)
	if err != nil {
		return err
	}

	cnpb, ok := cnode.(*mdag.ProtoNode)
	if !ok {
		return mdag.ErrNotProtobuf
	}

	return d.dirnode.AddNodeLinkClean(name, cnpb)
}

// GetNode returns the root of this directoryBuilder
func (d *directoryBuilder) GetNode() *mdag.ProtoNode {
	return d.dirnode
}
