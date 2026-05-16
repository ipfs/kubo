package libp2p

import (
	"net/http"
	"sync/atomic"
)

// HTTPProviderHandler is an http.Handler whose target can be set after the
// libp2p host is constructed. We need this because the WebSocket transport
// option is wired into the host at FX init time (before *core.IpfsNode and
// the trustless gateway handler exist), but the handler we want to expose
// behind the AutoTLS cert is only available later, once IpfsNode is up.
//
// Used by the HTTPProvider feature that exposes the trustless gateway
// (NoFetch, raw blocks + CAR only) on the same TCP port as the /tls/ws
// AutoWSS listener. See `cmd/ipfs/kubo/daemon.go:serveHTTPProviderOverLibp2p`
// for the install site.
//
// Until Set is called, the handler responds 503 Service Unavailable, which
// is the correct signal for the small race between Accept and handler-install
// during daemon startup.
type HTTPProviderHandler struct {
	target atomic.Pointer[http.Handler]
}

// NewHTTPProviderHandler returns an empty handler. Call Set once the real
// handler is built.
func NewHTTPProviderHandler() *HTTPProviderHandler {
	return &HTTPProviderHandler{}
}

// Set installs the handler that should serve incoming non-WebSocket requests.
// Safe to call concurrently with ServeHTTP.
func (l *HTTPProviderHandler) Set(h http.Handler) {
	l.target.Store(&h)
}

// ServeHTTP forwards to the installed handler, or returns 503 if none has been
// installed yet.
func (l *HTTPProviderHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h := l.target.Load(); h != nil {
		(*h).ServeHTTP(w, r)
		return
	}
	http.Error(w, "gateway not ready yet", http.StatusServiceUnavailable)
}

// RequireHTTP2OverTLS wraps an http.Handler so that requests arriving over
// TLS with HTTP/1.1 are rejected with 426 Upgrade Required. Cleartext
// requests pass through regardless of HTTP version, so reverse-proxy
// deployments that forward HTTP/1.1 to the backend keep working.
//
// Why this lives in kubo and not in the go-libp2p WS transport: the
// transport's WithFallbackHTTPHandler is intentionally interop-maximal
// (accepts every HTTP version on every listener) and leaves application
// policy to the caller. The HTTP-over-TLS path here is the public-facing
// AutoTLS endpoint we want to keep looking like a modern HTTPS server
// (multiplexing for bitswap-httpnet, smaller fingerprint surface for
// censors). The plain /ws path stays permissive so reverse proxies that
// only speak HTTP/1.1 to the backend keep working.
//
// HTTP/2 is detected via r.ProtoMajor; TLS via r.TLS != nil. Both fields
// are populated by net/http before the handler runs.
func RequireHTTP2OverTLS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS != nil && r.ProtoMajor < 2 {
			w.Header().Set("Connection", "Upgrade")
			w.Header().Set("Upgrade", "h2,websocket")
			http.Error(w, "this endpoint requires HTTP/2 over TLS; HTTP/1.1 is reserved for the WebSocket upgrade", http.StatusUpgradeRequired)
			return
		}
		h.ServeHTTP(w, r)
	})
}
