package rpc

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/ipfs/boxo/path"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/kubo/core/coreiface/options"
	multicodec "github.com/multiformats/go-multicodec"
)

type (
	httpNodeAdder        HttpApi
	HttpDagServ          httpNodeAdder
	pinningHttpNodeAdder httpNodeAdder
)

func (api *HttpDagServ) Get(ctx context.Context, c cid.Cid) (format.Node, error) {
	r, err := api.core().Block().Get(ctx, path.FromCid(c))
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	blk, err := blocks.NewBlockWithCid(data, c)
	if err != nil {
		return nil, err
	}

	return api.ipldDecoder.DecodeNode(ctx, blk)
}

func (api *HttpDagServ) GetMany(ctx context.Context, cids []cid.Cid) <-chan *format.NodeOption {
	out := make(chan *format.NodeOption)

	for _, c := range cids {
		// TODO: Consider limiting concurrency of this somehow
		go func(c cid.Cid) {
			n, err := api.Get(ctx, c)

			select {
			case out <- &format.NodeOption{Node: n, Err: err}:
			case <-ctx.Done():
			}
		}(c)
	}
	return out
}

func (api *httpNodeAdder) add(ctx context.Context, nd format.Node, pin bool) error {
	c := nd.Cid()
	prefix := c.Prefix()

	// preserve 'cid-codec' when sent over HTTP
	cidCodec := multicodec.Code(prefix.Codec).String()

	// 'format' got replaced by 'cid-codec' in https://github.com/ipfs/interface-go-ipfs-core/pull/80
	// but we still support it here for backward-compatibility with use of CIDv0
	format := ""
	if prefix.Version == 0 {
		cidCodec = ""
		format = "v0"
	}

	stat, err := api.core().Block().Put(ctx, bytes.NewReader(nd.RawData()),
		options.Block.Hash(prefix.MhType, prefix.MhLength),
		options.Block.CidCodec(cidCodec),
		options.Block.Format(format),
		options.Block.Pin(pin))
	if err != nil {
		return err
	}
	if !stat.Path().RootCid().Equals(c) {
		return fmt.Errorf("cids didn't match - local %s, remote %s", c.String(), stat.Path().RootCid().String())
	}
	return nil
}

func (api *httpNodeAdder) addMany(ctx context.Context, nds []format.Node, pin bool) error {
	for _, nd := range nds {
		// TODO: optimize
		if err := api.add(ctx, nd, pin); err != nil {
			return err
		}
	}
	return nil
}

func (api *HttpDagServ) AddMany(ctx context.Context, nds []format.Node) error {
	return (*httpNodeAdder)(api).addMany(ctx, nds, false)
}

func (api *HttpDagServ) Add(ctx context.Context, nd format.Node) error {
	return (*httpNodeAdder)(api).add(ctx, nd, false)
}

func (api *pinningHttpNodeAdder) Add(ctx context.Context, nd format.Node) error {
	return (*httpNodeAdder)(api).add(ctx, nd, true)
}

func (api *pinningHttpNodeAdder) AddMany(ctx context.Context, nds []format.Node) error {
	return (*httpNodeAdder)(api).addMany(ctx, nds, true)
}

func (api *HttpDagServ) Pinning() format.NodeAdder {
	return (*pinningHttpNodeAdder)(api)
}

func (api *HttpDagServ) Remove(ctx context.Context, c cid.Cid) error {
	return api.core().Block().Rm(ctx, path.FromCid(c)) // TODO: should we force rm?
}

func (api *HttpDagServ) RemoveMany(ctx context.Context, cids []cid.Cid) error {
	for _, c := range cids {
		// TODO: optimize
		if err := api.Remove(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

func (api *httpNodeAdder) core() *HttpApi {
	return (*HttpApi)(api)
}

func (api *HttpDagServ) core() *HttpApi {
	return (*HttpApi)(api)
}
