package coreunix

import (
	core "github.com/jbenet/go-ipfs/core"
	dag "github.com/jbenet/go-ipfs/merkledag"
	ft "github.com/jbenet/go-ipfs/unixfs"
	u "github.com/jbenet/go-ipfs/util"
)

func AddMetadataTo(key u.Key, m *ft.Metadata, n *core.IpfsNode) (u.Key, error) {
	nd, err := n.DAG.Get(key)
	if err != nil {
		return "", err
	}

	mdnode := new(dag.Node)
	mdata, err := ft.BytesForMetadata(m)
	if err != nil {
		return "", err
	}

	mdnode.Data = mdata
	err = mdnode.AddNodeLinkClean("file", nd)
	if err != nil {
		return "", err
	}

	return n.DAG.Add(mdnode)
}

func Metadata(key u.Key, n *core.IpfsNode) (*ft.Metadata, error) {
	nd, err := n.DAG.Get(key)
	if err != nil {
		return nil, err
	}

	return ft.MetadataFromBytes(nd.Data)
}
