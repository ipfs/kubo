package tests

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"strings"
	"testing"

	coreiface "github.com/ipfs/interface-go-ipfs-core"
	opt "github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/ipfs/interface-go-ipfs-core/path"

	mh "github.com/multiformats/go-multihash"
)

var (
	pbCid    = "QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN"
	cborCid  = "bafyreicnga62zhxnmnlt6ymq5hcbsg7gdhqdu6z4ehu3wpjhvqnflfy6nm"
	cborKCid = "bafyr2qgsohbwdlk7ajmmbb4lhoytmest4wdbe5xnexfvtxeatuyqqmwv3fgxp3pmhpc27gwey2cct56gloqefoqwcf3yqiqzsaqb7p4jefhcw"
)

func pbBlock() io.Reader {
	return bytes.NewReader([]byte{10, 12, 8, 2, 18, 6, 104, 101, 108, 108, 111, 10, 24, 6})
}

func cborBlock() io.Reader {
	return bytes.NewReader([]byte{101, 72, 101, 108, 108, 111})
}

func (tp *TestSuite) TestBlock(t *testing.T) {
	tp.hasApi(t, func(api coreiface.CoreAPI) error {
		if api.Block() == nil {
			return errAPINotImplemented
		}
		return nil
	})

	t.Run("TestBlockPut", tp.TestBlockPut)
	t.Run("TestBlockPutFormat", tp.TestBlockPutFormat)
	t.Run("TestBlockPutHash", tp.TestBlockPutHash)
	t.Run("TestBlockGet", tp.TestBlockGet)
	t.Run("TestBlockRm", tp.TestBlockRm)
	t.Run("TestBlockStat", tp.TestBlockStat)
	t.Run("TestBlockPin", tp.TestBlockPin)
}

func (tp *TestSuite) TestBlockPut(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	res, err := api.Block().Put(ctx, pbBlock())
	if err != nil {
		t.Fatal(err)
	}

	if res.Path().Cid().String() != pbCid {
		t.Errorf("got wrong cid: %s", res.Path().Cid().String())
	}
}

func (tp *TestSuite) TestBlockPutFormat(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	res, err := api.Block().Put(ctx, cborBlock(), opt.Block.Format("cbor"))
	if err != nil {
		t.Fatal(err)
	}

	if res.Path().Cid().String() != cborCid {
		t.Errorf("got wrong cid: %s", res.Path().Cid().String())
	}
}

func (tp *TestSuite) TestBlockPutHash(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	res, err := api.Block().Put(
		ctx,
		cborBlock(),
		opt.Block.Hash(mh.KECCAK_512, -1),
		opt.Block.Format("cbor"),
	)
	if err != nil {
		t.Fatal(err)
	}

	if res.Path().Cid().String() != cborKCid {
		t.Errorf("got wrong cid: %s", res.Path().Cid().String())
	}
}

func (tp *TestSuite) TestBlockGet(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	res, err := api.Block().Put(ctx, strings.NewReader(`Hello`), opt.Block.Format("raw"))
	if err != nil {
		t.Fatal(err)
	}

	r, err := api.Block().Get(ctx, res.Path())
	if err != nil {
		t.Fatal(err)
	}

	d, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	if string(d) != "Hello" {
		t.Error("didn't get correct data back")
	}

	p := path.New("/ipfs/" + res.Path().Cid().String())

	rp, err := api.ResolvePath(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	if rp.Cid().String() != res.Path().Cid().String() {
		t.Error("paths didn't match")
	}
}

func (tp *TestSuite) TestBlockRm(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	res, err := api.Block().Put(ctx, strings.NewReader(`Hello`), opt.Block.Format("raw"))
	if err != nil {
		t.Fatal(err)
	}

	r, err := api.Block().Get(ctx, res.Path())
	if err != nil {
		t.Fatal(err)
	}

	d, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	if string(d) != "Hello" {
		t.Error("didn't get correct data back")
	}

	err = api.Block().Rm(ctx, res.Path())
	if err != nil {
		t.Fatal(err)
	}

	_, err = api.Block().Get(ctx, res.Path())
	if err == nil {
		t.Fatal("expected err to exist")
	}
	if !strings.Contains(err.Error(), "blockservice: key not found") {
		t.Errorf("unexpected error; %s", err.Error())
	}

	err = api.Block().Rm(ctx, res.Path())
	if err == nil {
		t.Fatal("expected err to exist")
	}
	if !strings.Contains(err.Error(), "blockstore: block not found") {
		t.Errorf("unexpected error; %s", err.Error())
	}

	err = api.Block().Rm(ctx, res.Path(), opt.Block.Force(true))
	if err != nil {
		t.Fatal(err)
	}
}

func (tp *TestSuite) TestBlockStat(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	res, err := api.Block().Put(ctx, strings.NewReader(`Hello`), opt.Block.Format("raw"))
	if err != nil {
		t.Fatal(err)
	}

	stat, err := api.Block().Stat(ctx, res.Path())
	if err != nil {
		t.Fatal(err)
	}

	if stat.Path().String() != res.Path().String() {
		t.Error("paths don't match")
	}

	if stat.Size() != len("Hello") {
		t.Error("length doesn't match")
	}
}

func (tp *TestSuite) TestBlockPin(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	_, err = api.Block().Put(ctx, strings.NewReader(`Hello`), opt.Block.Format("raw"))
	if err != nil {
		t.Fatal(err)
	}

	if pins, err := api.Pin().Ls(ctx); err != nil || len(pins) != 0 {
		t.Fatal("expected 0 pins")
	}

	res, err := api.Block().Put(
		ctx,
		strings.NewReader(`Hello`),
		opt.Block.Pin(true),
		opt.Block.Format("raw"),
	)
	if err != nil {
		t.Fatal(err)
	}

	pins, err := accPins(api.Pin().Ls(ctx))
	if err != nil {
		t.Fatal(err)
	}
	if len(pins) != 1 {
		t.Fatal("expected 1 pin")
	}
	if pins[0].Type() != "recursive" {
		t.Error("expected a recursive pin")
	}
	if pins[0].Path().String() != res.Path().String() {
		t.Error("pin path didn't match")
	}
}
