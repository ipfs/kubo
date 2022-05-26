package config

// Routing defines configuration options for libp2p routing
type Routing struct {
	// Type sets default daemon routing mode.
	//
	// Can be one of "dht", "dhtclient", "dhtserver", "none", or unset.
	Type *OptionalString `json:",omitempty"`

	Routers map[string]Router
}

type Router struct {

	// Type can be one of "dht", "dhtclient", "dhtserver", "reframe".
	// Reframe type allows to add other resolvers using the Reframe spec:
	// https://github.com/ipfs/specs/blob/master/REFRAME.md
	Type    string
	Enabled bool

	// Methods that we want to use from this provider.
	// Leave it empty to use all the supported and available ones.
	// Actual supported methods: "FindProviders", "GetIPNS", "PutIPNS"
	Methods []string

	// Parameters are extra configuration that this router might need.
	// A common one for reframe endpoints is "Address".
	Parameters map[string]string
}

// Type is the routing type.
// Depending of the type we need to instantiate different Routing implementations.
type RouterType string

const (
	RouterTypeReframe RouterType = "reframe"
	RouterTypeDHT     RouterType = "dht"
)

type RouterParam string

const (
	// RouterParamAddress is the URL where the routing implementation will point to get the information.
	// Usually used for reframe Routers.
	RouterParamAddress RouterParam = "address"

	RouterParamPriority RouterParam = "priority"
)
