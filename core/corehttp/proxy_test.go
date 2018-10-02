package corehttp

import (
	"github.com/ipfs/go-ipfs/thirdparty/assert"
	"net/http"
	"strings"
	"testing"
)

func TestParseRequest(t *testing.T) {
	url := "http://localhost:5001/proxy/http/QmT8JtU54XSmC38xSb1XHFSMm775VuTeajg7LWWWTAwzxT/test-name/path/to/index.txt"
	req, _ := http.NewRequest("GET", url, strings.NewReader(""))

	parsed, err := parseRequest(req)
	if err != nil {
		t.Error(err)
	}
	assert.True(parsed.httpPath == "path/to/index.txt", t, "proxy request path")
	assert.True(parsed.name == "test-name", t, "proxy request name")
	assert.True(parsed.target.Pretty() == "QmT8JtU54XSmC38xSb1XHFSMm775VuTeajg7LWWWTAwzxT", t, "proxy request peer-id")
}

func TestParseRequestInvalidPath(t *testing.T) {
	url := "http://localhost:5001/proxy/http/foobar"
	req, _ := http.NewRequest("GET", url, strings.NewReader(""))

	_, err := parseRequest(req)
	if err == nil {
		t.Fail()
	}

	assert.True(err.Error() == "Invalid request path '/proxy/http/foobar'", t, "fails with invalid path")
}
