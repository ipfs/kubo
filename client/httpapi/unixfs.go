package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/go-ipld-format"
	"github.com/ipfs/interface-go-ipfs-core"
	caopts "github.com/ipfs/interface-go-ipfs-core/options"
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
	Type       iface.FileType
}

type lsObject struct {
	Hash  string
	Links []lsLink
}

type lsOutput struct {
	Objects []lsObject
}

func (api *UnixfsAPI) Ls(ctx context.Context, p iface.Path, opts ...caopts.UnixfsLsOption) (<-chan iface.LsLink, error) {
	options, err := caopts.UnixfsLsOptions(opts...)
	if err != nil {
		return nil, err
	}

	resp, err := api.core().request("ls", p.String()).
		Option("resolve-type", options.ResolveChildren).
		Option("size", options.ResolveChildren).
		Option("stream", true).
		Send(ctx)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}

	dec := json.NewDecoder(resp.Output)
	out := make(chan iface.LsLink)

	go func() {
		defer resp.Close()
		defer close(out)

		for {
			var link lsOutput
			if err := dec.Decode(&link); err != nil {
				if err == io.EOF {
					return
				}
				select {
				case out <- iface.LsLink{Err: err}:
				case <-ctx.Done():
				}
				return
			}

			if len(link.Objects) != 1 {
				select {
				case out <- iface.LsLink{Err: errors.New("unexpected Objects len")}:
				case <-ctx.Done():
				}
				return
			}

			if len(link.Objects[0].Links) != 1 {
				select {
				case out <- iface.LsLink{Err: errors.New("unexpected Links len")}:
				case <-ctx.Done():
				}
				return
			}

			l0 := link.Objects[0].Links[0]

			c, err := cid.Decode(l0.Hash)
			if err != nil {
				select {
				case out <- iface.LsLink{Err: err}:
				case <-ctx.Done():
				}
				return
			}

			select {
			case out <- iface.LsLink{
				Link: &format.Link{
					Cid:  c,
					Name: l0.Name,
					Size: l0.Size,
				},
				Size: l0.Size,
				Type: l0.Type,
			}:
			case <-ctx.Done():
			}
		}
	}()

	return out, nil
}

func (api *UnixfsAPI) core() *HttpApi {
	return (*HttpApi)(api)
}
