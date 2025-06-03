package git

import (
	"compress/zlib"
	"io"

	"github.com/ipfs/kubo/plugin"

	// Note that depending on this package registers it's multicodec encoder and decoder.
	git "github.com/ipfs/go-ipld-git"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/multicodec"
	mc "github.com/multiformats/go-multicodec"
)

// Plugins is exported list of plugins that will be loaded.
var Plugins = []plugin.Plugin{
	&gitPlugin{},
}

type gitPlugin struct{}

var _ plugin.PluginIPLD = (*gitPlugin)(nil)

func (*gitPlugin) Name() string {
	return "ipld-git"
}

func (*gitPlugin) Version() string {
	return "0.0.1"
}

func (*gitPlugin) Init(_ *plugin.Environment) error {
	return nil
}

func (*gitPlugin) Register(reg multicodec.Registry) error {
	// register a custom identifier in the reserved range for import of "zlib-encoded git objects."
	reg.RegisterDecoder(uint64(mc.ReservedStart+mc.GitRaw), decodeZlibGit)
	reg.RegisterEncoder(uint64(mc.GitRaw), git.Encode)
	reg.RegisterDecoder(uint64(mc.GitRaw), git.Decode)
	return nil
}

func decodeZlibGit(na ipld.NodeAssembler, r io.Reader) error {
	rc, err := zlib.NewReader(r)
	if err != nil {
		return err
	}

	defer rc.Close()

	return git.Decode(na, rc)
}
