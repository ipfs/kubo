package coreapi_test

import (
	"context"
	"io"
	"io/ioutil"
	"math/rand"
	"testing"
	"time"

	files "gx/ipfs/QmSP88ryZkHSRn1fnngAaV2Vcn63WUJzAavnRM9CVdU1Ky/go-ipfs-cmdkit/files"
	ipath "gx/ipfs/QmcjwUb36Z16NJkvDX6ccXPqsFswo6AsRXynyXcLLCphV2/go-path"

	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	opt "github.com/ipfs/go-ipfs/core/coreapi/interface/options"
)

var rnd = rand.New(rand.NewSource(0x62796532303137))

func addTestObject(ctx context.Context, api coreiface.CoreAPI) (coreiface.Path, error) {
	return api.Unixfs().Add(ctx, files.NewReaderFile("", "", ioutil.NopCloser(&io.LimitedReader{R: rnd, N: 4092}), nil))
}

func TestBasicPublishResolve(t *testing.T) {
	ctx := context.Background()
	nds, apis, err := makeAPISwarm(ctx, true, 5)
	if err != nil {
		t.Fatal(err)
		return
	}
	n := nds[0]
	api := apis[0]

	p, err := addTestObject(ctx, api)
	if err != nil {
		t.Fatal(err)
		return
	}

	e, err := api.Name().Publish(ctx, p)
	if err != nil {
		t.Fatal(err)
		return
	}

	if e.Name() != n.Identity.Pretty() {
		t.Errorf("expected e.Name to equal '%s', got '%s'", n.Identity.Pretty(), e.Name())
	}

	if e.Value().String() != p.String() {
		t.Errorf("expected paths to match, '%s'!='%s'", e.Value().String(), p.String())
	}

	resPath, err := api.Name().Resolve(ctx, e.Name())
	if err != nil {
		t.Fatal(err)
		return
	}

	if resPath.String() != p.String() {
		t.Errorf("expected paths to match, '%s'!='%s'", resPath.String(), p.String())
	}
}

func TestBasicPublishResolveKey(t *testing.T) {
	ctx := context.Background()
	_, apis, err := makeAPISwarm(ctx, true, 5)
	if err != nil {
		t.Fatal(err)
		return
	}
	api := apis[0]

	k, err := api.Key().Generate(ctx, "foo")
	if err != nil {
		t.Fatal(err)
		return
	}

	p, err := addTestObject(ctx, api)
	if err != nil {
		t.Fatal(err)
		return
	}

	e, err := api.Name().Publish(ctx, p, opt.Name.Key(k.Name()))
	if err != nil {
		t.Fatal(err)
		return
	}

	if ipath.Join([]string{"/ipns", e.Name()}) != k.Path().String() {
		t.Errorf("expected e.Name to equal '%s', got '%s'", e.Name(), k.Path().String())
	}

	if e.Value().String() != p.String() {
		t.Errorf("expected paths to match, '%s'!='%s'", e.Value().String(), p.String())
	}

	resPath, err := api.Name().Resolve(ctx, e.Name())
	if err != nil {
		t.Fatal(err)
		return
	}

	if resPath.String() != p.String() {
		t.Errorf("expected paths to match, '%s'!='%s'", resPath.String(), p.String())
	}
}

func TestBasicPublishResolveTimeout(t *testing.T) {
	t.Skip("ValidTime doesn't appear to work at this time resolution")

	ctx := context.Background()
	nds, apis, err := makeAPISwarm(ctx, true, 5)
	if err != nil {
		t.Fatal(err)
		return
	}
	n := nds[0]
	api := apis[0]
	p, err := addTestObject(ctx, api)
	if err != nil {
		t.Fatal(err)
		return
	}

	e, err := api.Name().Publish(ctx, p, opt.Name.ValidTime(time.Millisecond*100))
	if err != nil {
		t.Fatal(err)
		return
	}

	if e.Name() != n.Identity.Pretty() {
		t.Errorf("expected e.Name to equal '%s', got '%s'", n.Identity.Pretty(), e.Name())
	}

	if e.Value().String() != p.String() {
		t.Errorf("expected paths to match, '%s'!='%s'", e.Value().String(), p.String())
	}

	time.Sleep(time.Second)

	_, err = api.Name().Resolve(ctx, e.Name())
	if err == nil {
		t.Fatal("Expected an error")
		return
	}
}

//TODO: When swarm api is created, add multinode tests
