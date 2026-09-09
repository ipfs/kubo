package libp2p

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPProviderHandler_Unset(t *testing.T) {
	fb := NewHTTPProviderHandler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	fb.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 before Set, got %d", rec.Code)
	}
}

func TestHTTPProviderHandler_Set(t *testing.T) {
	fb := NewHTTPProviderHandler()
	fb.Set(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = io.WriteString(w, "served")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	fb.ServeHTTP(rec, req)

	if rec.Code != http.StatusTeapot {
		t.Fatalf("want 418 after Set, got %d", rec.Code)
	}
	if got := rec.Body.String(); got != "served" {
		t.Fatalf("unexpected body: %q", got)
	}
}

// TestHTTPProviderHandler_Reset confirms Set replaces the previous handler;
// useful in case the gateway needs to be swapped during reconfiguration.
func TestHTTPProviderHandler_Reset(t *testing.T) {
	fb := NewHTTPProviderHandler()
	fb.Set(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }))
	fb.Set(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(500) }))

	rec := httptest.NewRecorder()
	fb.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != 500 {
		t.Fatalf("want second handler (500), got %d", rec.Code)
	}
}

// TestRequireHTTP2OverTLS exercises the four (TLS yes/no × h1/h2) cases.
func TestRequireHTTP2OverTLS(t *testing.T) {
	hits := 0
	wrapped := RequireHTTP2OverTLS(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	}))

	tlsState := &tls.ConnectionState{}

	cases := []struct {
		name       string
		tls        *tls.ConnectionState
		protoMajor int
		wantStatus int
		wantHits   int
	}{
		{"TLS_h1_rejected", tlsState, 1, http.StatusUpgradeRequired, 0},
		{"TLS_h2_allowed", tlsState, 2, http.StatusOK, 1},
		{"cleartext_h1_allowed", nil, 1, http.StatusOK, 1},
		{"cleartext_h2_allowed", nil, 2, http.StatusOK, 1},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hits = 0
			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			req.TLS = c.tls
			req.ProtoMajor = c.protoMajor

			rec := httptest.NewRecorder()
			wrapped.ServeHTTP(rec, req)

			if rec.Code != c.wantStatus {
				t.Fatalf("status: want %d, got %d", c.wantStatus, rec.Code)
			}
			if hits != c.wantHits {
				t.Fatalf("handler invocations: want %d, got %d", c.wantHits, hits)
			}
			if c.wantStatus == http.StatusUpgradeRequired {
				if got := rec.Header().Get("Upgrade"); got != "h2,websocket" {
					t.Fatalf("Upgrade header: want %q, got %q", "h2,websocket", got)
				}
			}
		})
	}
}
