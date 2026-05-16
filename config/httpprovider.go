package config

// HTTPProvider configures kubo's role as an HTTP-native source of
// trustless-gateway block retrieval. When Enabled, the local trustless
// gateway handler (NoFetch, raw blocks via ?format=raw only,
// content-addressed verification client-side) is exposed for HTTP
// retrieval clients such as boxo/bitswap/network/httpnet.
//
// This is the server side of the HTTP retrieval story; the client side
// lives separately and is configured under HTTPRetrieval.
type HTTPProvider struct {
	// Enabled is the master switch. Default: false. When true (and other
	// prerequisites are met), the trustless gateway handler is registered
	// and the transport sub-toggles default to true unless explicitly set
	// to false.
	Enabled Flag `json:",omitempty"`

	// Libp2p exposes the trustless gateway over a libp2p stream, as
	// specified by the libp2p Gateway spec
	// (https://specs.ipfs.tech/http-gateways/libp2p-gateway/). The
	// handler is mounted under the /ipfs/gateway protocol ID and
	// advertised via .well-known/libp2p/protocols on the libp2p+HTTP
	// host. Default: true when Enabled is true.
	Libp2p Flag `json:",omitempty"`

	// Cleartext auto-appends a plaintext /ws listener to each /tcp/N
	// already in Addresses.Swarm, unless a cleartext /ws listener is
	// already present. The new /ws shares the existing TCP port via the
	// shared-TCP demuxer, so no extra socket is opened.
	//
	// Intended for deployments where the operator handles TLS termination
	// for /ws and /http themselves, typically a reverse proxy (Caddy,
	// Traefik, nginx, etc.) sitting in front of kubo and forwarding either
	// HTTP/1.1 or HTTP/2 cleartext (h2c) to this node. With AutoTLS, kubo
	// already serves /tls/ws and /tls/http directly with a Let's Encrypt
	// cert, so adding a cleartext path is unnecessary and would expose the
	// trustless gateway and WebSocket upgrade unencrypted on the public
	// network. Off by default for that reason; flip it on knowingly.
	//
	// The corresponding network advertisement (/http multiaddr beside
	// /ws) is controlled by AnnounceMultiaddrs.
	//
	// Default: false.
	Cleartext Flag `json:",omitempty"`

	// AnnounceMultiaddrs derives an HTTP-flavored multiaddr from each of
	// this peer's WebSocket listeners and includes it in the announced
	// address set: /ws -> /http, /tls/ws -> /tls/http,
	// /tls/sni/<host>/ws -> /tls/sni/<host>/http. The HTTP endpoint shares
	// the same TCP port and TLS certificate as the WebSocket listener, so
	// this is purely an announcement (no extra socket is opened). Lets
	// HTTP retrieval clients (e.g. boxo/bitswap/network/httpnet) discover
	// this peer as an HTTP source through identify, the DHT, and IPNI
	// without any out-of-band knowledge.
	//
	// Subject to Addresses.NoAnnounce filters, just like every other
	// announced multiaddr.
	//
	// Default: true when Enabled is true.
	AnnounceMultiaddrs Flag `json:",omitempty"`
}

// HTTPProvider defaults. The master switch is off; when on, the libp2p
// transport and the network advertisement default to on (zero-config
// "be discoverable as an HTTP source for what's already exposed"), while
// the cleartext path stays off and must be flipped on explicitly. See
// the per-field doc comments for why.
const (
	DefaultHTTPProviderEnabled            = false
	DefaultHTTPProviderLibp2p             = true
	DefaultHTTPProviderCleartext          = false
	DefaultHTTPProviderAnnounceMultiaddrs = true
)
