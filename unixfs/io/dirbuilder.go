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

func NewDirectory(dserv mdag.DAGService) *directoryBuilder {
	db := new(directoryBuilder)
	db.dserv = dserv
	db.dirnode = new(mdag.Node)
	db.dirnode.Data = format.FolderPBData()
	return db
}

func (d *directoryBuilder) AddChild(name string, k u.Key) error {
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

func (d *directoryBuilder) GetNode() *mdag.Node {
	return d.dirnode
}
