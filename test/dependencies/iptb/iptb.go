package main

import (
	"fmt"
	"os"

	cli "gx/ipfs/QmYAXfidRkyrQH5sGVA71TAwL1cknsDtMoLPV6Bjk13VrG/iptb/cli"
	testbed "gx/ipfs/QmYAXfidRkyrQH5sGVA71TAwL1cknsDtMoLPV6Bjk13VrG/iptb/testbed"

	plugin "gx/ipfs/QmZJXRAhsC7Zi94udXXdsnncJLYdSYBAckWxbxHJe9fPG3/iptb-plugins/local"
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
