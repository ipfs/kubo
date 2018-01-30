package coreapi_test

import (
	"context"
	"io/ioutil"
	"strings"
	"testing"

	mh "gx/ipfs/QmZyZDi491cCNTLfAhwcaDii2Kg4pwKRkhqQzURGDvY6ua/go-multihash"
)

func TestBlockPut(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	res, err := api.Block().Put(ctx, strings.NewReader(`Hello`))
	if err != nil {
		t.Error(err)
	}

	if res.Cid().String() != "QmPyo15ynbVrSTVdJL9th7JysHaAbXt9dM9tXk1bMHbRtk" {
		t.Errorf("got wrong cid: %s", res.Cid().String())
	}
}

func TestBlockPutFormat(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	res, err := api.Block().Put(ctx, strings.NewReader(`Hello`), api.Block().WithFormat("cbor"))
	if err != nil {
		t.Error(err)
	}

	if res.Cid().String() != "zdpuAn4amuLWo8Widi5v6VQpuo2dnpnwbVE3oB6qqs7mDSeoa" {
		t.Errorf("got wrong cid: %s", res.Cid().String())
	}
}

func TestBlockPutHash(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	res, err := api.Block().Put(ctx, strings.NewReader(`Hello`), api.Block().WithHash(mh.KECCAK_512, -1))
	if err != nil {
		t.Error(err)
	}

	if res.Cid().String() != "zBurKB9YZkcDf6xa53WBE8CFX4ydVqAyf9KPXBFZt5stJzEstaS8Hukkhu4gwpMtc1xHNDbzP7sPtQKyWsP3C8fbhkmrZ" {
		t.Errorf("got wrong cid: %s", res.Cid().String())
	}
}

func TestBlockGet(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	res, err := api.Block().Put(ctx, strings.NewReader(`Hello`), api.Block().WithHash(mh.KECCAK_512, -1))
	if err != nil {
		t.Error(err)
	}

	r, err := api.Block().Get(ctx, res)
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
}

func TestBlockRm(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	res, err := api.Block().Put(ctx, strings.NewReader(`Hello`))
	if err != nil {
		t.Error(err)
	}

	r, err := api.Block().Get(ctx, res)
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

	err = api.Block().Rm(ctx, res)
	if err != nil {
		t.Error(err)
	}

	_, err = api.Block().Get(ctx, res)
	if err == nil {
		t.Error("expected err to exist")
	}
	if err.Error() != "blockservice: key not found" {
		t.Errorf("unexpected error; %s", err.Error())
	}

	err = api.Block().Rm(ctx, res)
	if err == nil {
		t.Error("expected err to exist")
	}
	if err.Error() != "blockstore: block not found" {
		t.Errorf("unexpected error; %s", err.Error())
	}

	err = api.Block().Rm(ctx, res, api.Block().WithForce(true))
	if err != nil {
		t.Error(err)
	}
}

func TestBlockStat(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	res, err := api.Block().Put(ctx, strings.NewReader(`Hello`))
	if err != nil {
		t.Error(err)
	}

	stat, err := api.Block().Stat(ctx, res)
	if err != nil {
		t.Error(err)
	}

	if stat.Path().String() != res.String() {
		t.Error("paths don't match")
	}

	if stat.Size() != len("Hello") {
		t.Error("length doesn't match")
	}
}
