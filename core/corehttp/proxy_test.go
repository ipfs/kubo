package corehttp

import (
	"net/http"
	"strings"
	"testing"

	"github.com/ipfs/go-ipfs/thirdparty/assert"

	protocol "github.com/libp2p/go-libp2p-core/protocol"
)

type TestCase struct {
	urlprefix string
	target    string
	name      string
	path      string
}

var validtestCases = []TestCase{
	{"http://localhost:5001", "QmT8JtU54XSmC38xSb1XHFSMm775VuTeajg7LWWWTAwzxT", "/http", "path/to/index.txt"},
	{"http://localhost:5001", "QmT8JtU54XSmC38xSb1XHFSMm775VuTeajg7LWWWTAwzxT", "/x/custom/http", "path/to/index.txt"},
	{"http://localhost:5001", "QmT8JtU54XSmC38xSb1XHFSMm775VuTeajg7LWWWTAwzxT", "/x/custom/http", "http/path/to/index.txt"},
}

func TestParseRequest(t *testing.T) {
	for _, tc := range validtestCases {
		url := tc.urlprefix + "/p2p/" + tc.target + tc.name + "/" + tc.path
		req, _ := http.NewRequest(http.MethodGet, url, strings.NewReader(""))

		parsed, err := parseRequest(req)
		if err != nil {
			t.Fatal(err)
		}
		assert.True(parsed.httpPath == tc.path, t, "proxy request path")
		assert.True(parsed.name == protocol.ID(tc.name), t, "proxy request name")
		assert.True(parsed.target == tc.target, t, "proxy request peer-id")
	}
}

var invalidtestCases = []string{
	"http://localhost:5001/p2p/http/foobar",
	"http://localhost:5001/p2p/QmT8JtU54XSmC38xSb1XHFSMm775VuTeajg7LWWWTAwzxT/x/custom/foobar",
}

func TestParseRequestInvalidPath(t *testing.T) {
	for _, tc := range invalidtestCases {
		url := tc
		req, _ := http.NewRequest(http.MethodGet, url, strings.NewReader(""))

		_, err := parseRequest(req)
		if err == nil {
			t.Fail()
		}
	}
}
