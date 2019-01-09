package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"
	"io"

	"github.com/ipfs/go-ipfs/core/coreapi/interface"
	caopts "github.com/ipfs/go-ipfs/core/coreapi/interface/options"

	"github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/go-ipld-format"
	unixfspb "github.com/ipfs/go-unixfs/pb"
	mh "github.com/multiformats/go-multihash"
)

type addEvent struct {
	Name  string
	Hash  string `json:",omitempty"`
	Bytes int64  `json:",omitempty"`
	Size  string `json:",omitempty"`
}

type UnixfsAPI HttpApi

func (api *UnixfsAPI) Add(ctx context.Context, f files.Node, opts ...caopts.UnixfsAddOption) (iface.ResolvedPath, error) {
	options, _, err := caopts.UnixfsAddOptions(opts...)
	if err != nil {
		return nil, err
	}

	mht, ok := mh.Codes[options.MhType]
	if !ok {
		return nil, fmt.Errorf("unknowm mhType %d", options.MhType)
	}

	req := api.core().request("add").
		Option("hash", mht).
		Option("chunker", options.Chunker).
		Option("cid-version", options.CidVersion).
		Option("fscache", options.FsCache).
		Option("hidden", options.Hidden).
		Option("inline", options.Inline).
		Option("inline-limit", options.InlineLimit).
		Option("nocopy", options.NoCopy).
		Option("only-hash", options.OnlyHash).
		Option("pin", options.Pin).
		Option("silent", options.Silent).
		Option("stdin-name", options.StdinName).
		Option("wrap-with-directory", options.Wrap).
		Option("progress", options.Progress)

	if options.RawLeavesSet {
		req.Option("raw-leaves", options.RawLeaves)
	}

	switch options.Layout {
	case caopts.BalancedLayout:
		// noop, default
	case caopts.TrickleLayout:
		req.Option("trickle", true)
	}

	switch c := f.(type) {
	case files.Directory:
		req.Body(files.NewMultiFileReader(c, false))
	case files.File:
		d := files.NewMapDirectory(map[string]files.Node{"": c}) // unwrapped on the other side
		req.Body(files.NewMultiFileReader(d, false))
	}

	var out addEvent
	resp, err := req.Send(ctx)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}
	defer resp.Output.Close()
	dec := json.NewDecoder(resp.Output)
loop:
	for {
		var evt addEvent
		switch err := dec.Decode(&evt); err {
		case nil:
		case io.EOF:
			break loop
		default:
			return nil, err
		}
		out = evt

		if options.Events != nil {
			ifevt := &iface.AddEvent{
				Name:  out.Name,
				Size:  out.Size,
				Bytes: out.Bytes,
			}

			if out.Hash != "" {
				c, err := cid.Parse(out.Hash)
				if err != nil {
					return nil, err
				}

				ifevt.Path = iface.IpfsPath(c)
			}

			select {
			case options.Events <- ifevt:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	c, err := cid.Parse(out.Hash)
	if err != nil {
		return nil, err
	}

	return iface.IpfsPath(c), nil
}

type lsLink struct {
	Name, Hash string
	Size       uint64
	Type       unixfspb.Data_DataType
}

type lsObject struct {
	Hash  string
	Links []lsLink
}

type lsOutput struct {
	Objects []lsObject
}

func (api *UnixfsAPI) Ls(ctx context.Context, p iface.Path) ([]*format.Link, error) {
	var out lsOutput
	err := api.core().request("ls", p.String()).Exec(ctx, &out)
	if err != nil {
		return nil, err
	}

	if len(out.Objects) != 1 {
		return nil, errors.New("unexpected objects len")
	}

	links := make([]*format.Link, len(out.Objects[0].Links))
	for i, l := range out.Objects[0].Links {
		c, err := cid.Parse(l.Hash)
		if err != nil {
			return nil, err
		}
		links[i] = &format.Link{
			Name: l.Name,
			Size: l.Size,
			Cid:  c,
		}
	}
	return links, nil
}

func (api *UnixfsAPI) core() *HttpApi {
	return (*HttpApi)(api)
}
