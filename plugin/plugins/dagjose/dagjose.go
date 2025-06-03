package dagjose

import (
	"github.com/ipfs/kubo/plugin"

	"github.com/ceramicnetwork/go-dag-jose/dagjose"
	"github.com/ipld/go-ipld-prime/multicodec"
	mc "github.com/multiformats/go-multicodec"
)

// Plugins is exported list of plugins that will be loaded.
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
	reg.RegisterEncoder(uint64(mc.DagJose), dagjose.Encode)
	reg.RegisterDecoder(uint64(mc.DagJose), dagjose.Decode)
	return nil
}
