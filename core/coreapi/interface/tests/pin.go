package tests

import (
	"context"
	"strings"
	"testing"

	opt "github.com/ipfs/go-ipfs/core/coreapi/interface/options"
)

func (tp *provider) TestPin(t *testing.T) {
	t.Run("TestPinAdd", tp.TestPinAdd)
	t.Run("TestPinSimple", tp.TestPinSimple)
	t.Run("TestPinRecursive", tp.TestPinRecursive)
}

func (tp *provider) TestPinAdd(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	p, err := api.Unixfs().Add(ctx, strFile("foo")())
	if err != nil {
		t.Error(err)
	}

	err = api.Pin().Add(ctx, p)
	if err != nil {
		t.Error(err)
	}
}

func (tp *provider) TestPinSimple(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	p, err := api.Unixfs().Add(ctx, strFile("foo")())
	if err != nil {
		t.Error(err)
	}

	err = api.Pin().Add(ctx, p)
	if err != nil {
		t.Error(err)
	}

	list, err := api.Pin().Ls(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 1 {
		t.Errorf("unexpected pin list len: %d", len(list))
	}

	if list[0].Path().Cid().String() != p.Cid().String() {
		t.Error("paths don't match")
	}

	if list[0].Type() != "recursive" {
		t.Error("unexpected pin type")
	}

	err = api.Pin().Rm(ctx, p)
	if err != nil {
		t.Fatal(err)
	}

	list, err = api.Pin().Ls(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 0 {
		t.Errorf("unexpected pin list len: %d", len(list))
	}
}

func (tp *provider) TestPinRecursive(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	p0, err := api.Unixfs().Add(ctx, strFile("foo")())
	if err != nil {
		t.Error(err)
	}

	p1, err := api.Unixfs().Add(ctx, strFile("bar")())
	if err != nil {
		t.Error(err)
	}

	p2, err := api.Dag().Put(ctx, strings.NewReader(`{"lnk": {"/": "`+p0.Cid().String()+`"}}`))
	if err != nil {
		t.Error(err)
	}

	p3, err := api.Dag().Put(ctx, strings.NewReader(`{"lnk": {"/": "`+p1.Cid().String()+`"}}`))
	if err != nil {
		t.Error(err)
	}

	err = api.Pin().Add(ctx, p2)
	if err != nil {
		t.Error(err)
	}

	err = api.Pin().Add(ctx, p3, opt.Pin.Recursive(false))
	if err != nil {
		t.Error(err)
	}

	list, err := api.Pin().Ls(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 3 {
		t.Errorf("unexpected pin list len: %d", len(list))
	}

	list, err = api.Pin().Ls(ctx, opt.Pin.Type.Direct())
	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 1 {
		t.Errorf("unexpected pin list len: %d", len(list))
	}

	if list[0].Path().String() != p3.String() {
		t.Error("unexpected path")
	}

	list, err = api.Pin().Ls(ctx, opt.Pin.Type.Recursive())
	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 1 {
		t.Errorf("unexpected pin list len: %d", len(list))
	}

	if list[0].Path().String() != p2.String() {
		t.Error("unexpected path")
	}

	list, err = api.Pin().Ls(ctx, opt.Pin.Type.Indirect())
	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 1 {
		t.Errorf("unexpected pin list len: %d", len(list))
	}

	if list[0].Path().Cid().String() != p0.Cid().String() {
		t.Error("unexpected path")
	}

	res, err := api.Pin().Verify(ctx)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for r := range res {
		if !r.Ok() {
			t.Error("expected pin to be ok")
		}
		n++
	}

	if n != 1 {
		t.Errorf("unexpected verify result count: %d", n)
	}

	//TODO: figure out a way to test verify without touching IpfsNode
	/*
		err = api.Block().Rm(ctx, p0, opt.Block.Force(true))
		if err != nil {
			t.Fatal(err)
		}

		res, err = api.Pin().Verify(ctx)
		if err != nil {
			t.Fatal(err)
		}
		n = 0
		for r := range res {
			if r.Ok() {
				t.Error("expected pin to not be ok")
			}

			if len(r.BadNodes()) != 1 {
				t.Fatalf("unexpected badNodes len")
			}

			if r.BadNodes()[0].Path().Cid().String() != p0.Cid().String() {
				t.Error("unexpected badNode path")
			}

			if r.BadNodes()[0].Err().Error() != "merkledag: not found" {
				t.Errorf("unexpected badNode error: %s", r.BadNodes()[0].Err().Error())
			}
			n++
		}

		if n != 1 {
			t.Errorf("unexpected verify result count: %d", n)
		}
	*/
}
