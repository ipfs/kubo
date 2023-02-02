package config

const DefaultInlineDNSLink = false

type GatewaySpec struct {
	// Paths is explicit list of path prefixes that should be handled by
	// this gateway. Example: `["/ipfs", "/ipns", "/api"]`
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
}

// Gateway contains options for the HTTP gateway server.
type Gateway struct {

	// HTTPHeaders configures the headers that should be returned by this
	// gateway.
	HTTPHeaders map[string][]string // HTTP headers to return with the gateway

	// RootRedirect is the path to which requests to `/` on this gateway
	// should be redirected.
	RootRedirect string

	// DEPRECATED: Enables legacy PUT/POST request handling.
	// Modern replacement tracked in https://github.com/ipfs/specs/issues/375
	Writable Flag `json:",omitempty"`

	// PathPrefixes was removed: https://github.com/ipfs/go-ipfs/issues/7702
	PathPrefixes []string

	// FIXME: Not yet implemented: https://github.com/ipfs/kubo/issues/8059
	APICommands []string

	// NoFetch configures the gateway to _not_ fetch blocks in response to
	// requests.
	NoFetch bool

	// NoDNSLink configures the gateway to _not_ perform DNS TXT record
	// lookups in response to requests with values in `Host` HTTP header.
	// This flag can be overridden per FQDN in PublicGateways.
	NoDNSLink bool

	// PublicGateways configures behavior of known public gateways.
	// Each key is a fully qualified domain name (FQDN).
	PublicGateways map[string]*GatewaySpec
}
