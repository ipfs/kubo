package cli

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
)

func TestContentBlocking(t *testing.T) {
	t.Parallel()

	const blockedMsg = "blocked and cannot be provided"

	h := harness.NewT(t)

	// Init IPFS_PATH
	node := h.NewNode().Init("--empty-repo", "--profile=test")

	// Create CIDs we use in test
	h.WriteFile("blocked-dir/indirectly-blocked-file.txt", "indirectly blocked file content")
	blockedDirCID := node.IPFS("add", "-Q", "-r", filepath.Join(h.Dir, "blocked-dir")).Stdout.Trimmed()
	// indirectlyBlockedFileCID := node.IPFS("add", "-Q", filepath.Join(h.Dir, "blocked-dir", "indirectly-blocked-file.txt")).Stderr.Trimmed()

	h.WriteFile("directly-blocked-file.txt", "directly blocked file content")
	blockedCID := node.IPFS("add", "-Q", filepath.Join(h.Dir, "directly-blocked-file.txt")).Stdout.Trimmed()

	h.WriteFile("not-blocked-file.txt", "not blocked file content")
	allowedCID := node.IPFS("add", "-Q", filepath.Join(h.Dir, "not-blocked-file.txt")).Stdout.Trimmed()

	// Create denylist at $IPFS_PATH/denylists/test.deny
	denylistTmp := h.WriteToTemp(fmt.Sprintf(
		"//QmX9dhRcQcKUw3Ws8485T5a9dtjrSCQaUAHnG4iK9i4ceM\n"+ // Double hash CID block: base58btc-sha256-multihash(QmVTF1yEejXd9iMgoRTFDxBv7HAz9kuZcQNBzHrceuK9HR)
			"//QmbK7LDv5NNBvYQzNfm2eED17SNLt1yNMapcUhSuNLgkqz\n"+ // Double hash Path block using blake3 hashing: base58btc-blake3-multihash(gW7Nhu4HrfDtphEivm3Z9NNE7gpdh5Tga8g6JNZc1S8E47/path)
			"//d9d295bde21f422d471a90f2a37ec53049fdf3e5fa3ee2e8f20e10003da429e7\n"+ // Legacy CID double-hash block: sha256(bafybeiefwqslmf6zyyrxodaxx4vwqircuxpza5ri45ws3y5a62ypxti42e/)
			"//3f8b9febd851873b3774b937cce126910699ceac56e72e64b866f8e258d09572\n"+ // Legacy Path double-hash block: sha256(bafybeiefwqslmf6zyyrxodaxx4vwqircuxpza5ri45ws3y5a62ypxti42e/path)
			"/ipfs/%s/*\n"+ // block subpaths under a CID
			"/ipfs/%s\n"+ // block specific CID
			"/ipns/blocked-cid.example.com\n"+
			"/ipns/blocked-dnslink.example.com\n",
		blockedDirCID, blockedCID))

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

	// Start daemon, it should pick up denylist from $IPFS_PATH/denylists/test.deny
	node.StartDaemon("--offline")
	client := node.GatewayClient()

	// First, confirm gateway works
	t.Run("Gateway Allows CID that is not blocked", func(t *testing.T) {
		t.Parallel()
		resp := client.Get(fmt.Sprintf("/ipfs/%s", allowedCID))
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		// TODO assert.Equal(t, http.StatusGone, resp.StatusCode)
		assert.Equal(t, "not blocked file content", resp.Body)
	})

	// Then, does the most basic blocking case work?
	t.Run("Gateway Denies directly blocked CID", func(t *testing.T) {
		t.Parallel()
		resp := client.Get(fmt.Sprintf("/ipfs/%s", blockedCID))
		assert.NotEqual(t, http.StatusOK, resp.StatusCode)
		assert.NotEqual(t, "directly blocked file content", resp.Body)
		assert.Contains(t, resp.Body, blockedMsg)
	})

	// Ok, now the full list of test cases we want to cover in both CLI and Gateway
	testCases := []struct {
		name string
		path string
	}{
		{
			name: "directly blocked CID",
			path: fmt.Sprintf("/ipfs/%s", blockedCID),
		},
		{
			// TODO: this works for CLI but fails on Gateway
			name: "indirectly blocked subpath",
			path: fmt.Sprintf("/ipfs/%s/indirectly-blocked-file.txt", blockedDirCID),
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
			name: "double hash CID block: base58btc-sha256-multihash",
			path: "/ipfs/QmVTF1yEejXd9iMgoRTFDxBv7HAz9kuZcQNBzHrceuK9HR",
		},
		/* TODO
		{
			name: "double hash Path block: base58btc-blake3-multihash",
			path: "/ipfs/bafyb4ieqht3b2rssdmc7sjv2cy2gfdilxkfh7623nvndziyqnawkmo266a/path",
		},
		*/
		{
			name: "legacy CID double-hash block: sha256",
			path: "/ipfs/bafybeiefwqslmf6zyyrxodaxx4vwqircuxpza5ri45ws3y5a62ypxti42e",
		},

		/* TODO
		{
			name: "legacy Path double-hash: sha256",
			path: "/ipfs/bafybeiefwqslmf6zyyrxodaxx4vwqircuxpza5ri45ws3y5a62ypxti42e/path",
		},
		*/
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
				errMsg := node.RunIPFS(args...).Stderr.Trimmed()
				if !strings.Contains(errMsg, expectedMsg) {
					t.Errorf("Expected STDERR error message %q, but got: %q", expectedMsg, errMsg)
				}
			})
		}

		// Confirm that denylist is active for every content path in 'testCases'
		gwTestName := fmt.Sprintf("Gateway denies %s", testCase.name)
		t.Run(gwTestName, func(t *testing.T) {
			resp := client.Get(testCase.path)
			// TODO we should require HTTP 410, not 5XX: assert.Equal(t, http.StatusGone, resp.StatusCode)
			assert.NotEqual(t, http.StatusOK, resp.StatusCode)
			assert.Contains(t, resp.Body, blockedMsg)
		})

	}

	// Extra edge cases on subdomain gateway

	t.Run("Gateway Denies /ipns Path that is blocked by DNSLink name (subdomain redirect)", func(t *testing.T) {
		t.Parallel()

		gwURL, _ := url.Parse(node.GatewayURL())
		resp := client.Get("/ipns/blocked-dnslink.example.com", func(r *http.Request) {
			r.Host = "localhost:" + gwURL.Port()
		})

		assert.NotEqual(t, http.StatusOK, resp.StatusCode)
		// TODO assert.Equal(t, http.StatusGone, resp.StatusCode)
		assert.Contains(t, resp.Body, blockedMsg)
	})

	t.Run("Gateway Denies /ipns Path that is blocked by DNSLink name (subdomain, no TLS)", func(t *testing.T) {
		t.Parallel()

		gwURL, _ := url.Parse(node.GatewayURL())
		resp := client.Get("/", func(r *http.Request) {
			r.Host = "blocked-dnslink.example.com.ipns.localhost:" + gwURL.Port()
		})

		assert.NotEqual(t, http.StatusOK, resp.StatusCode)
		// TODO assert.Equal(t, http.StatusGone, resp.StatusCode)
		assert.Contains(t, resp.Body, blockedMsg)
	})

	t.Run("Gateway Denies /ipns Path that is blocked by DNSLink name (subdomain, inlined for TLS)", func(t *testing.T) {
		t.Parallel()

		gwURL, _ := url.Parse(node.GatewayURL())
		resp := client.Get("/", func(r *http.Request) {
			// Inlined DNSLink to fit in single DNS label for TLS interop:
			// https://specs.ipfs.tech/http-gateways/subdomain-gateway/#host-request-header
			r.Host = "blocked--dnslink-example-com.ipns.localhost:" + gwURL.Port()
		})

		assert.NotEqual(t, http.StatusOK, resp.StatusCode)
		// TODO assert.Equal(t, http.StatusGone, resp.StatusCode)
		assert.Contains(t, resp.Body, blockedMsg)
	})

}
