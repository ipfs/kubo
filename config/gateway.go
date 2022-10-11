package config

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
}

// Gateway contains options for the HTTP gateway server.
type Gateway struct {

	// HTTPHeaders configures the headers that should be returned by this
	// gateway.
	HTTPHeaders map[string][]string // HTTP headers to return with the gateway

	// RootRedirect is the path to which requests to `/` on this gateway
	// should be redirected.
	RootRedirect string

	// Writable enables PUT/POST request handling by this gateway. Usually,
	// writing is done through the API, not the gateway.
	Writable bool

	// PathPrefixes was removed: https://github.com/ipfs/go-ipfs/issues/7702
	PathPrefixes []string

	// FastDirIndexThreshold is the maximum number of items in a directory
	// before the Gateway switches to a shallow, faster listing which only
	// requires the root node. This allows for listing big directories fast,
	// without the linear slowdown caused by reading size metadata from child
	// nodes.
	// Setting to 0 will enable fast listings for all directories.
	FastDirIndexThreshold *OptionalInteger `json:",omitempty"`

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
