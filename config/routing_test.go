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
		Type: "custom",
		Routers: map[string]RouterParser{
			"router-dht": {Router{
				Type:    RouterTypeDHT,
				Enabled: True,
				Parameters: DHTRouterParams{
					Mode:                 "auto",
					AcceleratedDHTClient: true,
					PublicIPNetwork:      false,
				},
			}},
			"router-reframe": {Router{
				Type:    RouterTypeReframe,
				Enabled: True,
				Parameters: ReframeRouterParams{
					Endpoint: "reframe-endpoint",
				},
			}},
			"router-parallel": {Router{
				Type:    RouterTypeParallel,
				Enabled: True,
				Parameters: ComposableRouterParams{
					Routers: []ConfigRouter{
						{
							RouterName:   "router-dht",
							Timeout:      Duration{10 * time.Second},
							IgnoreErrors: true,
						},
						{
							RouterName:   "router-reframe",
							Timeout:      Duration{10 * time.Second},
							IgnoreErrors: false,
							ExecuteAfter: &OptionalDuration{&sec},
						},
					},
					Timeout: &OptionalDuration{&min},
				}},
			},
			"router-sequential": {Router{
				Type:    RouterTypeSequential,
				Enabled: True,
				Parameters: ComposableRouterParams{
					Routers: []ConfigRouter{
						{
							RouterName:   "router-dht",
							Timeout:      Duration{10 * time.Second},
							IgnoreErrors: true,
						},
						{
							RouterName:   "router-reframe",
							Timeout:      Duration{10 * time.Second},
							IgnoreErrors: false,
						},
					},
					Timeout: &OptionalDuration{&min},
				}},
			},
		},
		Methods: map[MethodName]Method{
			MethodNameFindPeers: {
				RouterName: "router-reframe",
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

	rp := r2.Routers["router-reframe"].Parameters
	require.IsType(&ReframeRouterParams{}, rp)

	sp := r2.Routers["router-sequential"].Parameters
	require.IsType(&ComposableRouterParams{}, sp)

	pp := r2.Routers["router-parallel"].Parameters
	require.IsType(&ComposableRouterParams{}, pp)
}

func TestRouterMissingParameters(t *testing.T) {
	require := require.New(t)

	r := Routing{
		Type: "custom",
		Routers: map[string]RouterParser{
			"router-wrong-reframe": {Router{
				Type:    RouterTypeReframe,
				Enabled: True,
				Parameters: DHTRouterParams{
					Mode:                 "auto",
					AcceleratedDHTClient: true,
					PublicIPNetwork:      false,
				},
			}},
		},
		Methods: map[MethodName]Method{
			MethodNameFindPeers: {
				RouterName: "router-wrong-reframe",
			},
			MethodNameFindProviders: {
				RouterName: "router-wrong-reframe",
			},
			MethodNameGetIPNS: {
				RouterName: "router-wrong-reframe",
			},
			MethodNameProvide: {
				RouterName: "router-wrong-reframe",
			},
			MethodNamePutIPNS: {
				RouterName: "router-wrong-reframe",
			},
		},
	}

	out, err := json.Marshal(r)
	require.NoError(err)

	r2 := &Routing{}

	err = json.Unmarshal(out, r2)
	require.NoError(err)
	require.Empty(r2.Routers["router-wrong-reframe"].Parameters.(*ReframeRouterParams).Endpoint)
}
