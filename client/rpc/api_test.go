package rpc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/boxo/path"
	iface "github.com/ipfs/kubo/core/coreiface"
	"github.com/ipfs/kubo/core/coreiface/tests"
	"github.com/ipfs/kubo/test/cli/harness"
	ma "github.com/multiformats/go-multiaddr"
	"go.uber.org/multierr"
)

type NodeProvider struct{}

func (np NodeProvider) MakeAPISwarm(t *testing.T, ctx context.Context, fullIdentity, online bool, n int) ([]iface.CoreAPI, error) {
	h := harness.NewT(t)

	apis := make([]iface.CoreAPI, n)
	nodes := h.NewNodes(n)

	var wg, zero sync.WaitGroup
	zeroNode := nodes[0]
	wg.Add(len(apis))
	zero.Add(1)

	var errs []error
	var errsLk sync.Mutex

	for i, n := range nodes {
		go func(i int, n *harness.Node) {
			if err := func() error {
				defer wg.Done()
				var err error

				n.Init("--empty-repo")

				c := n.ReadConfig()
				c.Experimental.FilestoreEnabled = true
				n.WriteConfig(c)
				n.StartDaemon("--enable-pubsub-experiment", "--offline="+strconv.FormatBool(!online))

				if online {
					if i > 0 {
						zero.Wait()
						n.Connect(zeroNode)
					} else {
						zero.Done()
					}
				}

				apiMaddr, err := n.TryAPIAddr()
				if err != nil {
					return err
				}

				api, err := NewApi(apiMaddr)
				if err != nil {
					return err
				}
				apis[i] = api

				// empty node is pinned even with --empty-repo, we don't want that
				emptyNode, err := path.NewPath("/ipfs/QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn")
				if err != nil {
					return err
				}

				if err := api.Pin().Rm(ctx, emptyNode); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				errsLk.Lock()
				errs = append(errs, err)
				errsLk.Unlock()
			}
		}(i, n)
	}

	wg.Wait()

	return apis, multierr.Combine(errs...)
}

func TestHttpApi(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("skipping due to #9905")
	}

	tests.TestApi(NodeProvider{})(t)
}

func Test_NewURLApiWithClient_With_Headers(t *testing.T) {
	t.Parallel()

	var (
		headerToTest        = "Test-Header"
		expectedHeaderValue = "thisisaheadertest"
	)
	ts := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			val := r.Header.Get(headerToTest)
			if val != expectedHeaderValue {
				w.WriteHeader(400)
				return
			}
			http.ServeContent(w, r, "", time.Now(), strings.NewReader("test"))
		}),
	)
	defer ts.Close()
	api, err := NewURLApiWithClient(ts.URL, &http.Client{
		Transport: &http.Transport{
			Proxy:             http.ProxyFromEnvironment,
			DisableKeepAlives: true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	api.Headers.Set(headerToTest, expectedHeaderValue)
	p, err := path.NewPath("/ipfs/QmS4ustL54uo8FzR9455qaxZwuMiUhyvMcX9Ba8nUH4uVv")
	if err != nil {
		t.Fatal(err)
	}
	if err := api.Pin().Rm(context.Background(), p); err != nil {
		t.Fatal(err)
	}
}

func Test_NewURLApiWithClient_HTTP_Variant(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		address  string
		expected string
	}{
		{address: "/ip4/127.0.0.1/tcp/80", expected: "http://127.0.0.1:80"},
		{address: "/ip4/127.0.0.1/tcp/443/tls", expected: "https://127.0.0.1:443"},
		{address: "/ip4/127.0.0.1/tcp/443/https", expected: "https://127.0.0.1:443"},
		{address: "/ip4/127.0.0.1/tcp/443/tls/http", expected: "https://127.0.0.1:443"},
	}

	for _, tc := range testcases {
		address, err := ma.NewMultiaddr(tc.address)
		if err != nil {
			t.Fatal(err)
		}

		api, err := NewApiWithClient(address, &http.Client{})
		if err != nil {
			t.Fatal(err)
		}

		if api.url != tc.expected {
			t.Errorf("Expected = %s; got %s", tc.expected, api.url)
		}
	}
}
