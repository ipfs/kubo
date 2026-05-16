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
