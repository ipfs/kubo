package config

const (
	ApiTag             = "API"
	HTTPAuthSecretsTag = "HTTPAuthSecrets"
)

type API struct {
	// HTTPHeaders are the HTTP headers to return with the API.
	HTTPHeaders map[string][]string

	// HTTPAuthSecrets are the secrets used to authenticate in the API.
	// A secret is in the format "type:value". Check the documentation for
	// supported types.
	HTTPAuthSecrets map[string]string `json:",omitempty"`
}
