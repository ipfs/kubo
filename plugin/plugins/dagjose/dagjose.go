package dagjose

import (
	"io"
	
	"github.com/ipfs/go-ipfs/plugin"

	"github.com/ceramicnetwork/go-dag-jose/dagjose"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/multicodec"
	mc "github.com/multiformats/go-multicodec"
)

// Plugins is exported list of plugins that will be loaded
var Plugins = []plugin.Plugin{
	&dagjosePlugin{},
}

type dagjosePlugin struct{}

var _ plugin.PluginIPLD = (*dagjosePlugin)(nil)

func (*dagjosePlugin) Name() string {
	return "ipld-codec-dagjose"
}

func (*dagjosePlugin) Version() string {
	return "0.0.1"
}

func (*dagjosePlugin) Init(_ *plugin.Environment) error {
	return nil
}

func (*dagjosePlugin) Register(reg multicodec.Registry) error {
	reg.RegisterEncoder(uint64(mc.DagJose), func(ipld.Node, io.Writer) error {
		// TODO: the upstream should have a `dagjose.Encode` method, and that should be what we use here.
		// (This whole anonymous function shouldn't be needed -- we should just be able to drop that function right into the RegisterEncoder call.)
		// This next line is a dummy, placeholder call to force linking to happen, but does not do logically correct things.
		dagjose.LoadJOSE(nil, ipld.LinkContext{}, ipld.LinkSystem{})
		return nil
	})
	reg.RegisterDecoder(uint64(mc.DagJose), func(ipld.NodeAssembler, io.Reader) error {
		// TODO: the upstream should have a `dagjose.Decode` method, and that should be what we use here.
		// (This whole anonymous function shouldn't be needed -- we should just be able to drop that function right into the RegisterEncoder call.)
		// This next line is a dummy, placeholder call to force linking to happen, but does not do logically correct things.
		dagjose.StoreJOSE(ipld.LinkContext{}, nil, ipld.LinkSystem{})
		return nil
	})
	return nil
}
