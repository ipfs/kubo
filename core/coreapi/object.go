package coreapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"

	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	caopts "github.com/ipfs/go-ipfs/core/coreapi/interface/options"
	dag "github.com/ipfs/go-ipfs/merkledag"
	dagutils "github.com/ipfs/go-ipfs/merkledag/utils"
	ft "github.com/ipfs/go-ipfs/unixfs"

	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"
)

const inputLimit = 2 << 20

type ObjectAPI CoreAPI

type Link struct {
	Name, Hash string
	Size       uint64
}

type Node struct {
	Links []Link
	Data  string
}

func (api *ObjectAPI) New(ctx context.Context, opts ...caopts.ObjectNewOption) (ipld.Node, error) {
	options, err := caopts.ObjectNewOptions(opts...)
	if err != nil {
		return nil, err
	}

	var n ipld.Node
	switch options.Type {
	case "empty":
		n = new(dag.ProtoNode)
	case "unixfs-dir":
		n = ft.EmptyDirNode()
	}

	err = api.node.DAG.Add(ctx, n)
	if err != nil {
		return nil, err
	}
	return n, nil
}

func (api *ObjectAPI) Put(ctx context.Context, src io.Reader, opts ...caopts.ObjectPutOption) (coreiface.Path, error) {
	options, err := caopts.ObjectPutOptions(opts...)
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadAll(io.LimitReader(src, inputLimit+10))
	if err != nil {
		return nil, err
	}

	var dagnode *dag.ProtoNode
	switch options.InputEnc {
	case "json":
		node := new(Node)
		err = json.Unmarshal(data, node)
		if err != nil {
			return nil, err
		}

		// check that we have data in the Node to add
		// otherwise we will add the empty object without raising an error
		if nodeEmpty(node) {
			return nil, errors.New("no data or links in this node")
		}

		dagnode, err = deserializeNode(node, options.DataType)
		if err != nil {
			return nil, err
		}

	case "protobuf":
		dagnode, err = dag.DecodeProtobuf(data)

	case "xml":
		node := new(Node)
		err = xml.Unmarshal(data, node)
		if err != nil {
			return nil, err
		}

		// check that we have data in the Node to add
		// otherwise we will add the empty object without raising an error
		if nodeEmpty(node) {
			return nil, errors.New("no data or links in this node")
		}

		dagnode, err = deserializeNode(node, options.DataType)
		if err != nil {
			return nil, err
		}

	default:
		return nil, errors.New("unknown object encoding")
	}

	if err != nil {
		return nil, err
	}

	err = api.node.DAG.Add(ctx, dagnode)
	if err != nil {
		return nil, err
	}

	return ParseCid(dagnode.Cid()), nil
}

func (api *ObjectAPI) Get(ctx context.Context, path coreiface.Path) (ipld.Node, error) {
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

func (api *ObjectAPI) Links(ctx context.Context, path coreiface.Path) ([]*ipld.Link, error) {
	nd, err := api.core().ResolveNode(ctx, path)
	if err != nil {
		return nil, err
	}

	links := nd.Links()
	out := make([]*ipld.Link, len(links))
	for n, l := range links {
		out[n] = (*ipld.Link)(l)
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

func (api *ObjectAPI) AddLink(ctx context.Context, base coreiface.Path, name string, child coreiface.Path, opts ...caopts.ObjectAddLinkOption) (coreiface.Path, error) {
	options, err := caopts.ObjectAddLinkOptions(opts...)
	if err != nil {
		return nil, err
	}

	baseNd, err := api.core().ResolveNode(ctx, base)
	if err != nil {
		return nil, err
	}

	childNd, err := api.core().ResolveNode(ctx, child)
	if err != nil {
		return nil, err
	}

	basePb, ok := baseNd.(*dag.ProtoNode)
	if !ok {
		return nil, dag.ErrNotProtobuf
	}

	var createfunc func() *dag.ProtoNode
	if options.Create {
		createfunc = ft.EmptyDirNode
	}

	e := dagutils.NewDagEditor(basePb, api.node.DAG)

	err = e.InsertNodeAtPath(ctx, name, childNd, createfunc)
	if err != nil {
		return nil, err
	}

	nnode, err := e.Finalize(ctx, api.node.DAG)
	if err != nil {
		return nil, err
	}

	return ParseCid(nnode.Cid()), nil
}

func (api *ObjectAPI) RmLink(ctx context.Context, base coreiface.Path, link string) (coreiface.Path, error) {
	baseNd, err := api.core().ResolveNode(ctx, base)
	if err != nil {
		return nil, err
	}

	basePb, ok := baseNd.(*dag.ProtoNode)
	if !ok {
		return nil, dag.ErrNotProtobuf
	}

	e := dagutils.NewDagEditor(basePb, api.node.DAG)

	err = e.RmLink(ctx, link)
	if err != nil {
		return nil, err
	}

	nnode, err := e.Finalize(ctx, api.node.DAG)
	if err != nil {
		return nil, err
	}

	return ParseCid(nnode.Cid()), nil
}

func (api *ObjectAPI) AppendData(ctx context.Context, path coreiface.Path, r io.Reader) (coreiface.Path, error) {
	return api.patchData(ctx, path, r, true)
}

func (api *ObjectAPI) SetData(ctx context.Context, path coreiface.Path, r io.Reader) (coreiface.Path, error) {
	return api.patchData(ctx, path, r, false)
}

func (api *ObjectAPI) patchData(ctx context.Context, path coreiface.Path, r io.Reader, appendData bool) (coreiface.Path, error) {
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

	err = api.node.DAG.Add(ctx, pbnd)
	if err != nil {
		return nil, err
	}

	return ParseCid(pbnd.Cid()), nil
}

func (api *ObjectAPI) core() coreiface.CoreAPI {
	return (*CoreAPI)(api)
}

func deserializeNode(nd *Node, dataFieldEncoding string) (*dag.ProtoNode, error) {
	dagnode := new(dag.ProtoNode)
	switch dataFieldEncoding {
	case "text":
		dagnode.SetData([]byte(nd.Data))
	case "base64":
		data, err := base64.StdEncoding.DecodeString(nd.Data)
		if err != nil {
			return nil, err
		}
		dagnode.SetData(data)
	default:
		return nil, fmt.Errorf("unkown data field encoding")
	}

	links := make([]*ipld.Link, len(nd.Links))
	for i, link := range nd.Links {
		c, err := cid.Decode(link.Hash)
		if err != nil {
			return nil, err
		}
		links[i] = &ipld.Link{
			Name: link.Name,
			Size: link.Size,
			Cid:  c,
		}
	}
	dagnode.SetLinks(links)

	return dagnode, nil
}

func nodeEmpty(node *Node) bool {
	return node.Data == "" && len(node.Links) == 0
}
