package coreapi_test

import (
	"context"
	"strings"
	"testing"
)

func TestMutablePath(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
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

	if blk.Mutable() {
		t.Error("expected /ipld path to be immutable")
	}
}
