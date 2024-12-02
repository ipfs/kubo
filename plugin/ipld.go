package plugin

import (
	multicodec "github.com/ipld/go-ipld-prime/multicodec"
)

// PluginIPLD is an interface that can be implemented to add handlers for
// for different IPLD codecs.
type PluginIPLD interface {
	Plugin

	Register(multicodec.Registry) error
}
