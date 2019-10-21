package filesystem

import (
	"context"
	"testing"

	"github.com/hugelgupf/p9/p9"
	plugin "github.com/ipfs/go-ipfs/plugin"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
)

var attrMaskIPFSTest = p9.AttrMask{
	Mode: true,
	Size: true,
}

func TestAll(t *testing.T) {
	ctx := context.TODO()

	core, err := InitCore(ctx)
	if err != nil {
		t.Fatalf("Failed to construct CoreAPI: %s\n", err)
	}

	t.Run("RootFS", func(t *testing.T) { testRootFS(ctx, t, core) })
	t.Run("PinFS", func(t *testing.T) { testPinFS(ctx, t, core) })
	t.Run("IPFS", func(t *testing.T) { testIPFS(ctx, t, core) })

	pluginEnv := &plugin.Environment{Config: defaultConfig()}
	t.Run("Plugin", func(t *testing.T) { testPlugin(t, pluginEnv, core) })
}

func testPlugin(t *testing.T, pluginEnv *plugin.Environment, core coreiface.CoreAPI) {
	// NOTE: all restrictive comments are in relation to our plugin, not all plugins
	var (
		module FileSystemPlugin
		err    error
	)

	// close and start before init are NOT allowed
	if err = module.Close(); err == nil {
		t.Fatal("plugin was not initialized but Close succeeded")
		// also should not hang
	}
	if err = module.Start(core); err == nil {
		t.Fatal("plugin was not initialized but Start succeeded")
		// also should not hang
	}

	// initialize the module
	if err = module.Init(pluginEnv); err != nil {
		t.Fatal("Plugin couldn't be initialized: ", err)
	}

	// double init is NOT allowed
	if err = module.Init(pluginEnv); err == nil {
		t.Fatal("init isn't intended to succeed twice")
	}

	// close before start is allowed
	if err = module.Close(); err != nil {
		t.Fatal("plugin isn't busy, but it can't close: ", err)
		// also should not hang
	}

	// double close is allowed
	if err = module.Close(); err != nil {
		t.Fatal("plugin couldn't close twice: ", err)
	}

	// start the module
	if err = module.Start(core); err != nil {
		t.Fatal("module could not start: ", err)
	}

	// double start is NOT allowed
	if err = module.Start(core); err == nil {
		t.Fatal("module is intended to be exclusive but was allowed to start twice")
	}

	// actual close
	if err = module.Close(); err != nil {
		t.Fatalf("plugin isn't busy, but it can't close: %#v", err)
		t.Fatal("plugin isn't busy, but it can't close: ", err)
	}

	// another redundant close
	if err = module.Close(); err != nil {
		t.Fatal("plugin isn't busy, but it can't close: ", err)
		// also should not hang
	}
}
