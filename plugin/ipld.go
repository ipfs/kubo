package plugin

import (
	"github.com/ipfs/go-ipfs/core/coredag"

	ipld "gx/ipfs/QmRL22E4paat7ky7vx9MLpR97JHHbFPrg3ytFQw6qp1y1s/go-ipld-format"
)

// PluginIPLD is an interface that can be implemented to add handlers for
// for different IPLD formats
type PluginIPLD interface {
	Plugin

	RegisterBlockDecoders(dec ipld.BlockDecoder) error
	RegisterInputEncParsers(iec coredag.InputEncParsers) error
}
