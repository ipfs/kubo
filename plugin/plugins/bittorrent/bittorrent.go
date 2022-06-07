package bittorrent

import (
	"github.com/ipfs/go-ipfs/plugin"
	// Note that depending on this package registers it's multicodec encoder and decoder.
	bencodeipld "github.com/aschmahmann/go-ipld-bittorrent/bencode"
	bittorrentipld "github.com/aschmahmann/go-ipld-bittorrent/bittorrent"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/multicodec"
	mc "github.com/multiformats/go-multicodec"
)

// Plugins is exported list of plugins that will be loaded
var Plugins = []plugin.Plugin{
	&bittorrent{},
}

type bittorrent struct{}

var _ plugin.PluginIPLD = (*bittorrent)(nil)
var _ plugin.PluginIPLDADL = (*bittorrent)(nil)

func (*bittorrent) Name() string {
	return "ipld-bittorrent"
}

func (*bittorrent) Version() string {
	return "0.0.1"
}

func (*bittorrent) Init(_ *plugin.Environment) error {
	return nil
}

func (*bittorrent) Register(reg multicodec.Registry) error {
	reg.RegisterEncoder(uint64(mc.Bencode), bencodeipld.Encode)
	reg.RegisterDecoder(uint64(mc.Bencode), bencodeipld.Decode)
	return nil
}

func (b *bittorrent) RegisterADL(m map[string]ipld.NodeReifier) error {
	const adlName = "bittorrentv1-file"
	m[adlName] = bittorrentipld.ReifyBTFile
	return nil
}
