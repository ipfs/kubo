// Tests HTTP RPC Content-Type headers.
// These tests verify that RPC endpoints return correct Content-Type headers
// for binary responses (CAR, tar, gzip, raw blocks, IPNS records).

package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
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

// TestHTTPRPCNameGet verifies the behavior of `ipfs name get` vs `ipfs routing get`:
//
// `ipfs name get <name>`:
//   - Purpose: dedicated command for retrieving IPNS records
//   - Returns: raw IPNS record bytes (protobuf)
//   - Content-Type: application/vnd.ipfs.ipns-record
//
// `ipfs routing get /ipns/<name>`:
//   - Purpose: generic routing get for any key type
//   - Returns: JSON with base64-encoded record in "Extra" field
//   - Content-Type: application/json
//
// Both commands retrieve the same underlying IPNS record data.
func TestHTTPRPCNameGet(t *testing.T) {
	t.Parallel()

	node := harness.NewT(t).NewNode().Init()
	node.StartDaemon() // must be online to use routing

	// add test content and publish IPNS record
	cid := node.IPFSAddStr("test content for name get")
	node.IPFS("name", "publish", cid)

	// get the node's peer ID (which is also the IPNS name)
	peerID := node.PeerID().String()

	// Test ipfs name get - returns raw IPNS record bytes with specific Content-Type
	nameGetURL := node.APIURL() + "/api/v0/name/get?arg=" + peerID
	nameGetReq, err := http.NewRequest(http.MethodPost, nameGetURL, nil)
	require.NoError(t, err)

	nameGetResp, err := http.DefaultClient.Do(nameGetReq)
	require.NoError(t, err)
	defer nameGetResp.Body.Close()

	assert.Equal(t, http.StatusOK, nameGetResp.StatusCode)
	assert.Equal(t, "application/vnd.ipfs.ipns-record", nameGetResp.Header.Get("Content-Type"),
		"name get should return application/vnd.ipfs.ipns-record")

	nameGetBytes, err := io.ReadAll(nameGetResp.Body)
	require.NoError(t, err)

	// Test ipfs routing get /ipns/... - returns JSON with base64-encoded record
	routingGetURL := node.APIURL() + "/api/v0/routing/get?arg=/ipns/" + peerID
	routingGetReq, err := http.NewRequest(http.MethodPost, routingGetURL, nil)
	require.NoError(t, err)

	routingGetResp, err := http.DefaultClient.Do(routingGetReq)
	require.NoError(t, err)
	defer routingGetResp.Body.Close()

	assert.Equal(t, http.StatusOK, routingGetResp.StatusCode)
	assert.Equal(t, "application/json", routingGetResp.Header.Get("Content-Type"),
		"routing get should return application/json")

	// Parse JSON response and decode base64 record from "Extra" field
	var routingResp struct {
		Extra string `json:"Extra"`
		Type  int    `json:"Type"`
	}
	err = json.NewDecoder(routingGetResp.Body).Decode(&routingResp)
	require.NoError(t, err)

	routingGetBytes, err := base64.StdEncoding.DecodeString(routingResp.Extra)
	require.NoError(t, err)

	// Verify both commands return identical IPNS record bytes
	assert.Equal(t, nameGetBytes, routingGetBytes,
		"name get and routing get should return identical IPNS record bytes")

	// Verify the record can be inspected and contains the published CID
	inspectOutput := node.PipeToIPFS(bytes.NewReader(nameGetBytes), "name", "inspect")
	assert.Contains(t, inspectOutput.Stdout.String(), cid,
		"ipfs name inspect should show the published CID")
}
