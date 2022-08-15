package config

// Routing defines configuration options for libp2p routing
type Routing struct {
	Routers map[string]Router
}

type Router struct {

	// Currenly only supported Types are "reframe", "dht" and "none".
	// Reframe type allows to add other resolvers using the Reframe spec:
	// https://github.com/ipfs/specs/tree/main/reframe
	// In the future we will support "dht" and other Types here.
	Type RouterType

	Enabled Flag `json:",omitempty"`

	// Parameters are extra configuration that this router might need.
	// A common one for reframe router is "Endpoint".
	Parameters RouterParams
}

type RouterParams map[RouterParam]interface{}

func (rp RouterParams) String(key RouterParam) (string, bool) {
	out, ok := rp[key].(string)
	return out, ok
}

func (rp RouterParams) Number(key RouterParam) (int, bool) {
	out, ok := rp[key].(int)
	return out, ok
}

func (rp RouterParams) StringSlice(key RouterParam) ([]string, bool) {
	out, ok := rp[key].([]string)
	return out, ok
}

func (rp RouterParams) Bool(key RouterParam) (bool, bool) {
	out, ok := rp[key].(bool)
	return out, ok
}

type RouterType string

// Type is the routing type.
// Depending of the type we need to instantiate different Routing implementations.
const (
	RouterTypeReframe RouterType = "reframe"
	RouterTypeDHT     RouterType = "dht"
	RouterTypeNone    RouterType = "none"
)

type RouterParam string

const (
	// RouterParamEndpoint is the URL where the routing implementation will point to get the information.
	// Usually used for reframe Routers.
	RouterParamEndpoint            RouterParam = "Endpoint"
	RouterParamPriority            RouterParam = "Priority"
	RouterParamDHTType             RouterParam = "Mode"
	RouterParamTrackFullNetworkDHT RouterParam = "TrackFullNetworkDHT"
	RouterParamPublicIPNetwork     RouterParam = "Public-IP-Network"
)

const (
	RouterValueDHTTypeServer = "server"
	RouterValueDHTTypeClient = "client"
	RouterValueDHTTypeAuto   = "auto"
)

var RouterValueDHTTypes = []string{string(RouterValueDHTTypeAuto), string(RouterValueDHTTypeServer), string(RouterValueDHTTypeClient)}
