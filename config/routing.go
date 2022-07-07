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

	// Currenly only supported Type is "reframe".
	// Reframe type allows to add other resolvers using the Reframe spec:
	// https://github.com/ipfs/specs/tree/main/reframe
	// In the future we will support "dht" and other Types here.
	Type string

	Enabled Flag `json:",omitempty"`

	// Parameters are extra configuration that this router might need.
	// A common one for reframe router is "Endpoint".
	Parameters map[string]string
}

// Type is the routing type.
// Depending of the type we need to instantiate different Routing implementations.
type RouterType string

const (
	RouterTypeReframe RouterType = "reframe"
)

type RouterParam string

const (
	// RouterParamEndpoint is the URL where the routing implementation will point to get the information.
	// Usually used for reframe Routers.
	RouterParamEndpoint RouterParam = "Endpoint"

	RouterParamPriority RouterParam = "Priority"
)
