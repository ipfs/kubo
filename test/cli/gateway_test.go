package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	. "github.com/ipfs/kubo/test/cli/testutils"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGateway(t *testing.T) {
	t.Parallel()
	h := harness.NewT(t)
	node := h.NewNode().Init().StartDaemon("--offline")
	cid := node.IPFSAddStr("Hello Worlds!")

	client := node.GatewayClient()
	client.TemplateData = map[string]string{
		"CID":    cid,
		"PeerID": node.PeerID().String(),
	}

	t.Run("GET IPFS path succeeds", func(t *testing.T) {
		t.Parallel()
		resp := client.Get("/ipfs/{{.CID}}")
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("GET IPFS path with explicit ?filename succeeds with proper header", func(t *testing.T) {
		t.Parallel()
		resp := client.Get("/ipfs/{{.CID}}?filename=testтест.pdf")
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t,
			`inline; filename="test____.pdf"; filename*=UTF-8''test%D1%82%D0%B5%D1%81%D1%82.pdf`,
			resp.Headers.Get("Content-Disposition"),
		)
	})

	t.Run("GET IPFS path with explicit ?filename and &download=true succeeds with proper header", func(t *testing.T) {
		t.Parallel()
		resp := client.Get("/ipfs/{{.CID}}?filename=testтест.mp4&download=true")
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t,
			`attachment; filename="test____.mp4"; filename*=UTF-8''test%D1%82%D0%B5%D1%81%D1%82.mp4`,
			resp.Headers.Get("Content-Disposition"),
		)
	})

	// https://github.com/ipfs/go-ipfs/issues/4025#issuecomment-342250616
	t.Run("GET for Server Worker registration outside of an IPFS content root errors", func(t *testing.T) {
		t.Parallel()
		resp := client.Get("/ipfs/{{.CID}}?filename=sw.js", client.WithHeader("Service-Worker", "script"))
		assert.Equal(t, 400, resp.StatusCode)
		assert.Contains(t, resp.Body, "navigator.serviceWorker: registration is not allowed for this scope")
	})

	t.Run("GET IPFS directory path succeeds", func(t *testing.T) {
		t.Parallel()
		client := node.GatewayClient().DisableRedirects()

		pageContents := "hello i am a webpage"
		fileContents := "12345"
		h.WriteFile("dir/test", fileContents)
		h.WriteFile("dir/dirwithindex/index.html", pageContents)
		cids := node.IPFS("add", "-r", "-q", filepath.Join(h.Dir, "dir")).Stdout.Lines()

		rootCID := cids[len(cids)-1]
		client.TemplateData = map[string]string{
			"IndexFileCID": cids[0],
			"TestFileCID":  cids[1],
			"RootCID":      rootCID,
		}

		t.Run("GET IPFS the index file CID", func(t *testing.T) {
			t.Parallel()
			resp := client.Get("/ipfs/{{.IndexFileCID}}")
			assert.Equal(t, 200, resp.StatusCode)
			assert.Equal(t, pageContents, resp.Body)
		})

		t.Run("GET IPFS the test file CID", func(t *testing.T) {
			t.Parallel()
			resp := client.Get("/ipfs/{{.TestFileCID}}")
			assert.Equal(t, 200, resp.StatusCode)
			assert.Equal(t, fileContents, resp.Body)
		})

		t.Run("GET IPFS directory with index.html returns redirect to add trailing slash", func(t *testing.T) {
			t.Parallel()
			resp := client.Head("/ipfs/{{.RootCID}}/dirwithindex?query=to-remember")
			assert.Equal(t, 301, resp.StatusCode)
			assert.Equal(t,
				fmt.Sprintf("/ipfs/%s/dirwithindex/?query=to-remember", rootCID),
				resp.Headers.Get("Location"),
			)
		})

		// This enables go get to parse go-import meta tags from index.html files stored in IPFS
		// https://github.com/ipfs/kubo/pull/3963
		t.Run("GET IPFS directory with index.html and no trailing slash returns expected output when go-get is passed", func(t *testing.T) {
			t.Parallel()
			resp := client.Get("/ipfs/{{.RootCID}}/dirwithindex?go-get=1")
			assert.Equal(t, pageContents, resp.Body)
		})

		t.Run("GET IPFS directory with index.html and trailing slash returns expected output", func(t *testing.T) {
			t.Parallel()
			resp := client.Get("/ipfs/{{.RootCID}}/dirwithindex/?query=to-remember")
			assert.Equal(t, pageContents, resp.Body)
		})

		t.Run("GET IPFS nonexistent file returns 404 (Not Found)", func(t *testing.T) {
			t.Parallel()
			resp := client.Get("/ipfs/{{.RootCID}}/pleaseDontAddMe")
			assert.Equal(t, 404, resp.StatusCode)
		})

		t.Run("GET IPFS invalid CID returns 500 (Internal Server Error)", func(t *testing.T) {
			t.Parallel()
			resp := client.Get("/ipfs/QmInvalid/pleaseDontAddMe")
			assert.Equal(t, 500, resp.StatusCode)
		})

		t.Run("GET IPFS inlined zero-length data object returns ok code (200)", func(t *testing.T) {
			t.Parallel()
			resp := client.Get("/ipfs/bafkqaaa")
			assert.Equal(t, 200, resp.StatusCode)
			assert.Equal(t, "0", resp.Resp.Header.Get("Content-Length"))
			assert.Equal(t, "", resp.Body)
		})

		t.Run("GET IPFS inlined zero-length data object with byte range returns ok code (200)", func(t *testing.T) {
			t.Parallel()
			resp := client.Get("/ipfs/bafkqaaa", client.WithHeader("Range", "bytes=0-1048575"))
			assert.Equal(t, 200, resp.StatusCode)
			assert.Equal(t, "0", resp.Resp.Header.Get("Content-Length"))
			assert.Equal(t, "text/plain", resp.Resp.Header.Get("Content-Type"))
		})

		t.Run("GET /ipfs/ipfs/{cid} returns redirect to the valid path", func(t *testing.T) {
			t.Parallel()
			resp := client.Get("/ipfs/ipfs/bafkqaaa?query=to-remember")
			assert.Contains(t,
				resp.Body,
				`<meta http-equiv="refresh" content="10;url=/ipfs/bafkqaaa?query=to-remember" />`,
			)
			assert.Contains(t,
				resp.Body,
				`<link rel="canonical" href="/ipfs/bafkqaaa?query=to-remember" />`,
			)
		})
	})

	t.Run("IPNS", func(t *testing.T) {
		t.Parallel()
		node.IPFS("name", "publish", "--allow-offline", cid)

		t.Run("GET invalid IPNS root returns 500 (Internal Server Error)", func(t *testing.T) {
			t.Parallel()
			resp := client.Get("/ipns/QmInvalid/pleaseDontAddMe")
			assert.Equal(t, 500, resp.StatusCode)
		})

		t.Run("GET IPNS path succeeds", func(t *testing.T) {
			t.Parallel()
			resp := client.Get("/ipns/{{.PeerID}}")
			assert.Equal(t, 200, resp.StatusCode)
			assert.Equal(t, "Hello Worlds!", resp.Body)
		})

		t.Run("GET /ipfs/ipns/{peerid} returns redirect to the valid path", func(t *testing.T) {
			t.Parallel()
			resp := client.Get("/ipfs/ipns/{{.PeerID}}?query=to-remember")
			peerID := node.PeerID().String()
			assert.Contains(t,
				resp.Body,
				fmt.Sprintf(`<meta http-equiv="refresh" content="10;url=/ipns/%s?query=to-remember" />`, peerID),
			)
			assert.Contains(t,
				resp.Body,
				fmt.Sprintf(`<link rel="canonical" href="/ipns/%s?query=to-remember" />`, peerID),
			)

		})

	})

	t.Run("GET invalid IPFS path errors", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, 500, client.Get("/ipfs/12345").StatusCode)
	})

	t.Run("GET invalid path errors", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, 404, client.Get("/12345").StatusCode)
	})

	// TODO: these tests that use the API URL shouldn't be part of gateway tests...
	t.Run("GET /webui returns 301 or 302", func(t *testing.T) {
		t.Parallel()
		resp := node.APIClient().DisableRedirects().Get("/webui")
		assert.Contains(t, []int{302, 301}, resp.StatusCode)
	})

	t.Run("GET /webui/ returns 301 or 302", func(t *testing.T) {
		t.Parallel()
		resp := node.APIClient().DisableRedirects().Get("/webui/")
		assert.Contains(t, []int{302, 301}, resp.StatusCode)
	})

	t.Run("GET /logs returns logs", func(t *testing.T) {
		t.Parallel()
		apiClient := node.APIClient()
		reqURL := apiClient.BuildURL("/logs")

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		require.NoError(t, err)

		resp, err := apiClient.Client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// read the first line of the output and parse its JSON
		dec := json.NewDecoder(resp.Body)
		event := struct{ Event string }{}
		err = dec.Decode(&event)
		require.NoError(t, err)

		assert.Equal(t, "log API client connected", event.Event)
	})

	t.Run("POST /api/v0/version succeeds", func(t *testing.T) {
		t.Parallel()
		resp := node.APIClient().Post("/api/v0/version", nil)
		assert.Equal(t, 200, resp.StatusCode)

		assert.Len(t, resp.Resp.TransferEncoding, 1)
		assert.Equal(t, "chunked", resp.Resp.TransferEncoding[0])

		vers := struct{ Version string }{}
		err := json.Unmarshal([]byte(resp.Body), &vers)
		require.NoError(t, err)
		assert.NotEmpty(t, vers.Version)
	})

	t.Run("pprof", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		apiClient := node.APIClient()
		t.Run("mutex", func(t *testing.T) {
			t.Parallel()
			t.Run("setting the mutex fraction works (negative so it doesn't enable)", func(t *testing.T) {
				t.Parallel()
				resp := apiClient.Post("/debug/pprof-mutex/?fraction=-1", nil)
				assert.Equal(t, 200, resp.StatusCode)
			})
			t.Run("mutex endpoint doesn't accept a string as an argument", func(t *testing.T) {
				t.Parallel()
				resp := apiClient.Post("/debug/pprof-mutex/?fraction=that_is_a_string", nil)
				assert.Equal(t, 400, resp.StatusCode)
			})
			t.Run("mutex endpoint returns 405 on GET", func(t *testing.T) {
				t.Parallel()
				resp := apiClient.Get("/debug/pprof-mutex/?fraction=-1")
				assert.Equal(t, 405, resp.StatusCode)
			})
		})
		t.Run("block", func(t *testing.T) {
			t.Parallel()
			t.Run("setting the block profiler rate works (0 so it doesn't enable)", func(t *testing.T) {
				t.Parallel()
				resp := apiClient.Post("/debug/pprof-block/?rate=0", nil)
				assert.Equal(t, 200, resp.StatusCode)
			})
			t.Run("block profiler endpoint doesn't accept a string as an argument", func(t *testing.T) {
				t.Parallel()
				resp := apiClient.Post("/debug/pprof-block/?rate=that_is_a_string", nil)
				assert.Equal(t, 400, resp.StatusCode)
			})
			t.Run("block profiler endpoint returns 405 on GET", func(t *testing.T) {
				t.Parallel()
				resp := apiClient.Get("/debug/pprof-block/?rate=0")
				assert.Equal(t, 405, resp.StatusCode)
			})
		})
	})

	t.Run("index content types", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)
		node := h.NewNode().Init().StartDaemon()

		h.WriteFile("index/index.html", "<p></p>")
		cid := node.IPFS("add", "-Q", "-r", filepath.Join(h.Dir, "index")).Stderr.Trimmed()

		apiClient := node.APIClient()
		apiClient.TemplateData = map[string]string{"CID": cid}

		t.Run("GET index.html has correct content type", func(t *testing.T) {
			t.Parallel()
			res := apiClient.Get("/ipfs/{{.CID}}/")
			assert.Equal(t, "text/html; charset=utf-8", res.Resp.Header.Get("Content-Type"))
		})

		t.Run("HEAD index.html has no content", func(t *testing.T) {
			t.Parallel()
			res := apiClient.Head("/ipfs/{{.CID}}/")
			assert.Equal(t, "", res.Body)
			assert.Equal(t, "", res.Resp.Header.Get("Content-Length"))
		})
	})

	t.Run("readonly API", func(t *testing.T) {
		t.Parallel()

		client := node.GatewayClient()

		fileContents := "12345"
		h.WriteFile("readonly/dir/test", fileContents)
		cids := node.IPFS("add", "-r", "-q", filepath.Join(h.Dir, "readonly/dir")).Stdout.Lines()

		rootCID := cids[len(cids)-1]
		client.TemplateData = map[string]string{"RootCID": rootCID}

		t.Run("Get IPFS directory file through readonly API succeeds", func(t *testing.T) {
			t.Parallel()
			resp := client.Get("/api/v0/cat?arg={{.RootCID}}/test")
			assert.Equal(t, 200, resp.StatusCode)
			assert.Equal(t, fileContents, resp.Body)
		})

		t.Run("refs IPFS directory file through readonly API succeeds", func(t *testing.T) {
			t.Parallel()
			resp := client.Get("/api/v0/refs?arg={{.RootCID}}/test")
			assert.Equal(t, 200, resp.StatusCode)
		})

		t.Run("test gateway API is sanitized", func(t *testing.T) {
			t.Parallel()
			for _, cmd := range []string{
				"add",
				"block/put",
				"bootstrap",
				"config",
				"dag/put",
				"dag/import",
				"dht",
				"diag",
				"id",
				"mount",
				"name/publish",
				"object/put",
				"object/new",
				"object/patch",
				"pin",
				"ping",
				"repo",
				"stats",
				"swarm",
				"file",
				"update",
				"bitswap",
			} {
				t.Run(cmd, func(t *testing.T) {
					cmd := cmd
					t.Parallel()
					assert.Equal(t, 404, client.Get("/api/v0/"+cmd).StatusCode)
				})
			}
		})
	})

	t.Run("refs/local", func(t *testing.T) {
		t.Parallel()
		gatewayAddr := URLStrToMultiaddr(node.GatewayURL())
		res := node.RunIPFS("--api", gatewayAddr.String(), "refs", "local")
		assert.Equal(t,
			`Error: invalid path "local": selected encoding not supported`,
			res.Stderr.Trimmed(),
		)
	})

	t.Run("raw leaves node", func(t *testing.T) {
		t.Parallel()
		contents := "This is RAW!"
		cid := node.IPFSAddStr(contents, "--raw-leaves")
		assert.Equal(t, contents, client.Get("/ipfs/"+cid).Body)
	})

	t.Run("compact blocks", func(t *testing.T) {
		t.Parallel()
		block1 := "\x0a\x09\x08\x02\x12\x03\x66\x6f\x6f\x18\x03"
		block2 := "\x0a\x04\x08\x02\x18\x06\x12\x24\x0a\x22\x12\x20\xcf\x92\xfd\xef\xcd\xc3\x4c\xac\x00\x9c" +
			"\x8b\x05\xeb\x66\x2b\xe0\x61\x8d\xb9\xde\x55\xec\xd4\x27\x85\xe9\xec\x67\x12\xf8\xdf\x65" +
			"\x12\x24\x0a\x22\x12\x20\xcf\x92\xfd\xef\xcd\xc3\x4c\xac\x00\x9c\x8b\x05\xeb\x66\x2b\xe0" +
			"\x61\x8d\xb9\xde\x55\xec\xd4\x27\x85\xe9\xec\x67\x12\xf8\xdf\x65"

		node.PipeStrToIPFS(block1, "block", "put")
		block2CID := node.PipeStrToIPFS(block2, "block", "put", "--cid-codec=dag-pb").Stdout.Trimmed()

		resp := client.Get("/ipfs/" + block2CID)
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, "foofoo", resp.Body)
	})

	t.Run("verify gateway file", func(t *testing.T) {
		t.Parallel()
		r := regexp.MustCompile(`Gateway \(readonly\) server listening on (?P<addr>.+)\s`)
		matches := r.FindStringSubmatch(node.Daemon.Stdout.String())
		ma, err := multiaddr.NewMultiaddr(matches[1])
		require.NoError(t, err)
		netAddr, err := manet.ToNetAddr(ma)
		require.NoError(t, err)
		expURL := "http://" + netAddr.String()

		b, err := os.ReadFile(filepath.Join(node.Dir, "gateway"))
		require.NoError(t, err)

		assert.Equal(t, expURL, string(b))
	})

	t.Run("verify gateway file diallable while on unspecified", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Addresses.Gateway = config.Strings{"/ip4/127.0.0.1/tcp/32563"}
		})
		node.StartDaemon()

		b, err := os.ReadFile(filepath.Join(node.Dir, "gateway"))
		require.NoError(t, err)

		assert.Equal(t, "http://127.0.0.1:32563", string(b))
	})

	t.Run("NoFetch", func(t *testing.T) {
		t.Parallel()
		nodes := harness.NewT(t).NewNodes(2).Init()
		node1 := nodes[0]
		node2 := nodes[1]

		node1.UpdateConfig(func(cfg *config.Config) {
			cfg.Gateway.NoFetch = true
		})

		nodes.StartDaemons().Connect()

		t.Run("not present", func(t *testing.T) {
			cidFoo := node2.IPFSAddStr("foo")

			t.Run("not present key from node 1", func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, 404, node1.GatewayClient().Get("/ipfs/"+cidFoo).StatusCode)
			})

			t.Run("not present IPNS key from node 1", func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, 500, node1.GatewayClient().Get("/ipns/"+node2.PeerID().String()).StatusCode)
			})
		})

		t.Run("present", func(t *testing.T) {
			cidBar := node1.IPFSAddStr("bar")

			t.Run("present key from node 1", func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, 200, node1.GatewayClient().Get("/ipfs/"+cidBar).StatusCode)
			})

			t.Run("present IPNS key from node 1", func(t *testing.T) {
				t.Parallel()
				node2.IPFS("name", "publish", "/ipfs/"+cidBar)
				assert.Equal(t, 200, node1.GatewayClient().Get("/ipns/"+node2.PeerID().String()).StatusCode)

			})
		})
	})
}
