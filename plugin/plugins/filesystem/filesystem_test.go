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
		t.Logf("Failed to construct CoreAPI: %s\n", err)
		t.FailNow()
	}

	t.Run("RootFS", func(t *testing.T) { testRootFS(ctx, t, core) })
	t.Run("PinFS", func(t *testing.T) { testPinFS(ctx, t, core) })
	t.Run("IPFS", func(t *testing.T) { testIPFS(ctx, t, core) })
	t.Run("MFS", func(t *testing.T) { testMFS(ctx, t, core) })
	t.Run("IPNS", func(t *testing.T) { testIPNS(ctx, t, core) })

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
		t.Logf("plugin was not initialized but Close succeeded")
		t.FailNow()
		// also should not hang
	}
	if err = module.Start(core); err == nil {
		t.Logf("plugin was not initialized but Start succeeded")
		t.FailNow()
		// also should not hang
	}

	// initialize the module
	if err = module.Init(pluginEnv); err != nil {
		t.Logf("Plugin couldn't be initialized: %s", err)
		t.FailNow()
	}

	// double init is NOT allowed
	if err = module.Init(pluginEnv); err == nil {
		t.Logf("init isn't intended to succeed twice")
		t.FailNow()
	}

	// close before start is allowed
	if err = module.Close(); err != nil {
		t.Logf("plugin isn't busy, but it can't close: %s", err)
		t.FailNow()
		// also should not hang
	}

	// double close is allowed
	if err = module.Close(); err != nil {
		t.Logf("plugin couldn't close twice: %s", err)
		t.FailNow()
	}

	// start the module
	if err = module.Start(core); err != nil {
		t.Logf("module could not start: %s", err)
		t.FailNow()
	}

	// double start is NOT allowed
	if err = module.Start(core); err == nil {
		t.Logf("module is intended to be exclusive but was allowed to start twice")
		t.FailNow()
	}

	// actual close
	if err = module.Close(); err != nil {
		t.Logf("plugin isn't busy, but it can't close: %s", err)
		t.FailNow()
	}

	// another redundant close
	if err = module.Close(); err != nil {
		t.Logf("plugin isn't busy, but it can't close: %s", err)
		t.FailNow()
		// also should not hang
	}
}
