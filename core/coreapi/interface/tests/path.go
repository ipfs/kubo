package tests

import (
	"context"
	"strings"
	"testing"

	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	"github.com/ipfs/go-ipfs/core/coreapi/interface/options"
)

func (tp *provider) TestPath(t *testing.T) {
	t.Run("TestMutablePath", tp.TestMutablePath)
	t.Run("TestPathRemainder", tp.TestPathRemainder)
	t.Run("TestEmptyPathRemainder", tp.TestEmptyPathRemainder)
	t.Run("TestInvalidPathRemainder", tp.TestInvalidPathRemainder)
	t.Run("TestPathRoot", tp.TestPathRoot)
}

func (tp *provider) TestMutablePath(t *testing.T) {
	ctx := context.Background()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// get self /ipns path
	keys, err := api.Key().List(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if !keys[0].Path().Mutable() {
		t.Error("expected self /ipns path to be mutable")
	}

	blk, err := api.Block().Put(ctx, strings.NewReader(`foo`))
	if err != nil {
		t.Error(err)
	}

	if blk.Path().Mutable() {
		t.Error("expected /ipld path to be immutable")
	}
}

func (tp *provider) TestPathRemainder(t *testing.T) {
	ctx := context.Background()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	obj, err := api.Dag().Put(ctx, strings.NewReader(`{"foo": {"bar": "baz"}}`))
	if err != nil {
		t.Fatal(err)
	}

	p1, err := coreiface.ParsePath(obj.String() + "/foo/bar")
	if err != nil {
		t.Error(err)
	}

	rp1, err := api.ResolvePath(ctx, p1)
	if err != nil {
		t.Fatal(err)
	}

	if rp1.Remainder() != "foo/bar" {
		t.Error("expected to get path remainder")
	}
}

func (tp *provider) TestEmptyPathRemainder(t *testing.T) {
	ctx := context.Background()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	obj, err := api.Dag().Put(ctx, strings.NewReader(`{"foo": {"bar": "baz"}}`))
	if err != nil {
		t.Fatal(err)
	}

	if obj.Remainder() != "" {
		t.Error("expected the resolved path to not have a remainder")
	}

	p1, err := coreiface.ParsePath(obj.String())
	if err != nil {
		t.Error(err)
	}

	rp1, err := api.ResolvePath(ctx, p1)
	if err != nil {
		t.Fatal(err)
	}

	if rp1.Remainder() != "" {
		t.Error("expected the resolved path to not have a remainder")
	}
}

func (tp *provider) TestInvalidPathRemainder(t *testing.T) {
	ctx := context.Background()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	obj, err := api.Dag().Put(ctx, strings.NewReader(`{"foo": {"bar": "baz"}}`))
	if err != nil {
		t.Fatal(err)
	}

	p1, err := coreiface.ParsePath(obj.String() + "/bar/baz")
	if err != nil {
		t.Error(err)
	}

	_, err = api.ResolvePath(ctx, p1)
	if err == nil || err.Error() != "no such link found" {
		t.Fatalf("unexpected error: %s", err)
	}
}

func (tp *provider) TestPathRoot(t *testing.T) {
	ctx := context.Background()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	blk, err := api.Block().Put(ctx, strings.NewReader(`foo`), options.Block.Format("raw"))
	if err != nil {
		t.Error(err)
	}

	obj, err := api.Dag().Put(ctx, strings.NewReader(`{"foo": {"/": "`+blk.Path().Cid().String()+`"}}`))
	if err != nil {
		t.Fatal(err)
	}

	p1, err := coreiface.ParsePath(obj.String() + "/foo")
	if err != nil {
		t.Error(err)
	}

	rp, err := api.ResolvePath(ctx, p1)
	if err != nil {
		t.Fatal(err)
	}

	if rp.Root().String() != obj.Cid().String() {
		t.Error("unexpected path root")
	}

	if rp.Cid().String() != blk.Path().Cid().String() {
		t.Error("unexpected path cid")
	}
}
