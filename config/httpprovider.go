package config

// HTTPProvider configures kubo's role as an HTTP-native source of
// trustless-gateway block retrieval. When Enabled, the local trustless
// gateway handler (NoFetch, raw blocks via ?format=raw only,
// content-addressed verification client-side) is exposed for HTTP
// retrieval clients such as boxo/bitswap/network/httpnet.
//
// This is the server side of the HTTP retrieval story; the client side
// lives separately under HTTPRetrieval.
//
// Defaults live on the DefaultHTTPProvider* constants below.
type HTTPProvider struct {
	// Enabled is the master switch. When true, the trustless gateway
	// handler is registered for the libp2p-stream transport (see
	// Libp2p). Cleartext and AnnounceMultiaddrs gate further surfaces
	// and must each be set explicitly.
	Enabled Flag `json:",omitempty"`

	// Libp2p exposes the trustless gateway over a libp2p stream, per the
	// libp2p Gateway spec (https://specs.ipfs.tech/http-gateways/libp2p-gateway/).
	// The handler mounts under the /ipfs/gateway protocol ID and is
	// advertised via .well-known/libp2p/protocols on the libp2p+HTTP
	// host.
	Libp2p Flag `json:",omitempty"`

	// Cleartext auto-appends a plaintext /ws listener to each /tcp/N in
	// Addresses.Swarm, unless one is already present. The new /ws shares
	// the existing TCP port via the shared-TCP demuxer, so no extra
	// socket is opened.
	//
	// Intended for deployments where a reverse proxy (Caddy, Traefik,
	// nginx, etc.) terminates TLS in front of kubo and forwards HTTP/1.1
	// or h2c to this node. With AutoTLS, kubo already serves /tls/ws and
	// /tls/http with a Let's Encrypt cert, so a cleartext path would
	// expose the trustless gateway and WebSocket upgrade unencrypted on
	// the public network. Turn it on knowingly.
	//
	// The matching /http multiaddr announcement is controlled by
	// AnnounceMultiaddrs.
	Cleartext Flag `json:",omitempty"`

	// AnnounceMultiaddrs derives an HTTP-flavored multiaddr from each
	// WebSocket listener and adds it to the announced address set:
	// /ws to /http, /tls/ws to /tls/http, /tls/sni/<host>/ws to
	// /tls/sni/<host>/http. The HTTP endpoint shares the WebSocket
	// listener's TCP port and TLS cert; no extra socket is opened. HTTP
	// retrieval clients (e.g. boxo/bitswap/network/httpnet) then discover
	// this peer as an HTTP source through identify, the DHT, and IPNI
	// without out-of-band knowledge.
	//
	// Subject to Addresses.NoAnnounce filters, like any other announced
	// multiaddr.
	AnnounceMultiaddrs Flag `json:",omitempty"`
}

// HTTPProvider defaults. The master switch is off, so the feature is
// inert until an operator opts in. Once Enabled, the libp2p-stream
// transport comes up automatically because the .well-known descriptor
// already advertises that path; the cleartext listener and the /http
// announcement stay off so operators broadcast HTTP availability on
// purpose, not by accident.
const (
	DefaultHTTPProviderEnabled            = false
	DefaultHTTPProviderLibp2p             = true
	DefaultHTTPProviderCleartext          = false
	DefaultHTTPProviderAnnounceMultiaddrs = false
)
