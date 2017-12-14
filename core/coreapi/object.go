package coreapi

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/ioutil"

	"github.com/ipfs/go-ipfs/merkledag/utils"

	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	dag "github.com/ipfs/go-ipfs/merkledag"
	ft "github.com/ipfs/go-ipfs/unixfs"
)

type ObjectAPI CoreAPI

func (api *ObjectAPI) New(ctx context.Context) (coreiface.Node, error) {
	node := new(dag.ProtoNode)

	_, err := api.node.DAG.Add(node)
	if err != nil {
		return nil, err
	}
	return node, nil
}

func (api *ObjectAPI) Put(context.Context, coreiface.Node) error {
	return errors.New("todo") // TODO: what should this method take? Should we just redir to dag-put?f
}

func (api *ObjectAPI) Get(ctx context.Context, path coreiface.Path) (coreiface.Node, error) {
	return api.core().ResolveNode(ctx, path)
}

func (api *ObjectAPI) Data(ctx context.Context, path coreiface.Path) (io.Reader, error) {
	nd, err := api.core().ResolveNode(ctx, path)
	if err != nil {
		return nil, err
	}

	pbnd, ok := nd.(*dag.ProtoNode)
	if !ok {
		return nil, dag.ErrNotProtobuf
	}

	return bytes.NewReader(pbnd.Data()), nil
}

func (api *ObjectAPI) Links(ctx context.Context, path coreiface.Path) ([]*coreiface.Link, error) {
	nd, err := api.core().ResolveNode(ctx, path)
	if err != nil {
		return nil, err
	}

	links := nd.Links()
	out := make([]*coreiface.Link, len(links))
	for n, l := range links {
		out[n] = (*coreiface.Link)(l)
	}

	return out, nil
}

func (api *ObjectAPI) Stat(ctx context.Context, path coreiface.Path) (*coreiface.ObjectStat, error) {
	nd, err := api.core().ResolveNode(ctx, path)
	if err != nil {
		return nil, err
	}

	stat, err := nd.Stat()
	if err != nil {
		return nil, err
	}

	out := &coreiface.ObjectStat{
		Cid:            nd.Cid(),
		NumLinks:       stat.NumLinks,
		BlockSize:      stat.BlockSize,
		LinksSize:      stat.LinksSize,
		DataSize:       stat.DataSize,
		CumulativeSize: stat.CumulativeSize,
	}

	return out, nil
}

func (api *ObjectAPI) AddLink(ctx context.Context, base coreiface.Path, name string, child coreiface.Path, create bool) (coreiface.Node, error) {
	rootNd, err := api.core().ResolveNode(ctx, base)
	if err != nil {
		return nil, err
	}

	childNd, err := api.core().ResolveNode(ctx, child)
	if err != nil {
		return nil, err
	}

	rootPb, ok := rootNd.(*dag.ProtoNode)
	if !ok {
		return nil, dag.ErrNotProtobuf
	}

	var createfunc func() *dag.ProtoNode
	if create {
		createfunc = ft.EmptyDirNode
	}

	e := dagutils.NewDagEditor(rootPb, api.node.DAG)

	err = e.InsertNodeAtPath(ctx, name, childNd, createfunc)
	if err != nil {
		return nil, err
	}

	nnode, err := e.Finalize(api.node.DAG)
	if err != nil {
		return nil, err
	}

	return nnode, nil
}

func (api *ObjectAPI) RmLink(ctx context.Context, root coreiface.Path, link string) (coreiface.Node, error) {
	rootNd, err := api.core().ResolveNode(ctx, root)
	if err != nil {
		return nil, err
	}

	rootPb, ok := rootNd.(*dag.ProtoNode)
	if !ok {
		return nil, dag.ErrNotProtobuf
	}

	e := dagutils.NewDagEditor(rootPb, api.node.DAG)

	err = e.RmLink(ctx, link)
	if err != nil {
		return nil, err
	}

	nnode, err := e.Finalize(api.node.DAG)
	if err != nil {
		return nil, err
	}

	return nnode, nil
}

func (api *ObjectAPI) AppendData(ctx context.Context, path coreiface.Path, r io.Reader) (coreiface.Node, error) {
	return api.patchData(ctx, path, r, true)
}

func (api *ObjectAPI) SetData(ctx context.Context, path coreiface.Path, r io.Reader) (coreiface.Node, error) {
	return api.patchData(ctx, path, r, false)
}

func (api *ObjectAPI) patchData(ctx context.Context, path coreiface.Path, r io.Reader, appendData bool) (coreiface.Node, error) {
	nd, err := api.core().ResolveNode(ctx, path)
	if err != nil {
		return nil, err
	}

	pbnd, ok := nd.(*dag.ProtoNode)
	if !ok {
		return nil, dag.ErrNotProtobuf
	}

	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	if appendData {
		data = append(pbnd.Data(), data...)
	}
	pbnd.SetData(data)

	_, err = api.node.DAG.Add(pbnd)
	if err != nil {
		return nil, err
	}

	return pbnd, nil
}

func (api *ObjectAPI) core() coreiface.CoreAPI {
	return (*CoreAPI)(api)
}
