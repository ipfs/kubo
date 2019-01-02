package tests

import (
	"context"
	"path"
	"strings"
	"testing"

	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	opt "github.com/ipfs/go-ipfs/core/coreapi/interface/options"

	mh "gx/ipfs/QmerPMzPk1mJVowm8KgmoknWa4yCYvvugMPsgWmDNUvDLW/go-multihash"
)

func (tp *provider) TestDag(t *testing.T) {
	t.Run("TestPut", tp.TestPut)
	t.Run("TestPutWithHash", tp.TestPutWithHash)
	t.Run("TestPath", tp.TestDagPath)
	t.Run("TestTree", tp.TestTree)
	t.Run("TestBatch", tp.TestBatch)
}

var (
	treeExpected = map[string]struct{}{
		"a":   {},
		"b":   {},
		"c":   {},
		"c/d": {},
		"c/e": {},
	}
)

func (tp *provider) TestPut(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	res, err := api.Dag().Put(ctx, strings.NewReader(`"Hello"`))
	if err != nil {
		t.Fatal(err)
	}

	if res.Cid().String() != "zdpuAqckYF3ToF3gcJNxPZXmnmGuXd3gxHCXhq81HGxBejEvv" {
		t.Errorf("got wrong cid: %s", res.Cid().String())
	}
}

func (tp *provider) TestPutWithHash(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	res, err := api.Dag().Put(ctx, strings.NewReader(`"Hello"`), opt.Dag.Hash(mh.ID, -1))
	if err != nil {
		t.Fatal(err)
	}

	if res.Cid().String() != "z5hRLNd2sv4z1c" {
		t.Errorf("got wrong cid: %s", res.Cid().String())
	}
}

func (tp *provider) TestDagPath(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	sub, err := api.Dag().Put(ctx, strings.NewReader(`"foo"`))
	if err != nil {
		t.Fatal(err)
	}

	res, err := api.Dag().Put(ctx, strings.NewReader(`{"lnk": {"/": "`+sub.Cid().String()+`"}}`))
	if err != nil {
		t.Fatal(err)
	}

	p, err := coreiface.ParsePath(path.Join(res.Cid().String(), "lnk"))
	if err != nil {
		t.Error(err)
	}

	nd, err := api.Dag().Get(ctx, p)
	if err != nil {
		t.Error(err)
	}

	if nd.Cid().String() != sub.Cid().String() {
		t.Errorf("got unexpected cid %s, expected %s", nd.Cid().String(), sub.Cid().String())
	}
}

func (tp *provider) TestTree(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	c, err := api.Dag().Put(ctx, strings.NewReader(`{"a": 123, "b": "foo", "c": {"d": 321, "e": 111}}`))
	if err != nil {
		t.Fatal(err)
	}

	res, err := api.Dag().Get(ctx, c)
	if err != nil {
		t.Error(err)
	}

	lst := res.Tree("", -1)
	if len(lst) != len(treeExpected) {
		t.Errorf("tree length of %d doesn't match expected %d", len(lst), len(treeExpected))
	}

	for _, ent := range lst {
		if _, ok := treeExpected[ent]; !ok {
			t.Errorf("unexpected tree entry %s", ent)
		}
	}
}

func (tp *provider) TestBatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	batch := api.Dag().Batch(ctx)

	c, err := batch.Put(ctx, strings.NewReader(`"Hello"`))
	if err != nil {
		t.Fatal(err)
	}

	if c.Cid().String() != "zdpuAqckYF3ToF3gcJNxPZXmnmGuXd3gxHCXhq81HGxBejEvv" {
		t.Errorf("got wrong cid: %s", c.Cid().String())
	}

	_, err = api.Dag().Get(ctx, c)
	if err == nil || err.Error() != "merkledag: not found" {
		t.Error(err)
	}

	if err := batch.Commit(ctx); err != nil {
		t.Error(err)
	}

	_, err = api.Dag().Get(ctx, c)
	if err != nil {
		t.Error(err)
	}
}
