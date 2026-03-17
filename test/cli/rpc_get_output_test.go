package cli

import (
	"net/http"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRPCGetContentType verifies that the RPC endpoint for `ipfs get` returns
// the correct Content-Type header based on output format options.
//
// Output formats and expected Content-Type:
// - default (no flags): tar (transport format) -> application/x-tar
// - --archive:          tar archive           -> application/x-tar
// - --compress:         gzip                  -> application/gzip
// - --archive --compress: tar.gz              -> application/gzip
//
// Fixes: https://github.com/ipfs/kubo/issues/2376
func TestRPCGetContentType(t *testing.T) {
	t.Parallel()

	node := harness.NewT(t).NewNode().Init()
	node.StartDaemon("--offline")

	// add test content
	cid := node.IPFSAddStr("test content for Content-Type header verification")

	tests := []struct {
		name                string
		query               string
		expectedContentType string
	}{
		{
			name:                "default returns application/x-tar",
			query:               "?arg=" + cid,
			expectedContentType: "application/x-tar",
		},
		{
			name:                "archive=true returns application/x-tar",
			query:               "?arg=" + cid + "&archive=true",
			expectedContentType: "application/x-tar",
		},
		{
			name:                "compress=true returns application/gzip",
			query:               "?arg=" + cid + "&compress=true",
			expectedContentType: "application/gzip",
		},
		{
			name:                "archive=true&compress=true returns application/gzip",
			query:               "?arg=" + cid + "&archive=true&compress=true",
			expectedContentType: "application/gzip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := node.APIURL() + "/api/v0/get" + tt.query

			req, err := http.NewRequest(http.MethodPost, url, nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, tt.expectedContentType, resp.Header.Get("Content-Type"),
				"Content-Type header mismatch for %s", tt.name)
		})
	}
}
