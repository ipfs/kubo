package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ipfs/go-ipfs/commands"
)

func assertHeaders(t *testing.T, resHeaders http.Header, reqHeaders map[string]string) {
	for name, value := range reqHeaders {
		if resHeaders.Get(name) != value {
			t.Errorf("Invalid header `%s', wanted `%s', got `%s'", name, value, resHeaders.Get(name))
		}
	}
}

func TestDisallowedOrigin(t *testing.T) {
	res := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "http://example.com/foo", nil)
	req.Header.Add("Origin", "http://barbaz.com")

	handler := NewHandler(commands.Context{}, nil, "")
	handler.ServeHTTP(res, req)

	assertHeaders(t, res.Header(), map[string]string{
		"Access-Control-Allow-Origin":      "",
		"Access-Control-Allow-Methods":     "",
		"Access-Control-Allow-Credentials": "",
		"Access-Control-Max-Age":           "",
		"Access-Control-Expose-Headers":    "",
	})
}

func TestWildcardOrigin(t *testing.T) {
	res := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "http://example.com/foo", nil)
	req.Header.Add("Origin", "http://foobar.com")

	handler := NewHandler(commands.Context{}, nil, "*")
	handler.ServeHTTP(res, req)

	assertHeaders(t, res.Header(), map[string]string{
		"Access-Control-Allow-Origin":      "http://foobar.com",
		"Access-Control-Allow-Methods":     "",
		"Access-Control-Allow-Headers":     "",
		"Access-Control-Allow-Credentials": "",
		"Access-Control-Max-Age":           "",
		"Access-Control-Expose-Headers":    "",
	})
}

func TestAllowedMethod(t *testing.T) {
	res := httptest.NewRecorder()
	req, _ := http.NewRequest("OPTIONS", "http://example.com/foo", nil)
	req.Header.Add("Origin", "http://www.foobar.com")
	req.Header.Add("Access-Control-Request-Method", "PUT")

	handler := NewHandler(commands.Context{}, nil, "http://www.foobar.com")
	handler.ServeHTTP(res, req)

	assertHeaders(t, res.Header(), map[string]string{
		"Access-Control-Allow-Origin":      "http://www.foobar.com",
		"Access-Control-Allow-Methods":     "PUT",
		"Access-Control-Allow-Headers":     "",
		"Access-Control-Allow-Credentials": "",
		"Access-Control-Max-Age":           "",
		"Access-Control-Expose-Headers":    "",
	})
}
