package loader

import (
	"github.com/ipfs/go-ipfs/core/coredag"
	"github.com/ipfs/go-ipfs/plugin"
	"gx/ipfs/QmWLWmRVSiagqP15jczsGME1qpob6HDbtbHAY2he9W5iUo/opentracing-go"

	ipld "gx/ipfs/QmX5CsuHyVZeTLxgRSYkgLSDQKb9UjE8xnhQzCEJWWWFsC/go-ipld-format"
)

func initialize(plugins []plugin.Plugin) error {
	for _, p := range plugins {
		err := p.Init()
		if err != nil {
			return err
		}
	}

	return nil
}

func run(plugins []plugin.Plugin) error {
	for _, pl := range plugins {
		switch pl := pl.(type) {
		case plugin.PluginIPLD:
			err := runIPLDPlugin(pl)
			if err != nil {
				return err
			}
		case plugin.PluginTracer:
			err := runTracerPlugin(pl)
			if err != nil {
				return err
			}
		default:
			panic(pl)
		}
	}
	return nil
}

func runIPLDPlugin(pl plugin.PluginIPLD) error {
	err := pl.RegisterBlockDecoders(ipld.DefaultBlockDecoder)
	if err != nil {
		return err
	}
	return pl.RegisterInputEncParsers(coredag.DefaultInputEncParsers)
}

func runTracerPlugin(pl plugin.PluginTracer) error {
	tracer, err := pl.InitTracer()
	if err != nil {
		return err
	}
	opentracing.SetGlobalTracer(tracer)
	return nil
}
