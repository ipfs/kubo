package tests

import (
	"context"
	"io"
	"math/rand"
	"testing"
	"time"

	"github.com/ipfs/boxo/files"
	"github.com/ipfs/boxo/ipns"
	"github.com/ipfs/boxo/path"
	coreiface "github.com/ipfs/kubo/core/coreiface"
	opt "github.com/ipfs/kubo/core/coreiface/options"
	"github.com/stretchr/testify/require"
)

func (tp *TestSuite) TestName(t *testing.T) {
	tp.hasApi(t, func(api coreiface.CoreAPI) error {
		if api.Name() == nil {
			return errAPINotImplemented
		}
		return nil
	})

	t.Run("TestPublishResolve", tp.TestPublishResolve)
	t.Run("TestBasicPublishResolveKey", tp.TestBasicPublishResolveKey)
	t.Run("TestBasicPublishResolveTimeout", tp.TestBasicPublishResolveTimeout)
}

var rnd = rand.New(rand.NewSource(0x62796532303137))

func addTestObject(ctx context.Context, api coreiface.CoreAPI) (path.Path, error) {
	return api.Unixfs().Add(ctx, files.NewReaderFile(&io.LimitedReader{R: rnd, N: 4092}))
}

func (tp *TestSuite) TestPublishResolve(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	init := func() (coreiface.CoreAPI, path.Path) {
		apis, err := tp.MakeAPISwarm(t, ctx, 5)
		require.NoError(t, err)
		api := apis[0]

		p, err := addTestObject(ctx, api)
		require.NoError(t, err)
		return api, p
	}
	run := func(t *testing.T, ropts []opt.NameResolveOption) {
		t.Run("basic", func(t *testing.T) {
			api, p := init()
			name, err := api.Name().Publish(ctx, p)
			require.NoError(t, err)

			self, err := api.Key().Self(ctx)
			require.NoError(t, err)
			require.Equal(t, name.String(), ipns.NameFromPeer(self.ID()).String())

			resPath, err := api.Name().Resolve(ctx, name.String(), ropts...)
			require.NoError(t, err)
			require.Equal(t, p.String(), resPath.String())
		})

		t.Run("publishPath", func(t *testing.T) {
			api, p := init()
			p, err := path.Join(p, "/test")
			require.NoError(t, err)

			name, err := api.Name().Publish(ctx, p)
			require.NoError(t, err)

			self, err := api.Key().Self(ctx)
			require.NoError(t, err)
			require.Equal(t, name.String(), ipns.NameFromPeer(self.ID()).String())

			resPath, err := api.Name().Resolve(ctx, name.String(), ropts...)
			require.NoError(t, err)
			require.Equal(t, p.String(), resPath.String())
		})

		t.Run("revolvePath", func(t *testing.T) {
			api, p := init()
			name, err := api.Name().Publish(ctx, p)
			require.NoError(t, err)

			self, err := api.Key().Self(ctx)
			require.NoError(t, err)
			require.Equal(t, name.String(), ipns.NameFromPeer(self.ID()).String())

			resPath, err := api.Name().Resolve(ctx, name.String()+"/test", ropts...)
			require.NoError(t, err)
			require.Equal(t, p.String()+"/test", resPath.String())
		})

		t.Run("publishRevolvePath", func(t *testing.T) {
			api, p := init()
			p, err := path.Join(p, "/a")
			require.NoError(t, err)

			name, err := api.Name().Publish(ctx, p)
			require.NoError(t, err)

			self, err := api.Key().Self(ctx)
			require.NoError(t, err)
			require.Equal(t, name.String(), ipns.NameFromPeer(self.ID()).String())

			resPath, err := api.Name().Resolve(ctx, name.String()+"/b", ropts...)
			require.NoError(t, err)
			require.Equal(t, p.String()+"/b", resPath.String())
		})
	}

	t.Run("default", func(t *testing.T) {
		run(t, []opt.NameResolveOption{})
	})

	t.Run("nocache", func(t *testing.T) {
		run(t, []opt.NameResolveOption{opt.Name.Cache(false)})
	})
}

func (tp *TestSuite) TestBasicPublishResolveKey(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	apis, err := tp.MakeAPISwarm(t, ctx, 5)
	require.NoError(t, err)
	api := apis[0]

	k, err := api.Key().Generate(ctx, "foo")
	require.NoError(t, err)

	p, err := addTestObject(ctx, api)
	require.NoError(t, err)

	name, err := api.Name().Publish(ctx, p, opt.Name.Key(k.Name()))
	require.NoError(t, err)
	require.Equal(t, name.String(), ipns.NameFromPeer(k.ID()).String())

	resPath, err := api.Name().Resolve(ctx, name.String())
	require.NoError(t, err)
	require.Equal(t, p.String(), resPath.String())
}

func (tp *TestSuite) TestBasicPublishResolveTimeout(t *testing.T) {
	t.Skip("ValidTime doesn't appear to work at this time resolution")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	apis, err := tp.MakeAPISwarm(t, ctx, 5)
	require.NoError(t, err)
	api := apis[0]
	p, err := addTestObject(ctx, api)
	require.NoError(t, err)

	self, err := api.Key().Self(ctx)
	require.NoError(t, err)

	name, err := api.Name().Publish(ctx, p, opt.Name.ValidTime(time.Millisecond*100))
	require.NoError(t, err)
	require.Equal(t, name.String(), ipns.NameFromPeer(self.ID()).String())

	time.Sleep(time.Second)

	_, err = api.Name().Resolve(ctx, name.String())
	require.NoError(t, err)
}

// TODO: When swarm api is created, add multinode tests
