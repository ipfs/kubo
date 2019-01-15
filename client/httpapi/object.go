package httpapi

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"

	"github.com/ipfs/go-ipfs/core/coreapi/interface"
	caopts "github.com/ipfs/go-ipfs/core/coreapi/interface/options"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"
)

type ObjectAPI HttpApi

type objectOut struct {
	Hash string
}

func (api *ObjectAPI) New(ctx context.Context, opts ...caopts.ObjectNewOption) (format.Node, error) {
	options, err := caopts.ObjectNewOptions(opts...)
	if err != nil {
		return nil, err
	}

	var out objectOut
	err = api.core().request("object/new", options.Type).Exec(ctx, &out)
	if err != nil {
		return nil, err
	}

	c, err := cid.Parse(out.Hash)
	if err != nil {
		return nil, err
	}

	return api.core().nodeFromPath(ctx, iface.IpfsPath(c)), nil
}

func (api *ObjectAPI) Put(ctx context.Context, r io.Reader, opts ...caopts.ObjectPutOption) (iface.ResolvedPath, error) {
	options, err := caopts.ObjectPutOptions(opts...)
	if err != nil {
		return nil, err
	}

	var out objectOut
	err = api.core().request("object/put").
		Option("inputenc", options.InputEnc).
		Option("datafieldenc", options.DataType).
		Option("pin", options.Pin).
		FileBody(r).
		Exec(ctx, &out)
	if err != nil {
		return nil, err
	}

	c, err := cid.Parse(out.Hash)
	if err != nil {
		return nil, err
	}

	return iface.IpfsPath(c), nil
}

func (api *ObjectAPI) Get(ctx context.Context, p iface.Path) (format.Node, error) {
	r, err := api.core().Block().Get(ctx, p)
	if err != nil {
		return nil, err
	}
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return merkledag.DecodeProtobuf(b)
}

func (api *ObjectAPI) Data(ctx context.Context, p iface.Path) (io.Reader, error) {
	resp, err := api.core().request("object/data", p.String()).Send(ctx)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}

	//TODO: make Data return ReadCloser to avoid copying
	defer resp.Close()
	b := new(bytes.Buffer)
	if _, err := io.Copy(b, resp.Output); err != nil {
		return nil, err
	}

	return b, nil
}

func (api *ObjectAPI) Links(ctx context.Context, p iface.Path) ([]*format.Link, error) {
	var out struct {
		Links []struct {
			Name string
			Hash string
			Size uint64
		}
	}
	if err := api.core().request("object/links", p.String()).Exec(ctx, &out); err != nil {
		return nil, err
	}
	res := make([]*format.Link, len(out.Links))
	for i, l := range out.Links {
		c, err := cid.Parse(l.Hash)
		if err != nil {
			return nil, err
		}

		res[i] = &format.Link{
			Cid:  c,
			Name: l.Name,
			Size: l.Size,
		}
	}

	return res, nil
}

func (api *ObjectAPI) Stat(ctx context.Context, p iface.Path) (*iface.ObjectStat, error) {
	var out struct {
		Hash           string
		NumLinks       int
		BlockSize      int
		LinksSize      int
		DataSize       int
		CumulativeSize int
	}
	if err := api.core().request("object/stat", p.String()).Exec(ctx, &out); err != nil {
		return nil, err
	}

	c, err := cid.Parse(out.Hash)
	if err != nil {
		return nil, err
	}

	return &iface.ObjectStat{
		Cid:            c,
		NumLinks:       out.NumLinks,
		BlockSize:      out.BlockSize,
		LinksSize:      out.LinksSize,
		DataSize:       out.DataSize,
		CumulativeSize: out.CumulativeSize,
	}, nil
}

func (api *ObjectAPI) AddLink(ctx context.Context, base iface.Path, name string, child iface.Path, opts ...caopts.ObjectAddLinkOption) (iface.ResolvedPath, error) {
	options, err := caopts.ObjectAddLinkOptions(opts...)
	if err != nil {
		return nil, err
	}

	var out objectOut
	err = api.core().request("object/patch/add-link", base.String(), name, child.String()).
		Option("create", options.Create).
		Exec(ctx, &out)
	if err != nil {
		return nil, err
	}

	c, err := cid.Parse(out.Hash)
	if err != nil {
		return nil, err
	}

	return iface.IpfsPath(c), nil
}

func (api *ObjectAPI) RmLink(ctx context.Context, base iface.Path, link string) (iface.ResolvedPath, error) {
	var out objectOut
	err := api.core().request("object/patch/rm-link", base.String(), link).
		Exec(ctx, &out)
	if err != nil {
		return nil, err
	}

	c, err := cid.Parse(out.Hash)
	if err != nil {
		return nil, err
	}

	return iface.IpfsPath(c), nil
}

func (api *ObjectAPI) AppendData(ctx context.Context, p iface.Path, r io.Reader) (iface.ResolvedPath, error) {
	var out objectOut
	err := api.core().request("object/patch/append-data", p.String()).
		FileBody(r).
		Exec(ctx, &out)
	if err != nil {
		return nil, err
	}

	c, err := cid.Parse(out.Hash)
	if err != nil {
		return nil, err
	}

	return iface.IpfsPath(c), nil
}

func (api *ObjectAPI) SetData(ctx context.Context, p iface.Path, r io.Reader) (iface.ResolvedPath, error) {
	var out objectOut
	err := api.core().request("object/patch/set-data", p.String()).
		FileBody(r).
		Exec(ctx, &out)
	if err != nil {
		return nil, err
	}

	c, err := cid.Parse(out.Hash)
	if err != nil {
		return nil, err
	}

	return iface.IpfsPath(c), nil
}

type change struct {
	Type   iface.ChangeType
	Path   string
	Before cid.Cid
	After  cid.Cid
}

func (api *ObjectAPI) Diff(ctx context.Context, a iface.Path, b iface.Path) ([]iface.ObjectChange, error) {
	var out struct {
		Changes []change
	}
	if err := api.core().request("object/diff", a.String(), b.String()).Exec(ctx, &out); err != nil {
		return nil, err
	}
	res := make([]iface.ObjectChange, len(out.Changes))
	for i, ch := range out.Changes {
		res[i] = iface.ObjectChange{
			Type: ch.Type,
			Path: ch.Path,
		}
		if ch.Before != cid.Undef {
			res[i].Before = iface.IpfsPath(ch.Before)
		}
		if ch.After != cid.Undef {
			res[i].After = iface.IpfsPath(ch.After)
		}
	}
	return res, nil
}

func (api *ObjectAPI) core() *HttpApi {
	return (*HttpApi)(api)
}
