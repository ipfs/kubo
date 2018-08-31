package coreapi_test

import (
	"context"
	"strings"
	"testing"

	opt "github.com/ipfs/go-ipfs/core/coreapi/interface/options"
)

func TestListSelf(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
		return
	}

	keys, err := api.Key().List(ctx)
	if err != nil {
		t.Fatalf("failed to list keys: %s", err)
		return
	}

	if len(keys) != 1 {
		t.Fatalf("there should be 1 key (self), got %d", len(keys))
		return
	}

	if keys[0].Name() != "self" {
		t.Errorf("expected the key to be called 'self', got '%s'", keys[0].Name())
	}

	if keys[0].Path().String() != "/ipns/"+testPeerID {
		t.Errorf("expected the key to have path '/ipns/%s', got '%s'", testPeerID, keys[0].Path().String())
	}
}

func TestRenameSelf(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
		return
	}

	_, _, err = api.Key().Rename(ctx, "self", "foo")
	if err == nil {
		t.Error("expected error to not be nil")
	} else {
		if err.Error() != "cannot rename key with name 'self'" {
			t.Fatalf("expected error 'cannot rename key with name 'self'', got '%s'", err.Error())
		}
	}

	_, _, err = api.Key().Rename(ctx, "self", "foo", opt.Key.Force(true))
	if err == nil {
		t.Error("expected error to not be nil")
	} else {
		if err.Error() != "cannot rename key with name 'self'" {
			t.Fatalf("expected error 'cannot rename key with name 'self'', got '%s'", err.Error())
		}
	}
}

func TestRemoveSelf(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
		return
	}

	_, err = api.Key().Remove(ctx, "self")
	if err == nil {
		t.Error("expected error to not be nil")
	} else {
		if err.Error() != "cannot remove key with name 'self'" {
			t.Fatalf("expected error 'cannot remove key with name 'self'', got '%s'", err.Error())
		}
	}
}

func TestGenerate(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	k, err := api.Key().Generate(ctx, "foo")
	if err != nil {
		t.Fatal(err)
		return
	}

	if k.Name() != "foo" {
		t.Errorf("expected the key to be called 'foo', got '%s'", k.Name())
	}

	if !strings.HasPrefix(k.Path().String(), "/ipns/Qm") {
		t.Errorf("expected the key to be prefixed with '/ipns/Qm', got '%s'", k.Path().String())
	}
}

func TestGenerateSize(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	k, err := api.Key().Generate(ctx, "foo", opt.Key.Size(1024))
	if err != nil {
		t.Fatal(err)
		return
	}

	if k.Name() != "foo" {
		t.Errorf("expected the key to be called 'foo', got '%s'", k.Name())
	}

	if !strings.HasPrefix(k.Path().String(), "/ipns/Qm") {
		t.Errorf("expected the key to be prefixed with '/ipns/Qm', got '%s'", k.Path().String())
	}
}

func TestGenerateType(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	k, err := api.Key().Generate(ctx, "bar", opt.Key.Type(opt.Ed25519Key))
	if err != nil {
		t.Fatal(err)
		return
	}

	if k.Name() != "bar" {
		t.Errorf("expected the key to be called 'foo', got '%s'", k.Name())
	}

	// Expected to be an inlined identity hash.
	if !strings.HasPrefix(k.Path().String(), "/ipns/12") {
		t.Errorf("expected the key to be prefixed with '/ipns/12', got '%s'", k.Path().String())
	}
}

func TestGenerateExisting(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	_, err = api.Key().Generate(ctx, "foo")
	if err != nil {
		t.Fatal(err)
		return
	}

	_, err = api.Key().Generate(ctx, "foo")
	if err == nil {
		t.Error("expected error to not be nil")
	} else {
		if err.Error() != "key with name 'foo' already exists" {
			t.Fatalf("expected error 'key with name 'foo' already exists', got '%s'", err.Error())
		}
	}

	_, err = api.Key().Generate(ctx, "self")
	if err == nil {
		t.Error("expected error to not be nil")
	} else {
		if err.Error() != "cannot create key with name 'self'" {
			t.Fatalf("expected error 'cannot create key with name 'self'', got '%s'", err.Error())
		}
	}
}

