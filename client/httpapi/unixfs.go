package httpapi

import (
	"context"
	"fmt"
	"github.com/ipfs/go-cid"

	"github.com/ipfs/go-ipfs/core/coreapi/interface"
	caopts "github.com/ipfs/go-ipfs/core/coreapi/interface/options"

	"github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/go-ipld-format"
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
		//Option("", options.Events).
		Option("fscache", options.FsCache).
		Option("hidden", options.Hidden).
		Option("inline", options.Inline).
		Option("inline-limit", options.InlineLimit).
		Option("nocopy", options.NoCopy).
		Option("only-hash", options.OnlyHash).
		Option("pin", options.Pin).
		//Option("", options.Progress).
		Option("silent", options.Silent).
		Option("stdin-name", options.StdinName).
		Option("wrap-with-directory", options.Wrap).
		Option("quieter", true) // TODO: rm after event impl

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
	if err := req.Exec(ctx, &out); err != nil { //TODO: ndjson events
		return nil, err
	}

	c, err := cid.Parse(out.Hash)
	if err != nil {
		return nil, err
	}

	return iface.IpfsPath(c), nil
}

func (api *UnixfsAPI) Get(context.Context, iface.Path) (files.Node, error) {
	panic("implement me")
}

func (api *UnixfsAPI) Ls(context.Context, iface.Path) ([]*format.Link, error) {
	panic("implement me")
}

func (api *UnixfsAPI) core() *HttpApi {
	return (*HttpApi)(api)
}
