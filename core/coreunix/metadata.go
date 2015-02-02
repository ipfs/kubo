package coreunix

import (
	dag "github.com/jbenet/go-ipfs/merkledag"
	ft "github.com/jbenet/go-ipfs/unixfs"
	u "github.com/jbenet/go-ipfs/util"
)

func AddMetadataTo(key u.Key, m *ft.Metadata, dserv dag.DAGService) (u.Key, error) {
	nd, err := dserv.Get(key)
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

	return dserv.Add(mdnode)
}

func Metadata(key u.Key, dserv dag.DAGService) (*ft.Metadata, error) {
	nd, err := dserv.Get(key)
	if err != nil {
		return nil, err
	}

	return ft.MetadataFromBytes(nd.Data)
}
