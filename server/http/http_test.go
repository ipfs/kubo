package http

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	core "github.com/jbenet/go-ipfs/core"
	merkledag "github.com/jbenet/go-ipfs/merkledag"
)

type getTest struct {
	url  string
	code int
	body string
}

func setup() {
	resolvePath = func(path string, node *core.IpfsNode) (*merkledag.Node, error) {
		if path == "/QmUxtEgtan9M7acwc8SXF3MGpgpD9Ya8ViLNGEXQ6n9vfA" {
			return &merkledag.Node{Data: []byte("some fine data")}, nil
		}

		return nil, errors.New("")
	}
}

func TestServeHTTP(t *testing.T) {
	setup()
	testhandler := &ipfsHandler{}
	tests := []getTest{
		{"/", http.StatusInternalServerError, ""},
		{"/QmUxtEgtan9M7acwc8SXF3MGpgpD9Ya8ViLNGEXQ6n9vfA", http.StatusOK, "some fine data"},
	}

	for _, test := range tests {
		req, _ := http.NewRequest("GET", test.url, nil)
		resp := httptest.NewRecorder()
		testhandler.ServeHTTP(resp, req)

		if resp.Code != test.code {
			t.Error("expected status code", test.code, "received", resp.Code)
		}

		if resp.Body.String() != test.body {
			t.Error("expected body:", test.body)
			t.Error("received body:", resp.Body)
		}
	}
}
