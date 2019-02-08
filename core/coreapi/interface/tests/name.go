package tests

import (
	"context"
	"io"
	"math/rand"
	"path"
	"testing"
	"time"

	ipath "gx/ipfs/QmQ3YSqfxunT5QBg6KBVskKyRE26q6hjSMyhpxchpm7jEN/go-path"
	"gx/ipfs/QmaXvvAVAQ5ABqM5xtjYmV85xmN5MkWAZsX9H9Fwo4FVXp/go-ipfs-files"

	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	opt "github.com/ipfs/go-ipfs/core/coreapi/interface/options"
)

func (tp *provider) TestName(t *testing.T) {
	tp.hasApi(t, func(api coreiface.CoreAPI) error {
		if api.Name() == nil {
			return apiNotImplemented
		}
		return nil
	})

	t.Run("TestPublishResolve", tp.TestPublishResolve)
	t.Run("TestBasicPublishResolveKey", tp.TestBasicPublishResolveKey)
	t.Run("TestBasicPublishResolveTimeout", tp.TestBasicPublishResolveTimeout)
}

var rnd = rand.New(rand.NewSource(0x62796532303137))

func addTestObject(ctx context.Context, api coreiface.CoreAPI) (coreiface.Path, error) {
	return api.Unixfs().Add(ctx, files.NewReaderFile(&io.LimitedReader{R: rnd, N: 4092}))
}

func appendPath(p coreiface.Path, sub string) coreiface.Path {
	p, err := coreiface.ParsePath(path.Join(p.String(), sub))
	if err != nil {
		panic(err)
	}
	return p
}

func (tp *provider) TestPublishResolve(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	init := func() (coreiface.CoreAPI, coreiface.Path) {
		apis, err := tp.MakeAPISwarm(ctx, true, 5)
		if err != nil {
			t.Fatal(err)
			return nil, nil
		}
		api := apis[0]

		p, err := addTestObject(ctx, api)
		if err != nil {
			t.Fatal(err)
			return nil, nil
		}
		return api, p
	}

	run := func(t *testing.T, ropts []opt.NameResolveOption) {
		t.Run("basic", func(t *testing.T) {
			api, p := init()
			e, err := api.Name().Publish(ctx, p)
			if err != nil {
				t.Fatal(err)
			}

			self, err := api.Key().Self(ctx)
			if err != nil {
				t.Fatal(err)
			}

			if e.Name() != self.ID().Pretty() {
				t.Errorf("expected e.Name to equal '%s', got '%s'", self.ID().Pretty(), e.Name())
			}

			if e.Value().String() != p.String() {
				t.Errorf("expected paths to match, '%s'!='%s'", e.Value().String(), p.String())
			}

			resPath, err := api.Name().Resolve(ctx, e.Name(), ropts...)
			if err != nil {
				t.Fatal(err)
			}

			if resPath.String() != p.String() {
				t.Errorf("expected paths to match, '%s'!='%s'", resPath.String(), p.String())
			}
		})

		t.Run("publishPath", func(t *testing.T) {
			api, p := init()
			e, err := api.Name().Publish(ctx, appendPath(p, "/test"))
			if err != nil {
				t.Fatal(err)
			}

			self, err := api.Key().Self(ctx)
			if err != nil {
				t.Fatal(err)
			}

			if e.Name() != self.ID().Pretty() {
				t.Errorf("expected e.Name to equal '%s', got '%s'", self.ID().Pretty(), e.Name())
			}

			if e.Value().String() != p.String()+"/test" {
				t.Errorf("expected paths to match, '%s'!='%s'", e.Value().String(), p.String())
			}

			resPath, err := api.Name().Resolve(ctx, e.Name(), ropts...)
			if err != nil {
				t.Fatal(err)
			}

			if resPath.String() != p.String()+"/test" {
				t.Errorf("expected paths to match, '%s'!='%s'", resPath.String(), p.String()+"/test")
			}
		})

		t.Run("revolvePath", func(t *testing.T) {
			api, p := init()
			e, err := api.Name().Publish(ctx, p)
			if err != nil {
				t.Fatal(err)
			}

			self, err := api.Key().Self(ctx)
			if err != nil {
				t.Fatal(err)
			}

			if e.Name() != self.ID().Pretty() {
				t.Errorf("expected e.Name to equal '%s', got '%s'", self.ID().Pretty(), e.Name())
			}

			if e.Value().String() != p.String() {
				t.Errorf("expected paths to match, '%s'!='%s'", e.Value().String(), p.String())
			}

			resPath, err := api.Name().Resolve(ctx, e.Name()+"/test", ropts...)
			if err != nil {
				t.Fatal(err)
			}

			if resPath.String() != p.String()+"/test" {
				t.Errorf("expected paths to match, '%s'!='%s'", resPath.String(), p.String()+"/test")
			}
		})

		t.Run("publishRevolvePath", func(t *testing.T) {
			api, p := init()
			e, err := api.Name().Publish(ctx, appendPath(p, "/a"))
			if err != nil {
				t.Fatal(err)
			}

			self, err := api.Key().Self(ctx)
			if err != nil {
				t.Fatal(err)
			}

			if e.Name() != self.ID().Pretty() {
				t.Errorf("expected e.Name to equal '%s', got '%s'", self.ID().Pretty(), e.Name())
			}

			if e.Value().String() != p.String()+"/a" {
				t.Errorf("expected paths to match, '%s'!='%s'", e.Value().String(), p.String())
			}

			resPath, err := api.Name().Resolve(ctx, e.Name()+"/b", ropts...)
			if err != nil {
				t.Fatal(err)
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

func (tp *provider) TestBasicPublishResolveKey(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	apis, err := tp.MakeAPISwarm(ctx, true, 5)
	if err != nil {
		t.Fatal(err)
	}
	api := apis[0]

	k, err := api.Key().Generate(ctx, "foo")
	if err != nil {
		t.Fatal(err)
	}

	p, err := addTestObject(ctx, api)
	if err != nil {
		t.Fatal(err)
	}

	e, err := api.Name().Publish(ctx, p, opt.Name.Key(k.Name()))
	if err != nil {
		t.Fatal(err)
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
	}

	if resPath.String() != p.String() {
		t.Errorf("expected paths to match, '%s'!='%s'", resPath.String(), p.String())
	}
}

func (tp *provider) TestBasicPublishResolveTimeout(t *testing.T) {
	t.Skip("ValidTime doesn't appear to work at this time resolution")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	apis, err := tp.MakeAPISwarm(ctx, true, 5)
	if err != nil {
		t.Fatal(err)
	}
	api := apis[0]
	p, err := addTestObject(ctx, api)
	if err != nil {
		t.Fatal(err)
	}

	e, err := api.Name().Publish(ctx, p, opt.Name.ValidTime(time.Millisecond*100))
	if err != nil {
		t.Fatal(err)
	}

	self, err := api.Key().Self(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if e.Name() != self.ID().Pretty() {
		t.Errorf("expected e.Name to equal '%s', got '%s'", self.ID().Pretty(), e.Name())
	}

	if e.Value().String() != p.String() {
		t.Errorf("expected paths to match, '%s'!='%s'", e.Value().String(), p.String())
	}

	time.Sleep(time.Second)

	_, err = api.Name().Resolve(ctx, e.Name())
	if err == nil {
		t.Fatal("Expected an error")
	}
}

//TODO: When swarm api is created, add multinode tests
