package plugin

import (
	"gx/ipfs/QmWLWmRVSiagqP15jczsGME1qpob6HDbtbHAY2he9W5iUo/opentracing-go"
)

// PluginTracer is an interface that can be implemented to add a tracer
type PluginTracer interface {
	Plugin
	InitTracer() (opentracing.Tracer, error)
}
