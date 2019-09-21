package main

import (
	"fmt"
	"os"

	"github.com/ipfs/go-ipfs/plugin"
)

var Plugins = []plugin.Plugin{
	&testPlugin{},
}

var _ = Plugins // used

type testPlugin struct{}

func (*testPlugin) Name() string {
	return "test-plugin"
}

func (*testPlugin) Version() string {
	return "0.1.0"
}

func (*testPlugin) Init(env *plugin.Environment) error {
	fmt.Fprintf(os.Stderr, "testplugin %s\n", env.Repo)
	fmt.Fprintf(os.Stderr, "testplugin %v\n", env.Config)
	return nil
}
