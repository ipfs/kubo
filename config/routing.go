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
	DefaultRoutingType               = "auto"
	CidContactRoutingURL             = "https://cid.contact"
	PublicGoodDelegatedRoutingURL    = "https://delegated-ipfs.dev" // cid.contact + amino dht (incl. IPNS PUTs)
	EnvHTTPRouters                   = "IPFS_HTTP_ROUTERS"
	EnvHTTPRoutersFilterProtocols    = "IPFS_HTTP_ROUTERS_FILTER_PROTOCOLS"
)

var (
	// Default filter-protocols to pass along with delegated routing requests (as defined in IPIP-484)
	// and also filter out locally
	DefaultHTTPRoutersFilterProtocols = getEnvOrDefault(EnvHTTPRoutersFilterProtocols, []string{
		"unknown", // allow results without protocol list, we can do libp2p identify to test them
		"transport-bitswap",
		// http is added dynamically in routing/delegated.go.
		// 'transport-ipfs-gateway-http'
	})
)

// Routing defines configuration options for libp2p routing.
type Routing struct {
	// Type sets default daemon routing mode.
	//
	// Can be one of "auto", "autoclient", "dht", "dhtclient", "dhtserver", "none", "delegated", or "custom".
	// When unset or set to "auto", DHT and implicit routers are used.
	// When "delegated" is set, only HTTP delegated routers and IPNS publishers are used (no DHT).
	// When "custom" is set, user-provided Routing.Routers is used.
	Type *OptionalString `json:",omitempty"`

	AcceleratedDHTClient Flag `json:",omitempty"`

	LoopbackAddressesOnLanDHT Flag `json:",omitempty"`

	IgnoreProviders []string `json:",omitempty"`

	// Simplified configuration used by default when Routing.Type=auto|autoclient
	DelegatedRouters []string

	// Advanced configuration used when Routing.Type=custom
	Routers Routers `json:",omitempty"`
	Methods Methods `json:",omitempty"`
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

// HasHTTPProviderConfigured checks if the node is configured to use HTTP routers
// for providing content announcements. This is used when determining if the node
// can provide content even when not connected to libp2p peers.
//
// Note: Right now we only support delegated HTTP content providing if Routing.Type=custom
// and Routing.Routers are configured according to:
// https://github.com/ipfs/kubo/blob/master/docs/delegated-routing.md#configuration-file-example
//
// This uses the `ProvideBitswap` request type that is not documented anywhere,
// because we hoped something like IPIP-378 (https://github.com/ipfs/specs/pull/378)
// would get finalized and we'd switch to that. It never happened due to politics,
// and now we are stuck with ProvideBitswap being the only API that works.
// Some people have reverse engineered it (example:
// https://discuss.ipfs.tech/t/only-peers-found-from-dht-seem-to-be-getting-used-as-relays-so-cant-use-http-routers/19545/9)
// and use it, so what we do here is the bare minimum to ensure their use case works
// using this old API until something better is available.
func (c *Config) HasHTTPProviderConfigured() bool {
	if len(c.Routing.Routers) == 0 {
		// No "custom" routers
		return false
	}
	method, ok := c.Routing.Methods[MethodNameProvide]
	if !ok {
		// No provide method configured
		return false
	}
	return c.routerSupportsHTTPProviding(method.RouterName)
}

// routerSupportsHTTPProviding checks if the supplied custom router is or
// includes an HTTP-based router.
func (c *Config) routerSupportsHTTPProviding(routerName string) bool {
	rp, ok := c.Routing.Routers[routerName]
	if !ok {
		// Router configured for providing doesn't exist
		return false
	}

	switch rp.Type {
	case RouterTypeHTTP:
		return true
	case RouterTypeParallel, RouterTypeSequential:
		// Check if any child router supports HTTP
		if children, ok := rp.Parameters.(*ComposableRouterParams); ok {
			for _, childRouter := range children.Routers {
				if c.routerSupportsHTTPProviding(childRouter.RouterName) {
					return true
				}
			}
		}
	}
	return false
}