func TestList(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	_, err = api.Key().Generate(ctx, "foo")
	if err != nil {
		t.Fatal(err)
		return
	}

	l, err := api.Key().List(ctx)
	if err != nil {
		t.Fatal(err)
		return
	}

	if len(l) != 2 {
		t.Fatalf("expected to get 2 keys, got %d", len(l))
		return
	}

	if l[0].Name() != "self" {
		t.Fatalf("expected key 0 to be called 'self', got '%s'", l[0].Name())
		return
	}

	if l[1].Name() != "foo" {
		t.Fatalf("expected key 1 to be called 'foo', got '%s'", l[1].Name())
		return
	}

	if !strings.HasPrefix(l[0].Path().String(), "/ipns/Qm") {
		t.Fatalf("expected key 0 to be prefixed with '/ipns/Qm', got '%s'", l[0].Name())
		return
	}

	if !strings.HasPrefix(l[1].Path().String(), "/ipns/Qm") {
		t.Fatalf("expected key 1 to be prefixed with '/ipns/Qm', got '%s'", l[1].Name())
		return
	}
}

func TestRename(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	_, err = api.Key().Generate(ctx, "foo")
	if err != nil {
		t.Fatal(err)
		return
	}

	k, overwrote, err := api.Key().Rename(ctx, "foo", "bar")
	if err != nil {
		t.Fatal(err)
		return
	}

	if overwrote {
		t.Error("overwrote should be false")
	}

	if k.Name() != "bar" {
		t.Errorf("returned key should be called 'bar', got '%s'", k.Name())
	}
}

func TestRenameToSelf(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	_, err = api.Key().Generate(ctx, "foo")
	if err != nil {
		t.Fatal(err)
		return
	}

	_, _, err = api.Key().Rename(ctx, "foo", "self")
	if err == nil {
		t.Error("expected error to not be nil")
	} else {
		if err.Error() != "cannot overwrite key with name 'self'" {
			t.Fatalf("expected error 'cannot overwrite key with name 'self'', got '%s'", err.Error())
		}
	}
}

func TestRenameToSelfForce(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	_, err = api.Key().Generate(ctx, "foo")
	if err != nil {
		t.Fatal(err)
		return
	}

	_, _, err = api.Key().Rename(ctx, "foo", "self", opt.Key.Force(true))
	if err == nil {
		t.Error("expected error to not be nil")
	} else {
		if err.Error() != "cannot overwrite key with name 'self'" {
			t.Fatalf("expected error 'cannot overwrite key with name 'self'', got '%s'", err.Error())
		}
	}
}

func TestRenameOverwriteNoForce(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	_, err = api.Key().Generate(ctx, "foo")
	if err != nil {
		t.Fatal(err)
		return
	}

	_, err = api.Key().Generate(ctx, "bar")
	if err != nil {
		t.Fatal(err)
		return
	}

	_, _, err = api.Key().Rename(ctx, "foo", "bar")
	if err == nil {
		t.Error("expected error to not be nil")
	} else {
		if err.Error() != "key by that name already exists, refusing to overwrite" {
			t.Fatalf("expected error 'key by that name already exists, refusing to overwrite', got '%s'", err.Error())
		}
	}
}

func TestRenameOverwrite(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	kfoo, err := api.Key().Generate(ctx, "foo")
	if err != nil {
		t.Fatal(err)
		return
	}

	_, err = api.Key().Generate(ctx, "bar")
	if err != nil {
		t.Fatal(err)
		return
	}

	k, overwrote, err := api.Key().Rename(ctx, "foo", "bar", opt.Key.Force(true))
	if err != nil {
		t.Fatal(err)
		return
	}

	if !overwrote {
		t.Error("overwrote should be true")
	}

	if k.Name() != "bar" {
		t.Errorf("returned key should be called 'bar', got '%s'", k.Name())
	}

	if k.Path().String() != kfoo.Path().String() {
		t.Errorf("k and kfoo should have equal paths, '%s'!='%s'", k.Path().String(), kfoo.Path().String())
	}
}

func TestRemove(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Error(err)
	}

	k, err := api.Key().Generate(ctx, "foo")
	if err != nil {
		t.Fatal(err)
		return
	}

	l, err := api.Key().List(ctx)
	if err != nil {
		t.Fatal(err)
		return
	}

	if len(l) != 2 {
		t.Fatalf("expected to get 2 keys, got %d", len(l))
		return
	}

	p, err := api.Key().Remove(ctx, "foo")
	if err != nil {
		t.Fatal(err)
		return
	}

	if k.Path().String() != p.Path().String() {
		t.Errorf("k and p should have equal paths, '%s'!='%s'", k.Path().String(), p.Path().String())
	}

	l, err = api.Key().List(ctx)
	if err != nil {
		t.Fatal(err)
		return
	}

	if len(l) != 1 {
		t.Fatalf("expected to get 1 key, got %d", len(l))
		return
	}

	if l[0].Name() != "self" {
		t.Errorf("expected the key to be called 'self', got '%s'", l[0].Name())
	}
}
