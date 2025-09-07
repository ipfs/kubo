package cli

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/kubo/test/cli/harness"
	carstore "github.com/ipld/go-car/v2/blockstore"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	libp2phttp "github.com/libp2p/go-libp2p/p2p/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContentBlocking(t *testing.T) {
	// NOTE: we can't run this with t.Parallel() because we set IPFS_NS_MAP
	// and running in parallel could impact other tests

	const blockedMsg = "blocked and cannot be provided"
	const statusExpl = "specific HTTP error code is expected"
	const bodyExpl = "Error message informing about content block is expected"

	h := harness.NewT(t)

	// Init IPFS_PATH
	node := h.NewNode().Init("--empty-repo", "--profile=test")

	// Create CIDs we use in test
	h.WriteFile("parent-dir/blocked-subdir/indirectly-blocked-file.txt", "indirectly blocked file content")
	h.WriteFile("parent-dir/safe-subdir/safe-file.txt", "safe file content")
	allowedParentDirCID := node.IPFS("add", "--raw-leaves", "-Q", "-r", "--pin=false", filepath.Join(h.Dir, "parent-dir")).Stdout.Trimmed()

	// Get the CID of subdirectories as they exist within the parent DAG
	// Note: These CIDs are different from adding the directories standalone
	safeSubDirCID := node.IPFS("resolve", "-r", "/ipfs/"+allowedParentDirCID+"/safe-subdir").Stdout.Trimmed()
	safeSubDirCID = strings.TrimPrefix(safeSubDirCID, "/ipfs/")
	blockedSubDirCID := node.IPFS("resolve", "-r", "/ipfs/"+allowedParentDirCID+"/blocked-subdir").Stdout.Trimmed()
	blockedSubDirCID = strings.TrimPrefix(blockedSubDirCID, "/ipfs/")

	// Get the CID of the safe file within the safe subdirectory
	safeFileCID := node.IPFS("resolve", "-r", "/ipfs/"+allowedParentDirCID+"/safe-subdir/safe-file.txt").Stdout.Trimmed()
	safeFileCID = strings.TrimPrefix(safeFileCID, "/ipfs/")

	// Remove the blocked subdirectory from blockstore
	node.IPFS("block", "rm", blockedSubDirCID)

	h.WriteFile("directly-blocked-file.txt", "directly blocked file content")
	blockedCID := node.IPFS("add", "--raw-leaves", "-Q", filepath.Join(h.Dir, "directly-blocked-file.txt")).Stdout.Trimmed()

	h.WriteFile("not-blocked-file.txt", "not blocked file content")
	allowedCID := node.IPFS("add", "--raw-leaves", "-Q", filepath.Join(h.Dir, "not-blocked-file.txt")).Stdout.Trimmed()

	// Create denylist at $IPFS_PATH/denylists/test.deny
	denylistTmp := h.WriteToTemp("name: test list\n---\n" +
		"//QmX9dhRcQcKUw3Ws8485T5a9dtjrSCQaUAHnG4iK9i4ceM\n" + // Double hash (sha256) CID block: base58btc(sha256-multihash(QmVTF1yEejXd9iMgoRTFDxBv7HAz9kuZcQNBzHrceuK9HR))
		"//gW813G35CnLsy7gRYYHuf63hrz71U1xoLFDVeV7actx6oX\n" + // Double hash (blake3) Path block under blake3 root CID: base58btc(blake3-multihash(gW7Nhu4HrfDtphEivm3Z9NNE7gpdh5Tga8g6JNZc1S8E47/path))
		"//8526ba05eec55e28f8db5974cc891d0d92c8af69d386fc6464f1e9f372caf549\n" + // Legacy CID double-hash block: sha256(bafkqahtcnrxwg23fmqqgi33vmjwgk2dbonuca3dfm5qwg6jamnuwicq/)
		"//e5b7d2ce2594e2e09901596d8e1f29fa249b74c8c9e32ea01eda5111e4d33f07\n" + // Legacy Path double-hash block: sha256(bafyaagyscufaqalqaacauaqiaejao43vmjygc5didacauaqiae/subpath)
		"/ipfs/" + blockedCID + "\n" + // block specific CID
		"/ipfs/" + allowedParentDirCID + "/blocked-subdir*\n" + // block only specific subpath
		"/ipns/blocked-cid.example.com\n" +
		"/ipns/blocked-dnslink.example.com\n")

	if err := os.MkdirAll(filepath.Join(node.Dir, "denylists"), 0o777); err != nil {
		log.Panicf("failed to create denylists dir: %s", err.Error())
	}
	if err := os.Rename(denylistTmp, filepath.Join(node.Dir, "denylists", "test.deny")); err != nil {
		log.Panicf("failed to create test denylist: %s", err.Error())
	}

	// Add two entries to namesys resolution cache
	// /ipns/blocked-cid.example.com point at a blocked CID (to confirm blocking impacts /ipns resolution)
	// /ipns/blocked-dnslink.example.com with safe CID (to test blocking of /ipns/ paths)
	os.Setenv("IPFS_NS_MAP", "blocked-cid.example.com:/ipfs/"+blockedCID+",blocked-dnslink.example.com/ipns/QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn")
	defer os.Unsetenv("IPFS_NS_MAP")

	// Enable GatewayOverLibp2p as we want to test denylist there too
	node.IPFS("config", "--json", "Experimental.GatewayOverLibp2p", "true")

	// Start daemon, it should pick up denylist from $IPFS_PATH/denylists/test.deny
	node.StartDaemon() // we need online mode for GatewayOverLibp2p tests
	client := node.GatewayClient()

	// First, confirm gateway works
	t.Run("Gateway Allows CID that is not blocked", func(t *testing.T) {
		t.Parallel()
		resp := client.Get("/ipfs/" + allowedCID)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "not blocked file content", resp.Body)
	})

	// Then, does the most basic blocking case work?
	t.Run("Gateway Denies directly blocked CID", func(t *testing.T) {
		t.Parallel()
		resp := client.Get("/ipfs/" + blockedCID)
		assert.Equal(t, http.StatusGone, resp.StatusCode, statusExpl)
		assert.NotEqual(t, "directly blocked file content", resp.Body)
		assert.Contains(t, resp.Body, blockedMsg, bodyExpl)
	})

	// Confirm parent of blocked subpath is not blocked
	t.Run("Gateway Allows parent Path that is not blocked", func(t *testing.T) {
		t.Parallel()
		resp := client.Get("/ipfs/" + allowedParentDirCID)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	// Verify that when requesting a subdirectory as CAR, the response includes
	// the safe content even when a sibling directory is blocked
	t.Run("Gateway returns 200 with CAR for safe subdir when sibling is blocked", func(t *testing.T) {
		// Request the SAFE subdirectory, verifying it's accessible even though a sibling is blocked
		resp := client.Get("/ipfs/" + allowedParentDirCID + "/safe-subdir?format=car")
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		bs, err := carstore.NewReadOnly(strings.NewReader(resp.Body), nil)
		assert.NoError(t, err)

		roots, err := bs.Roots()
		assert.NoError(t, err)
		assert.Equal(t, 1, len(roots))
		assert.Equal(t, safeSubDirCID, roots[0].String())

		// Verify the safe content IS in the CAR
		ctx := context.TODO()
		has, err := bs.Has(ctx, cid.MustParse(safeSubDirCID))
		assert.NoError(t, err)
		assert.True(t, has, "CAR should include the safe subdirectory")

		// Verify the safe file within the subdirectory IS in the CAR
		has, err = bs.Has(ctx, cid.MustParse(safeFileCID))
		assert.NoError(t, err)
		assert.True(t, has, "CAR should include the safe file within the subdirectory")

		// Verify the blocked content is NOT in the CAR (it shouldn't be traversed)
		has, err = bs.Has(ctx, cid.MustParse(blockedSubDirCID))
		assert.NoError(t, err)
		assert.False(t, has, "CAR should not include the blocked subdirectory")
	})

	// Test that requesting non-existent path with CAR format returns 404 (not 410)
	// This ensures we distinguish between blocked content and missing content
	t.Run("Gateway returns 404 for non-existent path with CAR format", func(t *testing.T) {
		// Request a non-existent subdirectory as CAR
		resp := client.Get("/ipfs/" + allowedParentDirCID + "/safe-404?format=car")
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, "Non-existent path should return 404 Not Found")

		// Verify response body is not a CAR file but an error message
		// CAR files start with specific magic bytes
		assert.NotContains(t, resp.Body, "CAR", "404 response should not contain CAR data")
		// Verify it contains an appropriate error message
		assert.Contains(t, resp.Body, "no link named", "404 response should contain IPFS path resolution error")
	})

	// Test that confirms blocking of children skips them from produced CAR.
	t.Run("Gateway returns 200 with CAR without directly blocked CID", func(t *testing.T) {
		// First verify that the blocked CID is actually blocked when accessed directly
		directResp := client.Get("/ipfs/" + blockedCID)
		assert.Equal(t, http.StatusGone, directResp.StatusCode, "Direct access to blocked CID should return 410")

		allowedDirWithDirectlyBlockedCID := node.IPFS("add", "--raw-leaves", "-Q", "-rw", filepath.Join(h.Dir, "directly-blocked-file.txt")).Stdout.Trimmed()
		t.Logf("Directory CID containing blocked file: %s", allowedDirWithDirectlyBlockedCID)
		t.Logf("Blocked CID that should not appear in CAR: %s", blockedCID)
		resp := client.Get("/ipfs/" + allowedDirWithDirectlyBlockedCID + "?format=car")

		// We expect HTTP 200 because root cid is not blocked, so we start
		// streaming directory recursively
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		bs, err := carstore.NewReadOnly(strings.NewReader(resp.Body), nil)
		assert.NoError(t, err)

		// The blocked CID MUST be omitted from HTTP response
		has, err := bs.Has(context.Background(), cid.MustParse(blockedCID))
		assert.NoError(t, err)
		assert.False(t, has, "Returned CAR should not include blockedCID")
	})

	// Test that requesting a CAR with a blocked root CID returns 410
	t.Run("Gateway returns 410 for CAR request with blocked root CID", func(t *testing.T) {
		resp := client.Get("/ipfs/" + blockedCID + "?format=car")
		assert.Equal(t, http.StatusGone, resp.StatusCode, "CAR request for blocked root CID should return 410 Gone")
		assert.Contains(t, resp.Body, blockedMsg, "Error message should indicate content is blocked")
	})

	// Test that requesting raw format with a blocked root CID returns 410
	t.Run("Gateway returns 410 for raw request with blocked root CID", func(t *testing.T) {
		resp := client.Get("/ipfs/" + blockedCID + "?format=raw")
		assert.Equal(t, http.StatusGone, resp.StatusCode, "Raw request for blocked root CID should return 410 Gone")
		assert.Contains(t, resp.Body, blockedMsg, "Error message should indicate content is blocked")
	})

	// Ok, now the full list of test cases we want to cover in both CLI and Gateway
	testCases := []struct {
		name string
		path string
	}{
		{
			name: "directly blocked file CID",
			path: "/ipfs/" + blockedCID,
		},
		{
			name: "indirectly blocked file (on a blocked subpath)",
			path: "/ipfs/" + allowedParentDirCID + "/blocked-subdir/indirectly-blocked-file.txt",
		},
		{
			name: "/ipns path that resolves to a blocked CID",
			path: "/ipns/blocked-cid.example.com",
		},
		{
			name: "/ipns Path that is blocked by DNSLink name",
			path: "/ipns/blocked-dnslink.example.com",
		},
		{
			name: "double-hash CID block (sha256-multihash)",
			path: "/ipfs/QmVTF1yEejXd9iMgoRTFDxBv7HAz9kuZcQNBzHrceuK9HR",
		},
		{
			name: "double-hash Path block (blake3-multihash)",
			path: "/ipfs/bafyb4ieqht3b2rssdmc7sjv2cy2gfdilxkfh7623nvndziyqnawkmo266a/path",
		},
		{
			name: "legacy CID double-hash block (sha256)",
			path: "/ipfs/bafkqahtcnrxwg23fmqqgi33vmjwgk2dbonuca3dfm5qwg6jamnuwicq",
		},

		{
			name: "legacy Path double-hash block (sha256)",
			path: "/ipfs/bafyaagyscufaqalqaacauaqiaejao43vmjygc5didacauaqiae/subpath",
		},
	}

	// Which specific cliCmds we test against testCases
	cliCmds := [][]string{
		{"block", "get"},
		{"block", "stat"},
		{"dag", "get"},
		{"dag", "export"},
		{"dag", "stat"},
		{"cat"},
		{"ls"},
		{"get"},
		{"refs"},
	}

	expectedMsg := blockedMsg
	for _, testCase := range testCases {

		// Confirm that denylist is active for every command in 'cliCmds' x 'testCases'
		for _, cmd := range cliCmds {
			cmd := cmd
			cliTestName := fmt.Sprintf("CLI '%s' denies %s", strings.Join(cmd, " "), testCase.name)
			t.Run(cliTestName, func(t *testing.T) {
				t.Parallel()
				args := append(cmd, testCase.path)
				cmd := node.RunIPFS(args...)
				stdout := cmd.Stdout.Trimmed()
				stderr := cmd.Stderr.Trimmed()
				if !strings.Contains(stderr, expectedMsg) {
					t.Errorf("Expected STDERR error message %q, but got: %q", expectedMsg, stderr)
					if stdout != "" {
						t.Errorf("Expected STDOUT to be empty, but got: %q", stdout)
					}
				}
			})
		}

		// Confirm that denylist is active for every content path in 'testCases'
		gwTestName := fmt.Sprintf("Gateway denies %s", testCase.name)
		t.Run(gwTestName, func(t *testing.T) {
			resp := client.Get(testCase.path)
			assert.Equal(t, http.StatusGone, resp.StatusCode, statusExpl)
			assert.Contains(t, resp.Body, blockedMsg, bodyExpl)
		})

	}

	// Extra edge cases on subdomain gateway

	t.Run("Gateway Denies /ipns Path that is blocked by DNSLink name (subdomain redirect)", func(t *testing.T) {
		t.Parallel()

		gwURL, _ := url.Parse(node.GatewayURL())
		resp := client.Get("/ipns/blocked-dnslink.example.com", func(r *http.Request) {
			r.Host = "localhost:" + gwURL.Port()
		})

		assert.Equal(t, http.StatusGone, resp.StatusCode, statusExpl)
		assert.Contains(t, resp.Body, blockedMsg, bodyExpl)
	})

	t.Run("Gateway Denies /ipns Path that is blocked by DNSLink name (subdomain, no TLS)", func(t *testing.T) {
		t.Parallel()

		gwURL, _ := url.Parse(node.GatewayURL())
		resp := client.Get("/", func(r *http.Request) {
			r.Host = "blocked-dnslink.example.com.ipns.localhost:" + gwURL.Port()
		})

		assert.Equal(t, http.StatusGone, resp.StatusCode, statusExpl)
		assert.Contains(t, resp.Body, blockedMsg, bodyExpl)
	})

	t.Run("Gateway Denies /ipns Path that is blocked by DNSLink name (subdomain, inlined for TLS)", func(t *testing.T) {
		t.Parallel()

		gwURL, _ := url.Parse(node.GatewayURL())
		resp := client.Get("/", func(r *http.Request) {
			// Inlined DNSLink to fit in single DNS label for TLS interop:
			// https://specs.ipfs.tech/http-gateways/subdomain-gateway/#host-request-header
			r.Host = "blocked--dnslink-example-com.ipns.localhost:" + gwURL.Port()
		})

		assert.Equal(t, http.StatusGone, resp.StatusCode, statusExpl)
		assert.Contains(t, resp.Body, blockedMsg, bodyExpl)
	})

	// We need to confirm denylist is active when gateway is run in NoFetch
	// mode (which usually swaps blockservice to a read-only one, and that swap
	// may cause denylists to not be applied, as it is a separate code path)
	t.Run("GatewayNoFetch", func(t *testing.T) {
		// NOTE: we don't run this in parallel, as it requires restart with different config

		// Switch gateway to NoFetch mode
		node.StopDaemon()
		node.IPFS("config", "--json", "Gateway.NoFetch", "true")
		node.StartDaemon()

		// update client, as the port of test node might've changed after restart
		client = node.GatewayClient()

		// First, confirm gateway works
		t.Run("Allows CID that is not blocked", func(t *testing.T) {
			resp := client.Get("/ipfs/" + allowedCID)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, "not blocked file content", resp.Body)
		})

		// Then, does the most basic blocking case work?
		t.Run("Denies directly blocked CID", func(t *testing.T) {
			resp := client.Get("/ipfs/" + blockedCID)
			assert.Equal(t, http.StatusGone, resp.StatusCode, statusExpl)
			assert.NotEqual(t, "directly blocked file content", resp.Body)
			assert.Contains(t, resp.Body, blockedMsg, bodyExpl)
		})

		// Restore default
		node.StopDaemon()
		node.IPFS("config", "--json", "Gateway.NoFetch", "false")
		node.StartDaemon()
		client = node.GatewayClient()
	})

	// We need to confirm denylist is active on the
	// trustless gateway exposed over libp2p
	// when Experimental.GatewayOverLibp2p=true
	// (https://github.com/ipfs/kubo/blob/master/docs/experimental-features.md#http-gateway-over-libp2p)
	// NOTE: this type of gateway is hardcoded to be NoFetch: it does not fetch
	// data that is not in local store, so we only need to run it once: a
	// simple smoke-test for allowed CID and blockedCID.
	t.Run("GatewayOverLibp2p", func(t *testing.T) {
		t.Parallel()

		// Create libp2p client that connects to our node over
		// /http1.1 and then talks gateway semantics over the /ipfs/gateway sub-protocol
		clientHost, err := libp2p.New(libp2p.NoListenAddrs)
		require.NoError(t, err)
		err = clientHost.Connect(context.Background(), peer.AddrInfo{
			ID:    node.PeerID(),
			Addrs: node.SwarmAddrs(),
		})
		require.NoError(t, err)

		libp2pClient, err := (&libp2phttp.Host{StreamHost: clientHost}).NamespacedClient("/ipfs/gateway", peer.AddrInfo{ID: node.PeerID()})
		require.NoError(t, err)

		t.Run("Serves Allowed CID", func(t *testing.T) {
			t.Parallel()
			resp, err := libp2pClient.Get(fmt.Sprintf("/ipfs/%s?format=raw", allowedCID))
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, string(body), "not blocked file content", bodyExpl)
		})

		t.Run("Denies Blocked CID", func(t *testing.T) {
			t.Parallel()
			resp, err := libp2pClient.Get(fmt.Sprintf("/ipfs/%s?format=raw", blockedCID))
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, http.StatusGone, resp.StatusCode, statusExpl)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.NotEqual(t, string(body), "directly blocked file content")
			assert.Contains(t, string(body), blockedMsg, bodyExpl)
		})

		t.Run("Denies Blocked CID as CAR", func(t *testing.T) {
			t.Parallel()
			resp, err := libp2pClient.Get(fmt.Sprintf("/ipfs/%s?format=car", blockedCID))
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, http.StatusGone, resp.StatusCode, statusExpl)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.NotContains(t, string(body), "directly blocked file content")
			assert.Contains(t, string(body), blockedMsg, bodyExpl)
		})
	})
}
