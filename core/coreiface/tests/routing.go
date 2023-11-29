package tests

import (
	"context"
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

	err = api.Routing().Put(ctx, ipns.NamespacePrefix+name.String(), data, options.Put.AllowOffline(true))
	require.NoError(t, err)
}
