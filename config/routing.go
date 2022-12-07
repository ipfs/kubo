package config

import (
	"encoding/json"
	"fmt"
	"runtime"
)

// Routing defines configuration options for libp2p routing
type Routing struct {
	// Type sets default daemon routing mode.
	//
	// Can be one of "auto", "dht", "dhtclient", "dhtserver", "none", or "custom".
	// When unset or set to "auto", DHT and implicit routers are used.
	// When "custom" is set, user-provided Routing.Routers is used.
	Type *OptionalString `json:",omitempty"`

	Routers Routers

	Methods Methods
}

type Router struct {

	// Currenly supported Types are "reframe", "dht", "parallel", "sequential".
	// Reframe type allows to add other resolvers using the Reframe spec:
	// https://github.com/ipfs/specs/tree/main/reframe
	// In the future we will support "dht" and other Types here.
	Type RouterType

	// Parameters are extra configuration that this router might need.
	// A common one for reframe router is "Endpoint".
	Parameters interface{}
}

type Routers map[string]RouterParser
type Methods map[MethodName]Method

func (m Methods) Check() error {

	// Check supported methods
	for _, mn := range MethodNameList {
		_, ok := m[mn]
		if !ok {
			return fmt.Errorf("method name %q is missing from Routing.Methods config param", mn)
		}
	}

	// Check unsupported methods
	for k := range m {
		seen := false
		for _, mn := range MethodNameList {
			if mn == k {
				seen = true
				break
			}
		}

		if seen {
			continue
		}

		return fmt.Errorf("method name %q is not a supported method on Routing.Methods config param", k)
	}

	return nil
}

type RouterParser struct {
	Router
}

func (r *RouterParser) UnmarshalJSON(b []byte) error {
	out := Router{}
	out.Parameters = &json.RawMessage{}
	if err := json.Unmarshal(b, &out); err != nil {
		return err
	}
	raw := out.Parameters.(*json.RawMessage)

	var p interface{}
	switch out.Type {
	case RouterTypeHTTP:
		p = &HTTPRouterParams{}
	case RouterTypeReframe:
		p = &ReframeRouterParams{}
	case RouterTypeDHT:
		p = &DHTRouterParams{}
	case RouterTypeSequential:
		p = &ComposableRouterParams{}
	case RouterTypeParallel:
		p = &ComposableRouterParams{}
	}

	if err := json.Unmarshal(*raw, &p); err != nil {
		return err
	}

	r.Router.Type = out.Type
	r.Router.Parameters = p

	return nil
}

// Type is the routing type.
// Depending of the type we need to instantiate different Routing implementations.
type RouterType string

const (
	RouterTypeReframe    RouterType = "reframe"
	RouterTypeHTTP       RouterType = "http"
	RouterTypeDHT        RouterType = "dht"
	RouterTypeSequential RouterType = "sequential"
	RouterTypeParallel   RouterType = "parallel"
)

type DHTMode string

const (
	DHTModeServer DHTMode = "server"
	DHTModeClient DHTMode = "client"
	DHTModeAuto   DHTMode = "auto"
)

type MethodName string

const (
	MethodNameProvide       MethodName = "provide"
	MethodNameFindProviders MethodName = "find-providers"
	MethodNameFindPeers     MethodName = "find-peers"
	MethodNameGetIPNS       MethodName = "get-ipns"
	MethodNamePutIPNS       MethodName = "put-ipns"
)

var MethodNameList = []MethodName{MethodNameProvide, MethodNameFindPeers, MethodNameFindProviders, MethodNameGetIPNS, MethodNamePutIPNS}

type ReframeRouterParams struct {
	// Endpoint is the URL where the routing implementation will point to get the information.
	// Usually used for reframe Routers.
	Endpoint string
}

type HTTPRouterParams struct {
	// Endpoint is the URL where the routing implementation will point to get the information.
	Endpoint string

	// MaxProvideBatchSize determines the maximum amount of CIDs sent per batch.
	// Servers might not accept more than 100 elements per batch. 100 elements by default.
	MaxProvideBatchSize int

	// MaxProvideConcurrency determines the number of threads used when providing content. GOMAXPROCS by default.
	MaxProvideConcurrency int
}

func (hrp *HTTPRouterParams) FillDefaults() {
	if hrp.MaxProvideBatchSize == 0 {
		hrp.MaxProvideBatchSize = 100
	}

	if hrp.MaxProvideConcurrency == 0 {
		hrp.MaxProvideConcurrency = runtime.GOMAXPROCS(0)
	}
}

type DHTRouterParams struct {
	Mode                 DHTMode
	AcceleratedDHTClient bool `json:",omitempty"`
	PublicIPNetwork      bool
}

type ComposableRouterParams struct {
	Routers []ConfigRouter
	Timeout *OptionalDuration `json:",omitempty"`
}

type ConfigRouter struct {
	RouterName   string
	Timeout      Duration
	IgnoreErrors bool
	ExecuteAfter *OptionalDuration `json:",omitempty"`
}

type Method struct {
	RouterName string
}
