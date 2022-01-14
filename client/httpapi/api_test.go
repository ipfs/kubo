package httpapi

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	iface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/path"

	"github.com/ipfs/interface-go-ipfs-core/tests"
	local "github.com/ipfs/iptb-plugins/local"
	"github.com/ipfs/iptb/testbed"
	testbedi "github.com/ipfs/iptb/testbed/interfaces"
	ma "github.com/multiformats/go-multiaddr"
)

const parallelSpeculativeNodes = 15 // 15 seems to work best

func init() {
	_, err := testbed.RegisterPlugin(testbed.IptbPlugin{
		From:        "<builtin>",
		NewNode:     local.NewNode,
		GetAttrList: local.GetAttrList,
		GetAttrDesc: local.GetAttrDesc,
		PluginName:  local.PluginName,
		BuiltIn:     true,
	}, false)
	if err != nil {
		panic(err)
	}
}

type NodeProvider struct {
	simple <-chan func(context.Context) ([]iface.CoreAPI, error)
}

func newNodeProvider(ctx context.Context) *NodeProvider {
	simpleNodes := make(chan func(context.Context) ([]iface.CoreAPI, error), parallelSpeculativeNodes)

	np := &NodeProvider{
		simple: simpleNodes,
	}

	// start basic nodes speculatively in parallel
	for i := 0; i < parallelSpeculativeNodes; i++ {
		go func() {
			for {
				ctx, cancel := context.WithCancel(ctx)

				snd, err := np.makeAPISwarm(ctx, false, 1)

				res := func(ctx context.Context) ([]iface.CoreAPI, error) {
					if err != nil {
						return nil, err
					}

					go func() {
						<-ctx.Done()
						cancel()
					}()

					return snd, nil
				}

				select {
				case simpleNodes <- res:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	return np
}

func (np *NodeProvider) MakeAPISwarm(ctx context.Context, fullIdentity bool, n int) ([]iface.CoreAPI, error) {
	if !fullIdentity && n == 1 {
		return (<-np.simple)(ctx)
	}
	return np.makeAPISwarm(ctx, fullIdentity, n)
}

func (NodeProvider) makeAPISwarm(ctx context.Context, fullIdentity bool, n int) ([]iface.CoreAPI, error) {

	dir, err := ioutil.TempDir("", "httpapi-tb-")
	if err != nil {
		return nil, err
	}

	tb := testbed.NewTestbed(dir)

	specs, err := testbed.BuildSpecs(tb.Dir(), n, "localipfs", nil)
	if err != nil {
		return nil, err
	}

	if err := testbed.WriteNodeSpecs(tb.Dir(), specs); err != nil {
		return nil, err
	}

	nodes, err := tb.Nodes()
	if err != nil {
		return nil, err
	}

	apis := make([]iface.CoreAPI, n)

	wg := sync.WaitGroup{}
	zero := sync.WaitGroup{}

	wg.Add(len(nodes))
	zero.Add(1)
	errs := make(chan error, len(nodes))

	for i, nd := range nodes {
		go func(i int, nd testbedi.Core) {
			defer wg.Done()

			if _, err := nd.Init(ctx, "--empty-repo"); err != nil {
				errs <- err
				return
			}

			if _, err := nd.RunCmd(ctx, nil, "ipfs", "config", "--json", "Experimental.FilestoreEnabled", "true"); err != nil {
				errs <- err
				return
			}

			if _, err := nd.Start(ctx, true, "--enable-pubsub-experiment", "--offline="+strconv.FormatBool(n == 1)); err != nil {
				errs <- err
				return
			}

			if i > 0 {
				zero.Wait()
				if err := nd.Connect(ctx, nodes[0]); err != nil {
					errs <- err
					return
				}
			} else {
				zero.Done()
			}

			addr, err := nd.APIAddr()
			if err != nil {
				errs <- err
				return
			}

			maddr, err := ma.NewMultiaddr(addr)
			if err != nil {
				errs <- err
				return
			}

			c := &http.Client{
				Transport: &http.Transport{
					Proxy:              http.ProxyFromEnvironment,
					DisableKeepAlives:  true,
					DisableCompression: true,
				},
			}
			apis[i], err = NewApiWithClient(maddr, c)
			if err != nil {
				errs <- err
				return
			}

			// empty node is pinned even with --empty-repo, we don't want that
			emptyNode := path.New("/ipfs/QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn")
			if err := apis[i].Pin().Rm(ctx, emptyNode); err != nil {
				errs <- err
				return
			}
		}(i, nd)
	}

	wg.Wait()

	go func() {
		<-ctx.Done()

		defer os.Remove(dir)

		defer func() {
			for _, nd := range nodes {
				_ = nd.Stop(context.Background())
			}
		}()
	}()

	select {
	case err = <-errs:
	default:
	}

	return apis, err
}

func TestHttpApi(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping due to #142")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tests.TestApi(newNodeProvider(ctx))(t)
}

func Test_NewURLApiWithClient_With_Headers(t *testing.T) {
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
	if err := api.Pin().Rm(context.Background(), path.New("/ipfs/QmS4ustL54uo8FzR9455qaxZwuMiUhyvMcX9Ba8nUH4uVv")); err != nil {
		t.Fatal(err)
	}
}
