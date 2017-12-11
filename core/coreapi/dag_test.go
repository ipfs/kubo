package coreapi_test

import (
	"context"
	"path"
	"strings"
	"testing"

	coreapi "github.com/ipfs/go-ipfs/core/coreapi"
)

var (
	treeExpected = map[string]struct{}{
		"a":   {},
		"b":   {},
		"c":   {},
		"c/d": {},
		"c/e": {},
	}
)

func TestPut(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	res, err := api.Dag().Put(ctx, strings.NewReader(`"Hello"`), "json", nil)
	if err != nil {
		t.Error(err)
	}

	if res[0].Cid().String() != "zdpuAqckYF3ToF3gcJNxPZXmnmGuXd3gxHCXhq81HGxBejEvv" {
		t.Errorf("got wrong cid: %s", res[0].Cid().String())
	}
}

func TestPath(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	sub, err := api.Dag().Put(ctx, strings.NewReader(`"foo"`), "json", nil)
	if err != nil {
		t.Error(err)
	}

	res, err := api.Dag().Put(ctx, strings.NewReader(`{"lnk": {"/": "`+sub[0].Cid().String()+`"}}`), "json", nil)
	if err != nil {
		t.Error(err)
	}

	p, err := coreapi.ParsePath(path.Join(res[0].Cid().String(), "lnk"))
	if err != nil {
		t.Error(err)
	}

	nd, err := api.Dag().Get(ctx, p)
	if err != nil {
		t.Error(err)
	}

	if nd.Cid().String() != sub[0].Cid().String() {
		t.Errorf("got unexpected cid %s, expected %s", nd.Cid().String(), sub[0].Cid().String())
	}
}

func TestTree(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	res, err := api.Dag().Put(ctx, strings.NewReader(`{"a": 123, "b": "foo", "c": {"d": 321, "e": 111}}`), "json", nil)
	if err != nil {
		t.Error(err)
	}

	lst := res[0].Tree("", -1)
	if len(lst) != len(treeExpected) {
		t.Errorf("tree length of %d doesn't match expected %d", len(lst), len(treeExpected))
	}

	for _, ent := range lst {
		if _, ok := treeExpected[ent]; !ok {
			t.Errorf("unexpected tree entry %s", ent)
		}
	}
}
