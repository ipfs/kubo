package plugin

import (
	"github.com/ipld/go-ipld-prime"
	multicodec "github.com/ipld/go-ipld-prime/multicodec"
)

// PluginIPLD is an interface that can be implemented to add handlers for
// for different IPLD codecs
type PluginIPLD interface {
	Plugin

	Register(multicodec.Registry) error
}

type PluginIPLDADL interface {
	Plugin

	RegisterADL(map[string]ipld.NodeReifier) error
}
