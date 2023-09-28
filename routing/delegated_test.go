package routing

import (
	"crypto/rand"
	"encoding/base64"
	"testing"

	"github.com/ipfs/kubo/config"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

func TestParser(t *testing.T) {
	require := require.New(t)

	pid, sk, err := generatePeerID()
	require.NoError(err)

	router, err := Parse(config.Routers{
		"r1": config.RouterParser{
			Router: config.Router{
				Type: config.RouterTypeHTTP,
				Parameters: &config.HTTPRouterParams{
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
	}, &ExtraDHTParams{}, &ExtraHTTPParams{
		PeerID:     string(pid),
		PrivKeyB64: sk,
	})

	require.NoError(err)

	comp, ok := router.(*Composer)
	require.True(ok)

	require.Equal(comp.FindPeersRouter, comp.FindProvidersRouter)
	require.Equal(comp.ProvideRouter, comp.PutValueRouter)
}

func TestParserRecursive(t *testing.T) {
	require := require.New(t)

	pid, sk, err := generatePeerID()
	require.NoError(err)

	router, err := Parse(config.Routers{
		"http1": config.RouterParser{
			Router: config.Router{
				Type: config.RouterTypeHTTP,
				Parameters: &config.HTTPRouterParams{
					Endpoint: "testEndpoint1",
				},
			},
		},
		"http2": config.RouterParser{
			Router: config.Router{
				Type: config.RouterTypeHTTP,
				Parameters: &config.HTTPRouterParams{
					Endpoint: "testEndpoint2",
				},
			},
		},
		"http3": config.RouterParser{
			Router: config.Router{
				Type: config.RouterTypeHTTP,
				Parameters: &config.HTTPRouterParams{
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
							RouterName: "http1",
						},
						{
							RouterName: "http2",
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
							RouterName: "http3",
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
	}, &ExtraDHTParams{}, &ExtraHTTPParams{
		PeerID:     string(pid),
		PrivKeyB64: sk,
	})

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

func generatePeerID() (string, string, error) {
	sk, pk, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return "", "", err
	}

	bytes, err := crypto.MarshalPrivateKey(sk)
	if err != nil {
		return "", "", err
	}

	enc := base64.StdEncoding.EncodeToString(bytes)
	if err != nil {
		return "", "", err
	}

	pid, err := peer.IDFromPublicKey(pk)
	return pid.String(), enc, err
}
