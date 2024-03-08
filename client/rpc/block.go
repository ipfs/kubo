package rpc

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/ipfs/boxo/path"
	"github.com/ipfs/go-cid"
	iface "github.com/ipfs/kubo/core/coreiface"
	caopts "github.com/ipfs/kubo/core/coreiface/options"
	mc "github.com/multiformats/go-multicodec"
	mh "github.com/multiformats/go-multihash"
)

type BlockAPI HttpApi

type blockStat struct {
	Key   string
	BSize int `json:"Size"`

	cid cid.Cid
}

func (s *blockStat) Size() int {
	return s.BSize
}

func (s *blockStat) Path() path.ImmutablePath {
	return path.FromCid(s.cid)
}

func (api *BlockAPI) Put(ctx context.Context, r io.Reader, opts ...caopts.BlockPutOption) (iface.BlockStat, error) {
	options, err := caopts.BlockPutOptions(opts...)
	px := options.CidPrefix
	if err != nil {
		return nil, err
	}

	mht, ok := mh.Codes[px.MhType]
	if !ok {
		return nil, fmt.Errorf("unknowm mhType %d", px.MhType)
	}

	var cidOptKey, cidOptVal string
	switch {
	case px.Version == 0 && px.Codec == cid.DagProtobuf:
		// ensure legacy --format=v0 passes as BlockPutOption still works
		cidOptKey = "format"
		cidOptVal = "v0"
	default:
		// pass codec as string
		cidOptKey = "cid-codec"
		cidOptVal = mc.Code(px.Codec).String()
	}

	req := api.core().Request("block/put").
		Option("mhtype", mht).
		Option("mhlen", px.MhLength).
		Option(cidOptKey, cidOptVal).
		Option("pin", options.Pin).
		FileBody(r)

	var out blockStat
	if err := req.Exec(ctx, &out); err != nil {
		return nil, err
	}
	out.cid, err = cid.Parse(out.Key)
	if err != nil {
		return nil, err
	}

	return &out, nil
}

func (api *BlockAPI) Get(ctx context.Context, p path.Path) (io.Reader, error) {
	resp, err := api.core().Request("block/get", p.String()).Send(ctx)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, parseErrNotFoundWithFallbackToError(resp.Error)
	}

	// TODO: make get return ReadCloser to avoid copying
	defer resp.Close()
	b := new(bytes.Buffer)
	if _, err := io.Copy(b, resp.Output); err != nil {
		return nil, err
	}

	return b, nil
}

func (api *BlockAPI) Rm(ctx context.Context, p path.Path, opts ...caopts.BlockRmOption) error {
	options, err := caopts.BlockRmOptions(opts...)
	if err != nil {
		return err
	}

	removedBlock := struct {
		Hash  string `json:",omitempty"`
		Error string `json:",omitempty"`
	}{}

	req := api.core().Request("block/rm").
		Option("force", options.Force).
		Arguments(p.String())

	if err := req.Exec(ctx, &removedBlock); err != nil {
		return err
	}

	return parseErrNotFoundWithFallbackToMSG(removedBlock.Error)
}

func (api *BlockAPI) Stat(ctx context.Context, p path.Path) (iface.BlockStat, error) {
	var out blockStat
	err := api.core().Request("block/stat", p.String()).Exec(ctx, &out)
	if err != nil {
		return nil, parseErrNotFoundWithFallbackToError(err)
	}
	out.cid, err = cid.Parse(out.Key)
	if err != nil {
		return nil, err
	}

	return &out, nil
}

func (api *BlockAPI) core() *HttpApi {
	return (*HttpApi)(api)
}
