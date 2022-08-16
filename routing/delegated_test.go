package routing

import (
	"context"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/kubo/config"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	"github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/require"
)

func TestPriority(t *testing.T) {
	require := require.New(t)
	params := make(map[string]string)
	p := GetPriority(params)

	require.Equal(defaultPriority, p)

	params[string(config.RouterParamPriority)] = "101"

	p = GetPriority(params)

	require.Equal(101, p)

	params[string(config.RouterParamPriority)] = "NAN"

	p = GetPriority(params)

	require.Equal(defaultPriority, p)
}

func TestRoutingFromConfig(t *testing.T) {
	require := require.New(t)

	r, err := RoutingFromConfig(config.Router{
		Type: "unknown",
	})

	require.Nil(r)
	require.EqualError(err, "router type unknown is not supported")

	r, err = RoutingFromConfig(config.Router{
		Type:       string(config.RouterTypeReframe),
		Parameters: make(map[string]string),
	})

	require.Nil(r)
	require.EqualError(err, "configuration param 'Endpoint' is needed for reframe delegated routing types")

	r, err = RoutingFromConfig(config.Router{
		Type: string(config.RouterTypeReframe),
		Parameters: map[string]string{
			string(config.RouterParamEndpoint): "test",
		},
	})

	require.NotNil(r)
	require.NoError(err)
}

func TestTieredRouter(t *testing.T) {
	require := require.New(t)

	tr := &Tiered{
		Tiered: routinghelpers.Tiered{
			Routers: []routing.Routing{routinghelpers.Null{}},
		},
	}

	pm := tr.ProvideMany()
	require.Nil(pm)

	tr.Tiered.Routers = append(tr.Tiered.Routers, &dummyRouter{})

	pm = tr.ProvideMany()
	require.NotNil(pm)
}

type dummyRouter struct {
}

func (dr *dummyRouter) Provide(context.Context, cid.Cid, bool) error {
	panic("not implemented")

}

func (dr *dummyRouter) FindProvidersAsync(context.Context, cid.Cid, int) <-chan peer.AddrInfo {
	panic("not implemented")
}

func (dr *dummyRouter) FindPeer(context.Context, peer.ID) (peer.AddrInfo, error) {
	panic("not implemented")
}

func (dr *dummyRouter) PutValue(context.Context, string, []byte, ...routing.Option) error {
	panic("not implemented")
}

func (dr *dummyRouter) GetValue(context.Context, string, ...routing.Option) ([]byte, error) {
	panic("not implemented")
}

func (dr *dummyRouter) SearchValue(context.Context, string, ...routing.Option) (<-chan []byte, error) {
	panic("not implemented")
}

func (dr *dummyRouter) Bootstrap(context.Context) error {
	panic("not implemented")
}

func (dr *dummyRouter) ProvideMany(ctx context.Context, keys []multihash.Multihash) error {
	panic("not implemented")
}

func (dr *dummyRouter) Ready() bool {
	panic("not implemented")
}
