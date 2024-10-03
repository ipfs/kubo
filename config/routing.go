package config

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
)

const (
	DefaultAcceleratedDHTClient      = false
	DefaultLoopbackAddressesOnLanDHT = false
)

var (
	// Default HTTP routers used in parallel to DHT when Routing.Type = "auto"
	DefaultHTTPRouters = getEnvOrDefault("IPFS_HTTP_ROUTERS", []string{
		"https://cid.contact", // https://github.com/ipfs/kubo/issues/9422#issuecomment-1338142084
	})

	// Default filter-protocols to pass along with delegated routing requests (as defined in IPIP-484)
	// and also filter out locally
	DefaultHTTPRoutersFilterProtocols = getEnvOrDefault("IPFS_HTTP_ROUTERS_FILTER_PROTOCOLS", []string{
		"unknown", // allow results without protocol list, we can do libp2p identify to test them
		"transport-bitswap",
		// TODO: add 'transport-ipfs-gateway-http' once https://github.com/ipfs/rainbow/issues/125 is addressed
	})
)

// Routing defines configuration options for libp2p routing.
type Routing struct {
	// Type sets default daemon routing mode.
	//
	// Can be one of "auto", "autoclient", "dht", "dhtclient", "dhtserver", "none", or "custom".
	// When unset or set to "auto", DHT and implicit routers are used.
	// When "custom" is set, user-provided Routing.Routers is used.
	Type *OptionalString `json:",omitempty"`

	AcceleratedDHTClient Flag `json:",omitempty"`

	LoopbackAddressesOnLanDHT Flag `json:",omitempty"`

	Routers Routers

	Methods Methods
}

type Router struct {
	// Router type ID. See RouterType for more info.
	Type RouterType

	// Parameters are extra configuration that this router might need.
	// A common one for HTTP router is "Endpoint".
	Parameters interface{}
}

type (
	Routers map[string]RouterParser
	Methods map[MethodName]Method
)

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
	RouterTypeHTTP       RouterType = "http"       // HTTP JSON API for delegated routing systems (IPIP-337).
	RouterTypeDHT        RouterType = "dht"        // DHT router.
	RouterTypeSequential RouterType = "sequential" // Router helper to execute several routers sequentially.
	RouterTypeParallel   RouterType = "parallel"   // Router helper to execute several routers in parallel.
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

// getEnvOrDefault reads space or comma separated strings from env if present,
// and uses provided defaultValue as a fallback
func getEnvOrDefault(key string, defaultValue []string) []string {
	if value, exists := os.LookupEnv(key); exists {
		splitFunc := func(r rune) bool { return r == ',' || r == ' ' }
		return strings.FieldsFunc(value, splitFunc)
	}
	return defaultValue
}
