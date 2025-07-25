package corehttp

import (
	"net/http"
	"strings"
	"testing"

	protocol "github.com/libp2p/go-libp2p/core/protocol"
	"github.com/stretchr/testify/require"
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
		require.NoError(t, err)
		require.Equal(t, tc.path, parsed.httpPath, "proxy request path")
		require.Equal(t, protocol.ID(tc.name), parsed.name, "proxy request name")
		require.Equal(t, tc.target, parsed.target, "proxy request peer-id")
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
		require.Error(t, err)
	}
}
