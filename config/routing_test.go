package config

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRouterParameters(t *testing.T) {
	require := require.New(t)
	sec := time.Second
	min := time.Minute
	r := Routing{
		Type: NewOptionalString("custom"),
		Routers: map[string]RouterParser{
			"router-dht": {Router{
				Type: RouterTypeDHT,
				Parameters: DHTRouterParams{
					Mode:                 "auto",
					AcceleratedDHTClient: true,
					PublicIPNetwork:      false,
				},
			}},
			"router-parallel": {
				Router{
					Type: RouterTypeParallel,
					Parameters: ComposableRouterParams{
						Routers: []ConfigRouter{
							{
								RouterName:   "router-dht",
								Timeout:      Duration{10 * time.Second},
								IgnoreErrors: true,
							},
							{
								RouterName:   "router-dht",
								Timeout:      Duration{10 * time.Second},
								IgnoreErrors: false,
								ExecuteAfter: &OptionalDuration{&sec},
							},
						},
						Timeout: &OptionalDuration{&min},
					},
				},
			},
			"router-sequential": {
				Router{
					Type: RouterTypeSequential,
					Parameters: ComposableRouterParams{
						Routers: []ConfigRouter{
							{
								RouterName:   "router-dht",
								Timeout:      Duration{10 * time.Second},
								IgnoreErrors: true,
							},
							{
								RouterName:   "router-dht",
								Timeout:      Duration{10 * time.Second},
								IgnoreErrors: false,
							},
						},
						Timeout: &OptionalDuration{&min},
					},
				},
			},
		},
		Methods: Methods{
			MethodNameFindPeers: {
				RouterName: "router-dht",
			},
			MethodNameFindProviders: {
				RouterName: "router-dht",
			},
			MethodNameGetIPNS: {
				RouterName: "router-sequential",
			},
			MethodNameProvide: {
				RouterName: "router-parallel",
			},
			MethodNamePutIPNS: {
				RouterName: "router-parallel",
			},
		},
	}

	out, err := json.Marshal(r)
	require.NoError(err)

	r2 := &Routing{}

	err = json.Unmarshal(out, r2)
	require.NoError(err)

	require.Equal(5, len(r2.Methods))

	dhtp := r2.Routers["router-dht"].Parameters
	require.IsType(&DHTRouterParams{}, dhtp)

	sp := r2.Routers["router-sequential"].Parameters
	require.IsType(&ComposableRouterParams{}, sp)

	pp := r2.Routers["router-parallel"].Parameters
	require.IsType(&ComposableRouterParams{}, pp)
}

func TestMethods(t *testing.T) {
	require := require.New(t)

	methodsOK := Methods{
		MethodNameFindPeers: {
			RouterName: "router-wrong",
		},
		MethodNameFindProviders: {
			RouterName: "router-wrong",
		},
		MethodNameGetIPNS: {
			RouterName: "router-wrong",
		},
		MethodNameProvide: {
			RouterName: "router-wrong",
		},
		MethodNamePutIPNS: {
			RouterName: "router-wrong",
		},
	}

	require.NoError(methodsOK.Check())

	methodsMissing := Methods{
		MethodNameFindPeers: {
			RouterName: "router-wrong",
		},
		MethodNameGetIPNS: {
			RouterName: "router-wrong",
		},
		MethodNameProvide: {
			RouterName: "router-wrong",
		},
		MethodNamePutIPNS: {
			RouterName: "router-wrong",
		},
	}

	require.Error(methodsMissing.Check())
}
