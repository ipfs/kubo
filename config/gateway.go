package config

import (
	"github.com/ipfs/boxo/gateway"
)

const (
	DefaultInlineDNSLink         = false
	DefaultDeserializedResponses = true
	DefaultDisableHTMLErrors     = false
	DefaultExposeRoutingAPI      = true
	DefaultDiagnosticServiceURL  = "https://check.ipfs.network"

	// Gateway limit defaults from boxo
	DefaultRetrievalTimeout        = gateway.DefaultRetrievalTimeout
	DefaultMaxConcurrentRequests   = gateway.DefaultMaxConcurrentRequests
	DefaultMaxRangeRequestFileSize = 0 // 0 means no limit
)

type GatewaySpec struct {
	// Paths is explicit list of path prefixes that should be handled by
	// this gateway. Example: `["/ipfs", "/ipns"]`
	Paths []string

	// UseSubdomains indicates whether or not this gateway uses subdomains
	// for IPFS resources instead of paths. That is: http://CID.ipfs.GATEWAY/...
	//
	// If this flag is set, any /ipns/$id and/or /ipfs/$id paths in Paths
	// will be permanently redirected to http://$id.[ipns|ipfs].$gateway/.
	//
	// We do not support using both paths and subdomains for a single domain
	// for security reasons (Origin isolation).
	UseSubdomains bool

	// NoDNSLink configures this gateway to _not_ resolve DNSLink for the FQDN
	// provided in `Host` HTTP header.
	NoDNSLink bool

	// InlineDNSLink configures this gateway to always inline DNSLink names
	// (FQDN) into a single DNS label in order to interop with wildcard TLS certs
	// and Origin per CID isolation provided by rules like https://publicsuffix.org
	InlineDNSLink Flag

	// DeserializedResponses configures this gateway to respond to deserialized
	// responses. Disabling this option enables a Trustless Gateway, as per:
	// https://specs.ipfs.tech/http-gateways/trustless-gateway/.
	DeserializedResponses Flag
}

// Gateway contains options for the HTTP gateway server.
type Gateway struct {
	// HTTPHeaders configures the headers that should be returned by this
	// gateway.
	HTTPHeaders map[string][]string // HTTP headers to return with the gateway

	// RootRedirect is the path to which requests to `/` on this gateway
	// should be redirected.
	RootRedirect string

	// NoFetch configures the gateway to _not_ fetch blocks in response to
	// requests.
	NoFetch bool

	// NoDNSLink configures the gateway to _not_ perform DNS TXT record
	// lookups in response to requests with values in `Host` HTTP header.
	// This flag can be overridden per FQDN in PublicGateways.
	NoDNSLink bool

	// DeserializedResponses configures this gateway to respond to deserialized
	// requests. Disabling this option enables a Trustless only gateway, as per:
	// https://specs.ipfs.tech/http-gateways/trustless-gateway/. This can
	// be overridden per FQDN in PublicGateways.
	DeserializedResponses Flag

	// DisableHTMLErrors disables pretty HTML pages when an error occurs. Instead, a `text/plain`
	// page will be sent with the raw error message.
	DisableHTMLErrors Flag

	// PublicGateways configures behavior of known public gateways.
	// Each key is a fully qualified domain name (FQDN).
	PublicGateways map[string]*GatewaySpec

	// ExposeRoutingAPI configures the gateway port to expose
	// routing system as HTTP API at /routing/v1 (https://specs.ipfs.tech/routing/http-routing-v1/).
	ExposeRoutingAPI Flag

	// RetrievalTimeout enforces a maximum duration for content retrieval:
	// - Time to first byte: If the gateway cannot start writing the response within
	//   this duration (e.g., stuck searching for providers), a 504 Gateway Timeout
	//   is returned.
	// - Time between writes: After the first byte, the timeout resets each time new
	//   bytes are written to the client. If the gateway cannot write additional data
	//   within this duration after the last successful write, the response is terminated.
	// This helps free resources when the gateway gets stuck looking for providers
	// or cannot retrieve the requested content.
	// A value of 0 disables this timeout.
	RetrievalTimeout *OptionalDuration `json:",omitempty"`

	// MaxConcurrentRequests limits concurrent HTTP requests handled by the gateway.
	// Requests beyond this limit receive 429 Too Many Requests with Retry-After header.
	// A value of 0 disables the limit.
	MaxConcurrentRequests *OptionalInteger `json:",omitempty"`

	// MaxRangeRequestFileSize limits the maximum file size for HTTP range requests.
	// Range requests for files larger than this limit return 501 Not Implemented.
	// This protects against CDN issues with large file range requests and prevents
	// excessive bandwidth consumption. A value of 0 disables the limit.
	MaxRangeRequestFileSize *OptionalBytes `json:",omitempty"`

	// DiagnosticServiceURL is the URL for a service to diagnose CID retrievability issues.
	// When the gateway returns a 504 Gateway Timeout error, an "Inspect retrievability of CID"
	// button will be shown that links to this service with the CID appended as ?cid=<CID-to-diagnose>.
	// Set to empty string to disable the button.
	DiagnosticServiceURL *OptionalString `json:",omitempty"`
}
