package httpapi

import (
	"context"
	"io/ioutil"
	gohttp "net/http"
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/tests"
	local "github.com/ipfs/iptb-plugins/local"
	"github.com/ipfs/iptb/testbed"
	"github.com/ipfs/iptb/testbed/interfaces"
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

	for i, nd := range nodes {
		go func(i int, nd testbedi.Core) {
			defer wg.Done()

			if _, err := nd.Init(ctx, "--empty-repo"); err != nil {
				panic(err)
			}

			if _, err := nd.RunCmd(ctx, nil, "ipfs", "config", "--json", "Experimental.FilestoreEnabled", "true"); err != nil {
				panic(err)
			}

			if _, err := nd.Start(ctx, true, "--enable-pubsub-experiment", "--offline="+strconv.FormatBool(n == 1)); err != nil {
				panic(err)
			}

			if i > 0 {
				zero.Wait()
				if err := nd.Connect(ctx, nodes[0]); err != nil {
					panic(err)
				}
			} else {
				zero.Done()
			}

			addr, err := nd.APIAddr()
			if err != nil {
				panic(err)
			}

			maddr, err := ma.NewMultiaddr(addr)
			if err != nil {
				panic(err)
			}

			c := &gohttp.Client{
				Transport: &gohttp.Transport{
					Proxy:              gohttp.ProxyFromEnvironment,
					DisableKeepAlives:  true,
					DisableCompression: true,
				},
			}
			apis[i], err = NewApiWithClient(maddr, c)
			if err != nil {
				panic(err)
			}

			// empty node is pinned even with --empty-repo, we don't want that
			emptyNode, err := iface.ParsePath("/ipfs/QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn")
			if err != nil {
				panic(err)
			}
			if err := apis[i].Pin().Rm(ctx, emptyNode); err != nil {
				panic(err)
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

	return apis, nil
}

func TestHttpApi(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tests.TestApi(newNodeProvider(ctx))(t)
}
