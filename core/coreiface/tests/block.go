package tests

import (
	"context"
	"io/ioutil"
	"strings"
	"testing"

	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	opt "github.com/ipfs/go-ipfs/core/coreapi/interface/options"

	mh "gx/ipfs/QmerPMzPk1mJVowm8KgmoknWa4yCYvvugMPsgWmDNUvDLW/go-multihash"
)

func TestBlock(t *testing.T) {
	t.Run("TestBlockPut", TestBlockPut)
	t.Run("TestBlockPutFormat", TestBlockPutFormat)
	t.Run("TestBlockPutHash", TestBlockPutHash)
	t.Run("TestBlockGet", TestBlockGet)
	t.Run("TestBlockRm", TestBlockRm)
	t.Run("TestBlockStat", TestBlockStat)
}

func TestBlockPut(t *testing.T) {
	ctx := context.Background()
	api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	res, err := api.Block().Put(ctx, strings.NewReader(`Hello`))
	if err != nil {
		t.Error(err)
	}

	if res.Path().Cid().String() != "QmPyo15ynbVrSTVdJL9th7JysHaAbXt9dM9tXk1bMHbRtk" {
		t.Errorf("got wrong cid: %s", res.Path().Cid().String())
	}
}

func TestBlockPutFormat(t *testing.T) {
	ctx := context.Background()
	api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	res, err := api.Block().Put(ctx, strings.NewReader(`Hello`), opt.Block.Format("cbor"))
	if err != nil {
		t.Error(err)
	}

	if res.Path().Cid().String() != "zdpuAn4amuLWo8Widi5v6VQpuo2dnpnwbVE3oB6qqs7mDSeoa" {
		t.Errorf("got wrong cid: %s", res.Path().Cid().String())
	}
}

func TestBlockPutHash(t *testing.T) {
	ctx := context.Background()
	api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	res, err := api.Block().Put(ctx, strings.NewReader(`Hello`), opt.Block.Hash(mh.KECCAK_512, -1))
	if err != nil {
		t.Fatal(err)
	}

	if res.Path().Cid().String() != "zBurKB9YZkcDf6xa53WBE8CFX4ydVqAyf9KPXBFZt5stJzEstaS8Hukkhu4gwpMtc1xHNDbzP7sPtQKyWsP3C8fbhkmrZ" {
		t.Errorf("got wrong cid: %s", res.Path().Cid().String())
	}
}

func TestBlockGet(t *testing.T) {
	ctx := context.Background()
	api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	res, err := api.Block().Put(ctx, strings.NewReader(`Hello`), opt.Block.Hash(mh.KECCAK_512, -1))
	if err != nil {
		t.Error(err)
	}

	r, err := api.Block().Get(ctx, res.Path())
	if err != nil {
		t.Error(err)
	}

	d, err := ioutil.ReadAll(r)
	if err != nil {
		t.Error(err)
	}

	if string(d) != "Hello" {
		t.Error("didn't get correct data back")
	}

	p, err := coreiface.ParsePath("/ipfs/" + res.Path().Cid().String())
	if err != nil {
		t.Error(err)
	}

	rp, err := api.ResolvePath(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	if rp.Cid().String() != res.Path().Cid().String() {
		t.Error("paths didn't match")
	}
}

func TestBlockRm(t *testing.T) {
	ctx := context.Background()
	api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	res, err := api.Block().Put(ctx, strings.NewReader(`Hello`))
	if err != nil {
		t.Error(err)
	}

	r, err := api.Block().Get(ctx, res.Path())
	if err != nil {
		t.Error(err)
	}

	d, err := ioutil.ReadAll(r)
	if err != nil {
		t.Error(err)
	}

	if string(d) != "Hello" {
		t.Error("didn't get correct data back")
	}

	err = api.Block().Rm(ctx, res.Path())
	if err != nil {
		t.Error(err)
	}

	_, err = api.Block().Get(ctx, res.Path())
	if err == nil {
		t.Error("expected err to exist")
	}
	if err.Error() != "blockservice: key not found" {
		t.Errorf("unexpected error; %s", err.Error())
	}

	err = api.Block().Rm(ctx, res.Path())
	if err == nil {
		t.Error("expected err to exist")
	}
	if err.Error() != "blockstore: block not found" {
		t.Errorf("unexpected error; %s", err.Error())
	}

	err = api.Block().Rm(ctx, res.Path(), opt.Block.Force(true))
	if err != nil {
		t.Error(err)
	}
}

func TestBlockStat(t *testing.T) {
	ctx := context.Background()
	api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	res, err := api.Block().Put(ctx, strings.NewReader(`Hello`))
	if err != nil {
		t.Error(err)
	}

	stat, err := api.Block().Stat(ctx, res.Path())
	if err != nil {
		t.Error(err)
	}

	if stat.Path().String() != res.Path().String() {
		t.Error("paths don't match")
	}

	if stat.Size() != len("Hello") {
		t.Error("length doesn't match")
	}
}
