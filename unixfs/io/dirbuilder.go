package io

import (
	"time"

	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	mdag "github.com/ipfs/go-ipfs/merkledag"
	format "github.com/ipfs/go-ipfs/unixfs"
	u "github.com/ipfs/go-ipfs/util"
)

type directoryBuilder struct {
	dserv   mdag.DAGService
	dirnode *mdag.Node
}

// NewEmptyDirectory returns an empty merkledag Node with a folder Data chunk
func NewEmptyDirectory() *mdag.Node {
	return &mdag.Node{Data: format.FolderPBData()}
}

// NewDirectory returns a directoryBuilder. It needs a DAGService to add the Children
func NewDirectory(dserv mdag.DAGService) *directoryBuilder {
	db := new(directoryBuilder)
	db.dserv = dserv
	db.dirnode = NewEmptyDirectory()
	return db
}

// AddChild adds a (name, key)-pair to the root node.
func (d *directoryBuilder) AddChild(name string, k u.Key) error {
	// TODO(cryptix): consolidate context managment
	ctx, cancel := context.WithTimeout(context.TODO(), time.Minute)
	defer cancel()

	cnode, err := d.dserv.Get(ctx, k)
	if err != nil {
		return err
	}

	err = d.dirnode.AddNodeLinkClean(name, cnode)
	if err != nil {
		return err
	}

	return nil
}

// GetNode returns the root of this directoryBuilder
func (d *directoryBuilder) GetNode() *mdag.Node {
	return d.dirnode
}
