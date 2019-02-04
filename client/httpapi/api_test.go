package httpapi

import (
	"context"
	"fmt"
	"io/ioutil"
	gohttp "net/http"
	"os"
	"path"
	"strconv"
	"testing"

	"github.com/ipfs/go-ipfs/core/coreapi/interface"
	caopts "github.com/ipfs/go-ipfs/core/coreapi/interface/options"
	"github.com/ipfs/go-ipfs/core/coreapi/interface/tests"

	local "github.com/ipfs/iptb-plugins/local"
	"github.com/ipfs/iptb/cli"
	"github.com/ipfs/iptb/testbed"
	"github.com/ipfs/iptb/testbed/interfaces"
)

type NodeProvider struct{}

func (NodeProvider) MakeAPISwarm(ctx context.Context, fullIdentity bool, n int) ([]iface.CoreAPI, error) {
	_, err := testbed.RegisterPlugin(testbed.IptbPlugin{
		From:        "<builtin>",
		NewNode:     local.NewNode,
		GetAttrList: local.GetAttrList,
		GetAttrDesc: local.GetAttrDesc,
		PluginName:  local.PluginName,
		BuiltIn:     true,
	}, false)
	if err != nil {
		return nil, err
	}

	dir, err := ioutil.TempDir("", "httpapi-tb-")
	if err != nil {
		return nil, err
	}

	c := cli.NewCli() //TODO: is there a better way?

	initArgs := []string{"iptb", "--IPTB_ROOT", dir, "auto", "-type", "localipfs", "-count", strconv.FormatInt(int64(n), 10)}
	if err := c.Run(initArgs); err != nil {
		return nil, err
	}

	filestoreArgs := []string{"iptb", "--IPTB_ROOT", dir, "run", fmt.Sprintf("[0-%d]", n-1), "--", "ipfs", "config", "--json", "Experimental.FilestoreEnabled", "true"}
	if err := c.Run(filestoreArgs); err != nil {
		return nil, err
	}

	startArgs := []string{"iptb", "--IPTB_ROOT", dir, "start", "-wait", "--", "--enable-pubsub-experiment", "--offline=" + strconv.FormatBool(n == 1)}
	if err := c.Run(startArgs); err != nil {
		return nil, err
	}

	if n > 1 {
		connectArgs := []string{"iptb", "--IPTB_ROOT", dir, "connect", fmt.Sprintf("[1-%d]", n-1), "0"}
		if err := c.Run(connectArgs); err != nil {
			return nil, err
		}
	}

	go func() {
		<-ctx.Done()

		defer os.Remove(dir)

		defer func() {
			_ = c.Run([]string{"iptb", "--IPTB_ROOT", dir, "stop"})
		}()
	}()

	apis := make([]iface.CoreAPI, n)

	for i := range apis {
		tb := testbed.NewTestbed(path.Join(dir, "testbeds", "default"))

		node, err := tb.Node(i)
		if err != nil {
			return nil, err
		}

		attrNode, ok := node.(testbedi.Attribute)
		if !ok {
			return nil, fmt.Errorf("node does not implement attributes")
		}

		pth, err := attrNode.Attr("path")
		if err != nil {
			return nil, err
		}

		a := ApiAddr(pth)
		if a == nil {
			return nil, fmt.Errorf("nil addr for node")
		}
		c := &gohttp.Client{
			Transport: &gohttp.Transport{
				Proxy:              gohttp.ProxyFromEnvironment,
				DisableKeepAlives:  true,
				DisableCompression: true,
			},
		}
		apis[i] = NewApiWithClient(a, c)

		// node cleanup
		// TODO: pass --empty-repo somehow (how?)
		pins, err := apis[i].Pin().Ls(ctx, caopts.Pin.Type.Recursive())
		if err != nil {
			return nil, err
		}
		for _, pin := range pins { //TODO: parallel
			if err := apis[i].Pin().Rm(ctx, pin.Path()); err != nil {
				return nil, err
			}
		}

	}

	return apis, nil
}

func TestHttpApi(t *testing.T) {
	tests.TestApi(&NodeProvider{})(t)
}
