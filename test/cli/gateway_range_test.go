package cli

import (
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
)

func TestGatewayHAMTDirectory(t *testing.T) {
	t.Parallel()

	const (
		// The CID of the HAMT-sharded directory that has 10k items
		hamtCid = "bafybeiggvykl7skb2ndlmacg2k5modvudocffxjesexlod2pfvg5yhwrqm"

		// fixtureCid is the CID of root of the DAG that is a subset of hamtCid DAG
		// representing the minimal set of blocks necessary for directory listing.
		// It also includes a "files_refs" file with the list of the references
		// we do NOT needs to fetch (files inside the directory)
		fixtureCid = "bafybeig3yoibxe56aolixqa4zk55gp5sug3qgaztkakpndzk2b2ynobd4i"
	)

	// Start node
	h := harness.NewT(t)
	node := h.NewNode().Init("--empty-repo", "--profile=test").StartDaemon("--offline")
	client := node.GatewayClient()

	// Import fixtures
	r, err := os.Open("./fixtures/TestGatewayHAMTDirectory.car")
	assert.NoError(t, err)
	defer r.Close()
	err = node.IPFSDagImport(r, fixtureCid)
	assert.NoError(t, err)

	// Fetch HAMT directory succeeds with minimal refs
	resp := client.Get(fmt.Sprintf("/ipfs/%s/", hamtCid))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestGatewayHAMTRanges(t *testing.T) {
	t.Parallel()

	const (
		// fileCid is the CID of the large HAMT-sharded file.
		fileCid = "bafybeiae5abzv6j3ucqbzlpnx3pcqbr2otbnpot7d2k5pckmpymin4guau"

		// fixtureCid is the CID of root of the DAG that is a subset of fileCid DAG
		// representing the minimal set of blocks necessary for a simple byte range request.
		fixtureCid = "bafybeicgsg3lwyn3yl75lw7sn4zhyj5dxtb7wfxwscpq6yzippetmr2w3y"
	)

	// Start node
	h := harness.NewT(t)
	node := h.NewNode().Init("--empty-repo", "--profile=test").StartDaemon("--offline")
	client := node.GatewayClient()

	// Import fixtures
	r, err := os.Open("./fixtures/TestGatewayMultiRange.car")
	assert.NoError(t, err)
	defer r.Close()
	err = node.IPFSDagImport(r, fixtureCid)
	assert.NoError(t, err)

	t.Run("Succeeds Fetching Range", func(t *testing.T) {
		t.Parallel()

		resp := client.Get(fmt.Sprintf("/ipfs/%s", fileCid), func(r *http.Request) {
			r.Header.Set("Range", "bytes=1276-1279")
		})
		assert.Equal(t, http.StatusPartialContent, resp.StatusCode)
		assert.Equal(t, "bytes 1276-1279/109266405", resp.Headers.Get("Content-Range"))
		assert.Equal(t, "iana", resp.Body)
	})

	t.Run("Succeeds Fetching Second Range", func(t *testing.T) {
		t.Parallel()

		resp := client.Get(fmt.Sprintf("/ipfs/%s", fileCid), func(r *http.Request) {
			r.Header.Set("Range", "bytes=29839070-29839080")
		})
		assert.Equal(t, http.StatusPartialContent, resp.StatusCode)
		assert.Equal(t, "bytes 29839070-29839080/109266405", resp.Headers.Get("Content-Range"))
		assert.Equal(t, "EXAMPLE.COM", resp.Body)
	})

	t.Run("Succeeds Fetching First Range of Multi-range Request", func(t *testing.T) {
		t.Parallel()

		resp := client.Get(fmt.Sprintf("/ipfs/%s", fileCid), func(r *http.Request) {
			r.Header.Set("Range", "bytes=1276-1279, 29839070-29839080")
		})
		assert.Equal(t, http.StatusPartialContent, resp.StatusCode)
		assert.Equal(t, "bytes 1276-1279/109266405", resp.Headers.Get("Content-Range"))
		assert.Equal(t, "iana", resp.Body)
	})
}
