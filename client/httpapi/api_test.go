package httpapi

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"testing"

	"github.com/ipfs/go-ipfs/core/coreapi/interface"
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

	c := cli.NewCli()

	initArgs := []string{"iptb", "--IPTB_ROOT", dir, "auto", "-type", "localipfs", "-count", strconv.FormatInt(int64(n), 10)}
	if err := c.Run(initArgs); err != nil {
		return nil, err
	}

	startArgs := []string{"iptb", "--IPTB_ROOT", dir, "start", "-wait", "--", "--offline=" + strconv.FormatBool(n == 1)}
	if err := c.Run(startArgs); err != nil {
		return nil, err
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

		apis[i] = NewPathApi(pth)
	}

	return apis, nil
}

func TestHttpApi(t *testing.T) {
	tests.TestApi(&NodeProvider{})(t)
}
