package libp2p

import (
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

