package coreapi_test

import (
	"context"
	"github.com/ipfs/go-ipfs/core"
	"io"
	"io/ioutil"
	"math/rand"
	"path"
	"testing"
	"time"

	ipath "gx/ipfs/QmVi2uUygezqaMTqs3Yzt5FcZFHJoYD4B7jQ2BELjj7ZuY/go-path"
	files "gx/ipfs/QmZMWMvWMVKCbHetJ4RgndbuEF1io2UpUxwQwtNjtYPzSC/go-ipfs-files"

	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	opt "github.com/ipfs/go-ipfs/core/coreapi/interface/options"
)

var rnd = rand.New(rand.NewSource(0x62796532303137))

func addTestObject(ctx context.Context, api coreiface.CoreAPI) (coreiface.Path, error) {
	return api.Unixfs().Add(ctx, files.NewReaderFile("", "", ioutil.NopCloser(&io.LimitedReader{R: rnd, N: 4092}), nil))
}

func appendPath(p coreiface.Path, sub string) coreiface.Path {
	p, err := coreiface.ParsePath(path.Join(p.String(), sub))
	if err != nil {
		panic(err)
	}
	return p
}

func TestPublishResolve(t *testing.T) {
	ctx := context.Background()
	init := func() (*core.IpfsNode, coreiface.CoreAPI, coreiface.Path) {
		nds, apis, err := makeAPISwarm(ctx, true, 5)
		if err != nil {
			t.Fatal(err)
			return nil, nil, nil
		}
		n := nds[0]
		api := apis[0]

		p, err := addTestObject(ctx, api)
		if err != nil {
			t.Fatal(err)
			return nil, nil, nil
		}
		return n, api, p
	}

	run := func(t *testing.T, ropts []opt.NameResolveOption) {
		t.Run("basic", func(t *testing.T) {
			n, api, p := init()
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

			resPath, err := api.Name().Resolve(ctx, e.Name(), ropts...)
			if err != nil {
				t.Fatal(err)
				return
			}

			if resPath.String() != p.String() {
				t.Errorf("expected paths to match, '%s'!='%s'", resPath.String(), p.String())
			}
		})

		t.Run("publishPath", func(t *testing.T) {
			n, api, p := init()
			e, err := api.Name().Publish(ctx, appendPath(p, "/test"))
			if err != nil {
				t.Fatal(err)
				return
			}

			if e.Name() != n.Identity.Pretty() {
				t.Errorf("expected e.Name to equal '%s', got '%s'", n.Identity.Pretty(), e.Name())
			}

			if e.Value().String() != p.String()+"/test" {
				t.Errorf("expected paths to match, '%s'!='%s'", e.Value().String(), p.String())
			}

			resPath, err := api.Name().Resolve(ctx, e.Name(), ropts...)
			if err != nil {
				t.Fatal(err)
				return
			}

			if resPath.String() != p.String()+"/test" {
				t.Errorf("expected paths to match, '%s'!='%s'", resPath.String(), p.String()+"/test")
			}
		})

		t.Run("revolvePath", func(t *testing.T) {
			n, api, p := init()
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

			resPath, err := api.Name().Resolve(ctx, e.Name()+"/test", ropts...)
			if err != nil {
				t.Fatal(err)
				return
			}

			if resPath.String() != p.String()+"/test" {
				t.Errorf("expected paths to match, '%s'!='%s'", resPath.String(), p.String()+"/test")
			}
		})

		t.Run("publishRevolvePath", func(t *testing.T) {
			n, api, p := init()
			e, err := api.Name().Publish(ctx, appendPath(p, "/a"))
			if err != nil {
				t.Fatal(err)
				return
			}

			if e.Name() != n.Identity.Pretty() {
				t.Errorf("expected e.Name to equal '%s', got '%s'", n.Identity.Pretty(), e.Name())
			}

			if e.Value().String() != p.String()+"/a" {
				t.Errorf("expected paths to match, '%s'!='%s'", e.Value().String(), p.String())
			}

			resPath, err := api.Name().Resolve(ctx, e.Name()+"/b", ropts...)
			if err != nil {
				t.Fatal(err)
				return
			}

			if resPath.String() != p.String()+"/a/b" {
				t.Errorf("expected paths to match, '%s'!='%s'", resPath.String(), p.String()+"/a/b")
			}
		})
	}

	t.Run("default", func(t *testing.T) {
		run(t, []opt.NameResolveOption{})
	})

	t.Run("nocache", func(t *testing.T) {
		run(t, []opt.NameResolveOption{opt.Name.Cache(false)})
	})
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
