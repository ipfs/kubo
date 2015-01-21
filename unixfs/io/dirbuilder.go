package io

import (
	mdag "github.com/jbenet/go-ipfs/merkledag"
	format "github.com/jbenet/go-ipfs/unixfs"
	u "github.com/jbenet/go-ipfs/util"
)

type directoryBuilder struct {
	dserv   mdag.DAGService
	dirnode *mdag.Node
}

func NewDirectory(dserv mdag.DAGService) *directoryBuilder {
	db := new(directoryBuilder)
	db.dserv = dserv
	db.dirnode = new(mdag.Node)
	db.dirnode.Data = format.FolderPBData()
	return db
}

func (d *directoryBuilder) AddChild(name string, k u.Key) error {
	cnode, err := d.dserv.Get(k)
	if err != nil {
		return err
	}

	err = d.dirnode.AddNodeLinkClean(name, cnode)
	if err != nil {
		return err
	}

	return nil
}

func (d *directoryBuilder) GetNode() *mdag.Node {
	return d.dirnode
}
