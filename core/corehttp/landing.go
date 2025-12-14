package corehttp

import (
	"bufio"
	_ "embed"
	"net"
	"net/http"

	core "github.com/ipfs/kubo/core"
)

//go:embed assets/landing.html
var landingPageHTML []byte

// LandingPageOption returns a ServeOption that serves a default landing page
// for the gateway root ("/") when Gateway.RootRedirect is not configured.
// This helps third-party gateway operators by clearly indicating that the
// gateway software is working but needs configuration, and provides guidance
// for abuse reporting.
func LandingPageOption() ServeOption {
	return func(n *core.IpfsNode, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		cfg, err := n.Repo.Config()
		if err != nil {
			return nil, err
		}
		headers := cfg.Gateway.HTTPHeaders
		mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" {
				http.NotFound(w, r)
				return
			}
			serveLandingPage(w, headers)
		}))
		return mux, nil
	}
}

// serveLandingPage writes the landing page HTML with appropriate headers.
func serveLandingPage(w http.ResponseWriter, headers map[string][]string) {
	for k, v := range headers {
		w.Header()[http.CanonicalHeaderKey(k)] = v
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(landingPageHTML)
}

// withLandingPageFallback wraps an http.Handler to intercept 404 responses for
// the root path "/" on loopback addresses and serve a landing page instead.
//
// This is needed because boxo's HostnameHandler returns 404 for bare gateway
// hostnames (like "localhost") that don't have content configured. Without this
// fallback, users would see a confusing 404 instead of a helpful landing page.
//
// The middleware only intercepts requests to loopback addresses (127.0.0.1,
// localhost, ::1) because these cannot have DNSLink configured, so any 404 on
// "/" is guaranteed to be "no content configured" rather than "content not
// found". This avoids false positives where a real 404 (e.g., from DNSLink
// pointing to missing content) would incorrectly show the landing page.
func withLandingPageFallback(next http.Handler, headers map[string][]string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only intercept requests to exactly "/"
		if r.URL.Path != "/" {
			next.ServeHTTP(w, r)
			return
		}

		// Only intercept for loopback addresses. These cannot have DNSLink
		// configured, so any 404 is genuinely "no content configured".
		// For other hosts, pass through to avoid intercepting real 404s
		// from DNSLink or other content resolution.
		host := r.Host
		if h, _, err := net.SplitHostPort(r.Host); err == nil {
			host = h
		}
		switch host {
		case "localhost", "127.0.0.1", "::1", "[::1]":
			// Continue to intercept
		default:
			next.ServeHTTP(w, r)
			return
		}

		// Wrap ResponseWriter to intercept 404 responses
		lw := &landingResponseWriter{ResponseWriter: w}
		next.ServeHTTP(lw, r)

		// If 404 was suppressed, serve the landing page
		if lw.suppressed404 {
			serveLandingPage(w, headers)
		}
	})
}

// landingResponseWriter wraps http.ResponseWriter to intercept 404 responses.
// It suppresses the 404 status and body so we can serve a landing page instead.
type landingResponseWriter struct {
	http.ResponseWriter
	wroteHeader   bool
	suppressed404 bool
}

func (w *landingResponseWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	if code == http.StatusNotFound {
		w.suppressed404 = true
		return // Suppress 404 - we'll serve landing page instead
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *landingResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if w.suppressed404 {
		return len(b), nil // Discard 404 body
	}
	return w.ResponseWriter.Write(b)
}

// Flush implements http.Flusher for streaming responses.
func (w *landingResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack implements http.Hijacker for websocket support.
func (w *landingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// Unwrap returns the underlying ResponseWriter for http.ResponseController.
func (w *landingResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
