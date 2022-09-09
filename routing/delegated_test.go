package routing

import (
	"testing"

	"github.com/ipfs/kubo/config"
	"github.com/stretchr/testify/require"
)

func TestRoutingFromConfig(t *testing.T) {
	require := require.New(t)

	r, err := routingFromConfig(config.Router{
		Type: "unknown",
	}, nil)

	require.Nil(r)
	require.EqualError(err, "unknown router type unknown")

	r, err = routingFromConfig(config.Router{
		Type:       config.RouterTypeReframe,
		Parameters: &config.ReframeRouterParams{},
	}, nil)

	require.Nil(r)
	require.EqualError(err, "configuration param 'Endpoint' is needed for reframe delegated routing types")

	r, err = routingFromConfig(config.Router{
		Type: config.RouterTypeReframe,
		Parameters: &config.ReframeRouterParams{
			Endpoint: "test",
		},
	}, nil)

	require.NoError(err)
	require.NotNil(r)
}

func TestParser(t *testing.T) {
	require := require.New(t)

	router, err := Parse(config.Routers{
		"r1": config.RouterParser{
			Router: config.Router{
				Type:    config.RouterTypeReframe,
				Enabled: config.True,
				Parameters: &config.ReframeRouterParams{
					Endpoint: "testEndpoint",
				},
			},
		},
		"r2": config.RouterParser{
			Router: config.Router{
				Type:    config.RouterTypeSequential,
				Enabled: config.True,
				Parameters: &config.ComposableRouterParams{
					Routers: []config.ConfigRouter{
						{
							RouterName: "r1",
						},
					},
				},
			},
		},
	}, config.Methods{
		config.MethodNameFindPeers: config.Method{
			RouterName: "r1",
		},
		config.MethodNameFindProviders: config.Method{
			RouterName: "r1",
		},
		config.MethodNameGetIPNS: config.Method{
			RouterName: "r1",
		},
		config.MethodNamePutIPNS: config.Method{
			RouterName: "r2",
		},
		config.MethodNameProvide: config.Method{
			RouterName: "r2",
		},
	}, &ExtraDHTParams{})

	require.NoError(err)

	comp, ok := router.(*Composer)
	require.True(ok)

	require.Equal(comp.FindPeersRouter, comp.FindProvidersRouter)
	require.Equal(comp.ProvideRouter, comp.PutValueRouter)
}
