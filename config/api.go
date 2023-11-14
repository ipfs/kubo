package config

const (
	APITag           = "API"
	AuthorizationTag = "Authorizations"
)

type RPCAuthScope struct {
	// HTTPAuthSecret is the secret that will be compared to the HTTP "Authorization".
	// A secret is in the format "type:value". Check the documentation for
	// supported types.
	HTTPAuthSecret string

	// AllowedPaths is an explicit list of RPC path prefixes to allow.
	// By default, none are allowed. ["/api/v0"] exposes all RPCs.
	AllowedPaths []string
}

type API struct {
	// HTTPHeaders are the HTTP headers to return with the API.
	HTTPHeaders map[string][]string

	// Authorization is a map of authorizations used to authenticate in the API.
	// If the map is empty, then the RPC API is exposed to everyone. Check the
	// documentation for more details.
	Authorizations map[string]*RPCAuthScope `json:",omitempty"`
}
