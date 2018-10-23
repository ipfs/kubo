package main

import (
	"fmt"
	"os"

	cli "gx/ipfs/QmU5w6sBozzDcfHXuKn1ZZAYuBw1rE57YYRVxgUcCjEX8C/iptb/cli"
	plugin "gx/ipfs/QmU5w6sBozzDcfHXuKn1ZZAYuBw1rE57YYRVxgUcCjEX8C/iptb/plugins/ipfs/local"
	testbed "gx/ipfs/QmU5w6sBozzDcfHXuKn1ZZAYuBw1rE57YYRVxgUcCjEX8C/iptb/testbed"
)

func init() {
	_, err := testbed.RegisterPlugin(testbed.IptbPlugin{
		From:        "<builtin>",
		NewNode:     plugin.NewNode,
		GetAttrList: plugin.GetAttrList,
		GetAttrDesc: plugin.GetAttrDesc,
		PluginName:  plugin.PluginName,
		BuiltIn:     true,
	}, false)

	if err != nil {
		panic(err)
	}
}

func main() {
	cli := cli.NewCli()
	if err := cli.Run(os.Args); err != nil {
		fmt.Fprintf(cli.ErrWriter, "%s\n", err)
		os.Exit(1)
	}
}
