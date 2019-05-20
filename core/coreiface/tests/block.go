package tests

import (
	"context"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"io/ioutil"
	"strings"
	"testing"

	coreiface "github.com/ipfs/interface-go-ipfs-core"
	opt "github.com/ipfs/interface-go-ipfs-core/options"

	mh "github.com/multiformats/go-multihash"
)

func (tp *TestSuite) TestBlock(t *testing.T) {
	tp.hasApi(t, func(api coreiface.CoreAPI) error {
		if api.Block() == nil {
			return apiNotImplemented
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

	res, err := api.Block().Put(ctx, strings.NewReader(`Hello`))
	if err != nil {
		t.Fatal(err)
	}

	if res.Path().Cid().String() != "QmPyo15ynbVrSTVdJL9th7JysHaAbXt9dM9tXk1bMHbRtk" {
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

	res, err := api.Block().Put(ctx, strings.NewReader(`Hello`), opt.Block.Format("cbor"))
	if err != nil {
		t.Fatal(err)
	}

	if res.Path().Cid().String() != "bafyreiayl6g3gitr7ys7kyng7sjywlrgimdoymco3jiyab6rozecmoazne" {
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

	res, err := api.Block().Put(ctx, strings.NewReader(`Hello`), opt.Block.Hash(mh.KECCAK_512, -1))
	if err != nil {
		t.Fatal(err)
	}

	if res.Path().Cid().String() != "bafyb2qgdh7w6dcq24u65xbtdoehyavegnpvxcqce7ttvs6ielgmwdfxrahmu37d33atik57x5y6s7d7qz32aasuwgirh3ocn6ywswqdifvu6e" {
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

	res, err := api.Block().Put(ctx, strings.NewReader(`Hello`), opt.Block.Hash(mh.KECCAK_512, -1))
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

	res, err := api.Block().Put(ctx, strings.NewReader(`Hello`))
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
		t.Error("expected err to exist")
	}
	if !strings.Contains(err.Error(), "blockservice: key not found") {
		t.Errorf("unexpected error; %s", err.Error())
	}

	err = api.Block().Rm(ctx, res.Path())
	if err == nil {
		t.Error("expected err to exist")
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

	res, err := api.Block().Put(ctx, strings.NewReader(`Hello`))
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

	_, err = api.Block().Put(ctx, strings.NewReader(`Hello`))
	if err != nil {
		t.Fatal(err)
	}

	if pins, err := api.Pin().Ls(ctx); err != nil || len(pins) != 0 {
		t.Fatal("expected 0 pins")
	}

	res, err := api.Block().Put(ctx, strings.NewReader(`Hello`), opt.Block.Pin(true))
	if err != nil {
		t.Fatal(err)
	}

	pins, err := api.Pin().Ls(ctx)
	if err != nil {
		return
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
