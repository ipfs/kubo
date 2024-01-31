package tests

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/ipfs/boxo/ipns"
	"github.com/ipfs/boxo/path"
	iface "github.com/ipfs/kubo/core/coreiface"
	"github.com/ipfs/kubo/core/coreiface/options"
	"github.com/stretchr/testify/require"
)

func (tp *TestSuite) TestRouting(t *testing.T) {
	tp.hasApi(t, func(api iface.CoreAPI) error {
		if api.Routing() == nil {
			return errAPINotImplemented
		}
		return nil
	})

	t.Run("TestRoutingGet", tp.TestRoutingGet)
	t.Run("TestRoutingPut", tp.TestRoutingPut)
	t.Run("TestRoutingPutOffline", tp.TestRoutingPutOffline)
	t.Run("TestRoutingFindPeer", tp.TestRoutingFindPeer)
	t.Run("TestRoutingFindProviders", tp.TestRoutingFindProviders)
	t.Run("TestRoutingProvide", tp.TestRoutingProvide)
}

func (tp *TestSuite) testRoutingPublishKey(t *testing.T, ctx context.Context, api iface.CoreAPI, opts ...options.NamePublishOption) (path.Path, ipns.Name) {
	p, err := addTestObject(ctx, api)
	require.NoError(t, err)

	name, err := api.Name().Publish(ctx, p, opts...)
	require.NoError(t, err)

	time.Sleep(3 * time.Second)
	return p, name
}

func (tp *TestSuite) TestRoutingGet(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	apis, err := tp.MakeAPISwarm(t, ctx, 2)
	require.NoError(t, err)

	// Node 1: publishes an IPNS name
	p, name := tp.testRoutingPublishKey(t, ctx, apis[0])

	// Node 2: retrieves the best value for the IPNS name.
	data, err := apis[1].Routing().Get(ctx, ipns.NamespacePrefix+name.String())
	require.NoError(t, err)

	rec, err := ipns.UnmarshalRecord(data)
	require.NoError(t, err)

	val, err := rec.Value()
	require.NoError(t, err)
	require.Equal(t, p.String(), val.String())
}

func (tp *TestSuite) TestRoutingPut(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	apis, err := tp.MakeAPISwarm(t, ctx, 2)
	require.NoError(t, err)

	// Create and publish IPNS entry.
	_, name := tp.testRoutingPublishKey(t, ctx, apis[0])

	// Get valid routing value.
	data, err := apis[0].Routing().Get(ctx, ipns.NamespacePrefix+name.String())
	require.NoError(t, err)

	// Put routing value.
	err = apis[1].Routing().Put(ctx, ipns.NamespacePrefix+name.String(), data)
	require.NoError(t, err)
}

func (tp *TestSuite) TestRoutingPutOffline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// init a swarm & publish an IPNS entry to get a valid payload
	apis, err := tp.MakeAPISwarm(t, ctx, 2)
	require.NoError(t, err)

	_, name := tp.testRoutingPublishKey(t, ctx, apis[0], options.Name.AllowOffline(true))
	data, err := apis[0].Routing().Get(ctx, ipns.NamespacePrefix+name.String())
	require.NoError(t, err)

	// init our offline node and try to put the payload
	api, err := tp.makeAPIWithIdentityAndOffline(t, ctx)
	require.NoError(t, err)

	err = api.Routing().Put(ctx, ipns.NamespacePrefix+name.String(), data)
	require.Error(t, err, "this operation should fail because we are offline")

	err = api.Routing().Put(ctx, ipns.NamespacePrefix+name.String(), data, options.Routing.AllowOffline(true))
	require.NoError(t, err)
}

func (tp *TestSuite) TestRoutingFindPeer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	apis, err := tp.MakeAPISwarm(t, ctx, 5)
	if err != nil {
		t.Fatal(err)
	}

	self0, err := apis[0].Key().Self(ctx)
	if err != nil {
		t.Fatal(err)
	}

	laddrs0, err := apis[0].Swarm().LocalAddrs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(laddrs0) != 1 {
		t.Fatal("unexpected number of local addrs")
	}

	time.Sleep(3 * time.Second)

	pi, err := apis[2].Routing().FindPeer(ctx, self0.ID())
	if err != nil {
		t.Fatal(err)
	}

	if pi.Addrs[0].String() != laddrs0[0].String() {
		t.Errorf("got unexpected address from FindPeer: %s", pi.Addrs[0].String())
	}

	self2, err := apis[2].Key().Self(ctx)
	if err != nil {
		t.Fatal(err)
	}

	pi, err = apis[1].Routing().FindPeer(ctx, self2.ID())
	if err != nil {
		t.Fatal(err)
	}

	laddrs2, err := apis[2].Swarm().LocalAddrs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(laddrs2) != 1 {
		t.Fatal("unexpected number of local addrs")
	}

	if pi.Addrs[0].String() != laddrs2[0].String() {
		t.Errorf("got unexpected address from FindPeer: %s", pi.Addrs[0].String())
	}
}

func (tp *TestSuite) TestRoutingFindProviders(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	apis, err := tp.MakeAPISwarm(t, ctx, 5)
	if err != nil {
		t.Fatal(err)
	}

	p, err := addTestObject(ctx, apis[0])
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(3 * time.Second)

	out, err := apis[2].Routing().FindProviders(ctx, p, options.Routing.NumProviders(1))
	if err != nil {
		t.Fatal(err)
	}

	provider := <-out

	self0, err := apis[0].Key().Self(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if provider.ID.String() != self0.ID().String() {
		t.Errorf("got wrong provider: %s != %s", provider.ID.String(), self0.ID().String())
	}
}

func (tp *TestSuite) TestRoutingProvide(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	apis, err := tp.MakeAPISwarm(t, ctx, 5)
	if err != nil {
		t.Fatal(err)
	}

	off0, err := apis[0].WithOptions(options.Api.Offline(true))
	if err != nil {
		t.Fatal(err)
	}

	s, err := off0.Block().Put(ctx, &io.LimitedReader{R: rnd, N: 4092})
	if err != nil {
		t.Fatal(err)
	}

	p := s.Path()

	time.Sleep(3 * time.Second)

	out, err := apis[2].Routing().FindProviders(ctx, p, options.Routing.NumProviders(1))
	if err != nil {
		t.Fatal(err)
	}

	_, ok := <-out

	if ok {
		t.Fatal("did not expect to find any providers")
	}

	self0, err := apis[0].Key().Self(ctx)
	if err != nil {
		t.Fatal(err)
	}

	err = apis[0].Routing().Provide(ctx, p)
	if err != nil {
		t.Fatal(err)
	}

	out, err = apis[2].Routing().FindProviders(ctx, p, options.Routing.NumProviders(1))
	if err != nil {
		t.Fatal(err)
	}

	provider := <-out

	if provider.ID.String() != self0.ID().String() {
		t.Errorf("got wrong provider: %s != %s", provider.ID.String(), self0.ID().String())
	}
}
