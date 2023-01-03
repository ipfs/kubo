package routing

import (
	"encoding/base64"
	"testing"

	"github.com/ipfs/kubo/config"
	crypto "github.com/libp2p/go-libp2p/core/crypto"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

func TestReframeRoutingFromConfig(t *testing.T) {
	require := require.New(t)

	r, err := reframeRoutingFromConfig(config.Router{
		Type:       config.RouterTypeReframe,
		Parameters: &config.ReframeRouterParams{},
	}, nil)

	require.Nil(r)
	require.EqualError(err, "configuration param 'Endpoint' is needed for reframe delegated routing types")

	r, err = reframeRoutingFromConfig(config.Router{
		Type: config.RouterTypeReframe,
		Parameters: &config.ReframeRouterParams{
			Endpoint: "test",
		},
	}, nil)

	require.NoError(err)
	require.NotNil(r)

	priv, pub, err := crypto.GenerateKeyPair(crypto.RSA, 2048)
	require.NoError(err)

	id, err := peer.IDFromPublicKey(pub)
	require.NoError(err)

	privM, err := crypto.MarshalPrivateKey(priv)
	require.NoError(err)

	r, err = reframeRoutingFromConfig(config.Router{
		Type: config.RouterTypeReframe,
		Parameters: &config.ReframeRouterParams{
			Endpoint: "test",
		},
	}, &ExtraHTTPParams{
		PeerID:     id.String(),
		Addrs:      []string{"/ip4/0.0.0.0/tcp/4001"},
		PrivKeyB64: base64.StdEncoding.EncodeToString(privM),
	})

	require.NotNil(r)
	require.NoError(err)
}

func TestParser(t *testing.T) {
	require := require.New(t)

	router, err := Parse(config.Routers{
		"r1": config.RouterParser{
			Router: config.Router{
				Type: config.RouterTypeReframe,
				Parameters: &config.ReframeRouterParams{
					Endpoint: "testEndpoint",
				},
			},
		},
		"r2": config.RouterParser{
			Router: config.Router{
				Type: config.RouterTypeSequential,
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
	}, &ExtraDHTParams{}, nil)

	require.NoError(err)

	comp, ok := router.(*Composer)
	require.True(ok)

	require.Equal(comp.FindPeersRouter, comp.FindProvidersRouter)
	require.Equal(comp.ProvideRouter, comp.PutValueRouter)
}

func TestParserRecursive(t *testing.T) {
	require := require.New(t)

	router, err := Parse(config.Routers{
		"reframe1": config.RouterParser{
			Router: config.Router{
				Type: config.RouterTypeReframe,
				Parameters: &config.ReframeRouterParams{
					Endpoint: "testEndpoint1",
				},
			},
		},
		"reframe2": config.RouterParser{
			Router: config.Router{
				Type: config.RouterTypeReframe,
				Parameters: &config.ReframeRouterParams{
					Endpoint: "testEndpoint2",
				},
			},
		},
		"reframe3": config.RouterParser{
			Router: config.Router{
				Type: config.RouterTypeReframe,
				Parameters: &config.ReframeRouterParams{
					Endpoint: "testEndpoint3",
				},
			},
		},
		"composable1": config.RouterParser{
			Router: config.Router{
				Type: config.RouterTypeSequential,
				Parameters: &config.ComposableRouterParams{
					Routers: []config.ConfigRouter{
						{
							RouterName: "reframe1",
						},
						{
							RouterName: "reframe2",
						},
					},
				},
			},
		},
		"composable2": config.RouterParser{
			Router: config.Router{
				Type: config.RouterTypeParallel,
				Parameters: &config.ComposableRouterParams{
					Routers: []config.ConfigRouter{
						{
							RouterName: "composable1",
						},
						{
							RouterName: "reframe3",
						},
					},
				},
			},
		},
	}, config.Methods{
		config.MethodNameFindPeers: config.Method{
			RouterName: "composable2",
		},
		config.MethodNameFindProviders: config.Method{
			RouterName: "composable2",
		},
		config.MethodNameGetIPNS: config.Method{
			RouterName: "composable2",
		},
		config.MethodNamePutIPNS: config.Method{
			RouterName: "composable2",
		},
		config.MethodNameProvide: config.Method{
			RouterName: "composable2",
		},
	}, &ExtraDHTParams{}, nil)

	require.NoError(err)

	_, ok := router.(*Composer)
	require.True(ok)

}

func TestParserRecursiveLoop(t *testing.T) {
	require := require.New(t)

	_, err := Parse(config.Routers{
		"composable1": config.RouterParser{
			Router: config.Router{
				Type: config.RouterTypeSequential,
				Parameters: &config.ComposableRouterParams{
					Routers: []config.ConfigRouter{
						{
							RouterName: "composable2",
						},
					},
				},
			},
		},
		"composable2": config.RouterParser{
			Router: config.Router{
				Type: config.RouterTypeParallel,
				Parameters: &config.ComposableRouterParams{
					Routers: []config.ConfigRouter{
						{
							RouterName: "composable1",
						},
					},
				},
			},
		},
	}, config.Methods{
		config.MethodNameFindPeers: config.Method{
			RouterName: "composable2",
		},
		config.MethodNameFindProviders: config.Method{
			RouterName: "composable2",
		},
		config.MethodNameGetIPNS: config.Method{
			RouterName: "composable2",
		},
		config.MethodNamePutIPNS: config.Method{
			RouterName: "composable2",
		},
		config.MethodNameProvide: config.Method{
			RouterName: "composable2",
		},
	}, &ExtraDHTParams{}, nil)

	require.ErrorContains(err, "dependency loop creating router with name \"composable2\"")
}
