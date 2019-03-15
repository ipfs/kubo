package config

type GatewaySpec struct {
	// Paths is explicit list of path prefixes that should be handled by
	// this gateway. Example: `["/ipfs", "/ipns", "/api"]`
	Paths []string

	// UseSubdomains indicates whether or not this gateway uses subdomains
	// for IPFS resources instead of paths. That is: http://CID.ipfs.GATEWAY/...
	//
	// If this flag is set, any /ipns/$id and/or /ipfs/$id paths in PathPrefixes
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

	// PathPrefixes  is an array of acceptable url paths that a client can
	// specify in X-Ipfs-Path-Prefix header.
	//
	// The X-Ipfs-Path-Prefix header is used to specify a base path to prepend
	// to links in directory listings and for trailing-slash redirects. It is
	// intended to be set by a frontend http proxy like nginx.
	//
	// Example: To mount blog.ipfs.io (a DNSLink site) at ipfs.io/blog
	// set PathPrefixes to ["/blog"] and nginx config to translate paths
	// and pass Host header (for DNSLink):
	//  location /blog/ {
	//    rewrite "^/blog(/.*)$" $1 break;
	//    proxy_set_header Host blog.ipfs.io;
	//    proxy_set_header X-Ipfs-Gateway-Prefix /blog;
	//    proxy_pass http://127.0.0.1:8080;
	//  }
	PathPrefixes []string

	// FIXME: Not yet implemented
	APICommands []string

	// NoFetch configures the gateway to _not_ fetch blocks in response to
	// requests.
	NoFetch bool

	// NoDNSLink configures the gateway to _not_ perform DNS TXT record
	// lookups in response to requests with values in `Host` HTTP header.
	// This flag can be overriden per FQDN in PublicGateways.
	NoDNSLink bool

	// PublicGateways configures behavior of known public gateways.
	// Each key is a fully qualified domain name (FQDN).
	PublicGateways map[string]*GatewaySpec
}
