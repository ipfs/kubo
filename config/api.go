package config

import (
	"encoding/base64"
	"strings"
)

const (
	APITag           = "API"
	AuthorizationTag = "Authorizations"
)

type RPCAuthScope struct {
	// AuthSecret is the secret that will be compared to the HTTP "Authorization".
	// header. A secret is in the format "type:value". Check the documentation for
	// supported types.
	AuthSecret string

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

// ConvertAuthSecret converts the given secret in the format "type:value" into an
// HTTP Authorization header value. It can handle 'bearer' and 'basic' as type.
// If type exists and is not known, an empty string is returned. If type does not
// exist, 'bearer' type is assumed.
func ConvertAuthSecret(secret string) string {
	if secret == "" {
		return secret
	}

	split := strings.SplitN(secret, ":", 2)
	if len(split) < 2 {
		// No prefix: assume bearer token.
		return "Bearer " + secret
	}

	if strings.HasPrefix(secret, "basic:") {
		if strings.Contains(split[1], ":") {
			// Assume basic:user:password
			return "Basic " + base64.StdEncoding.EncodeToString([]byte(split[1]))
		} else {
			// Assume already base64 encoded.
			return "Basic " + split[1]
		}
	} else if strings.HasPrefix(secret, "bearer:") {
		return "Bearer " + split[1]
	}

	// Unknown. Type is present, but we can't handle it.
	return ""
}
