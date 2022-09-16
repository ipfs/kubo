package routing

import (
	"encoding/base64"
	"testing"

	"github.com/ipfs/kubo/config"
	crypto "github.com/libp2p/go-libp2p/core/crypto"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

func TestRoutingFromConfig(t *testing.T) {
	require := require.New(t)

	r, err := routingFromConfig(config.Router{
		Type: "unknown",
	}, nil, nil)

	require.Nil(r)
	require.EqualError(err, "unknown router type unknown")

	r, err = routingFromConfig(config.Router{
		Type:       config.RouterTypeReframe,
		Parameters: &config.ReframeRouterParams{},
	}, nil, nil)

	require.Nil(r)
	require.EqualError(err, "configuration param 'Endpoint' is needed for reframe delegated routing types")

	r, err = routingFromConfig(config.Router{
		Type: config.RouterTypeReframe,
		Parameters: &config.ReframeRouterParams{
			Endpoint: "test",
		},
	}, nil, nil)

	require.NoError(err)
	require.NotNil(r)

	priv, pub, err := crypto.GenerateKeyPair(crypto.RSA, 2048)
	require.NoError(err)

	id, err := peer.IDFromPublicKey(pub)
	require.NoError(err)

	privM, err := crypto.MarshalPrivateKey(priv)
	require.NoError(err)

	r, err = routingFromConfig(config.Router{
		Type: config.RouterTypeReframe,
		Parameters: &config.ReframeRouterParams{
			Endpoint: "test",
		},
	}, nil, &ExtraReframeParams{
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
	}, &ExtraDHTParams{}, nil)

	require.NoError(err)

	comp, ok := router.(*Composer)
	require.True(ok)

	require.Equal(comp.FindPeersRouter, comp.FindProvidersRouter)
	require.Equal(comp.ProvideRouter, comp.PutValueRouter)
}
