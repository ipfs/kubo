package coreunix

import (
	core "github.com/ipfs/go-ipfs/core"
	ft "gx/ipfs/QmWJRM6rLjXGEXb5JkKu17Y68eJtCFcKPyRhb8JH2ELZ2Q/go-unixfs"
	dag "gx/ipfs/QmXkZeJmx4c3ddjw81DQMUpM1e5LjAack5idzZYWUb2qAJ/go-merkledag"
	cid "gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
)

func AddMetadataTo(n *core.IpfsNode, skey string, m *ft.Metadata) (string, error) {
	c, err := cid.Decode(skey)
	if err != nil {
		return "", err
	}

	nd, err := n.DAG.Get(n.Context(), c)
	if err != nil {
		return "", err
	}

	mdnode := new(dag.ProtoNode)
	mdata, err := ft.BytesForMetadata(m)
	if err != nil {
		return "", err
	}

	mdnode.SetData(mdata)
	if err := mdnode.AddNodeLink("file", nd); err != nil {
		return "", err
	}

	err = n.DAG.Add(n.Context(), mdnode)
	if err != nil {
		return "", err
	}

	return mdnode.Cid().String(), nil
}

func Metadata(n *core.IpfsNode, skey string) (*ft.Metadata, error) {
	c, err := cid.Decode(skey)
	if err != nil {
		return nil, err
	}

	nd, err := n.DAG.Get(n.Context(), c)
	if err != nil {
		return nil, err
	}

	pbnd, ok := nd.(*dag.ProtoNode)
	if !ok {
		return nil, dag.ErrNotProtobuf
	}

	return ft.MetadataFromBytes(pbnd.Data())
}
