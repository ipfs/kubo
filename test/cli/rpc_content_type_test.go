package cli

import (
	"net/http"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRPCDagExportContentType verifies that the RPC endpoint for `ipfs dag export`
// returns the correct Content-Type header for CAR output.
func TestRPCDagExportContentType(t *testing.T) {
	t.Parallel()

	node := harness.NewT(t).NewNode().Init()
	node.StartDaemon("--offline")

	// add test content
	cid := node.IPFSAddStr("test content for dag export")

	url := node.APIURL() + "/api/v0/dag/export?arg=" + cid

	req, err := http.NewRequest(http.MethodPost, url, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/vnd.ipld.car", resp.Header.Get("Content-Type"),
		"dag export should return application/vnd.ipld.car")
}

// TestRPCBlockGetContentType verifies that the RPC endpoint for `ipfs block get`
// returns the correct Content-Type header for raw block data.
func TestRPCBlockGetContentType(t *testing.T) {
	t.Parallel()

	node := harness.NewT(t).NewNode().Init()
	node.StartDaemon("--offline")

	// add test content
	cid := node.IPFSAddStr("test content for block get")

	url := node.APIURL() + "/api/v0/block/get?arg=" + cid

	req, err := http.NewRequest(http.MethodPost, url, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/vnd.ipld.raw", resp.Header.Get("Content-Type"),
		"block get should return application/vnd.ipld.raw")
}

// TestRPCProfileContentType verifies that the RPC endpoint for `ipfs diag profile`
// returns the correct Content-Type header for ZIP output.
func TestRPCProfileContentType(t *testing.T) {
	t.Parallel()

	node := harness.NewT(t).NewNode().Init()
	node.StartDaemon("--offline")

	// use profile-time=0 to skip sampling profiles and return quickly
	url := node.APIURL() + "/api/v0/diag/profile?profile-time=0"

	req, err := http.NewRequest(http.MethodPost, url, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/zip", resp.Header.Get("Content-Type"),
		"diag profile should return application/zip")
}
