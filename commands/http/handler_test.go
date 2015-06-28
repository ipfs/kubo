package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ipfs/go-ipfs/commands"
	corecommands "github.com/ipfs/go-ipfs/core/commands"
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

	commandList := map[*commands.Command]bool{}

	for _, cmd := range corecommands.Root.Subcommands {
		commandList[cmd] = true
	}

	handler := NewHandler(commands.Context{}, nil, "", commandList)
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

	commandList := map[*commands.Command]bool{}

	for _, cmd := range corecommands.Root.Subcommands {
		commandList[cmd] = true
	}

	handler := NewHandler(commands.Context{}, nil, "*", commandList)
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

	commandList := map[*commands.Command]bool{}

	for _, cmd := range corecommands.Root.Subcommands {
		commandList[cmd] = true
	}

	handler := NewHandler(commands.Context{}, nil, "http://www.foobar.com", commandList)
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

func TestLimitingCommands(t *testing.T) {
	res := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "http://example.com/api/v0/cat", nil)

	query := req.URL.Query()
	query.Add("arg", "QmFooBar")
	req.URL.RawQuery = query.Encode()

	commandList := map[*commands.Command]bool{}

	handler := NewHandler(commands.Context{}, corecommands.Root, "*", commandList)
	handler.ServeHTTP(res, req)

	if(res.Code != http.StatusForbidden) {
		t.Errorf("Invalid status code, expected `%d` received `%d`\nBody:\n %s", http.StatusForbidden, res.Code, res.Body)
	}
}
