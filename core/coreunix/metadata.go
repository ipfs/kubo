package coreunix

import (
	"time"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	core "github.com/ipfs/go-ipfs/core"
	dag "github.com/ipfs/go-ipfs/merkledag"
	ft "github.com/ipfs/go-ipfs/unixfs"
	u "github.com/ipfs/go-ipfs/util"
)

func AddMetadataTo(n *core.IpfsNode, key string, m *ft.Metadata) (string, error) {
	ukey := u.B58KeyDecode(key)

	ctx, cancel := context.WithTimeout(context.TODO(), time.Minute)
	defer cancel()
	nd, err := n.DAG.Get(ctx, ukey)
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

	nk, err := n.DAG.Add(mdnode)
	if err != nil {
		return "", err
	}

	return nk.B58String(), nil
}

func Metadata(n *core.IpfsNode, key string) (*ft.Metadata, error) {
	ukey := u.B58KeyDecode(key)

	ctx, cancel := context.WithTimeout(context.TODO(), time.Minute)
	defer cancel()
	nd, err := n.DAG.Get(ctx, ukey)
	if err != nil {
		return nil, err
	}

	return ft.MetadataFromBytes(nd.Data)
}
