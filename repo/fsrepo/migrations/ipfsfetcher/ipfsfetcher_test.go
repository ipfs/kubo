package ipfsfetcher

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ipfs/go-ipfs/plugin/loader"
	"github.com/ipfs/go-ipfs/repo/fsrepo/migrations"
)

func init() {
	err := setupPlugins()
	if err != nil {
		panic(err)
	}
}

func TestIpfsFetcher(t *testing.T) {
	skipUnlessEpic(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fetcher := NewIpfsFetcher("", 0, nil, nil)
	defer fetcher.Close()

	rc, err := fetcher.Fetch(ctx, "go-ipfs/versions")
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()

	var lines []string
	scan := bufio.NewScanner(rc)
	for scan.Scan() {
		lines = append(lines, scan.Text())
	}
	err = scan.Err()
	if err != nil {
		t.Fatal("could not read versions:", err)
	}

	if len(lines) < 6 {
		t.Fatal("do not get all expected data")
	}
	if lines[0] != "v0.3.2" {
		t.Fatal("expected v1.0.0 as first line, got", lines[0])
	}

	// Check not found
	_, err = fetcher.Fetch(ctx, "/no_such_file")
	if err == nil {
		t.Fatal("expected error 404")
	}

}

func TestInitIpfsFetcher(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	f := NewIpfsFetcher("", 0, nil, nil)
	defer f.Close()

	// Init ipfs repo
	f.ipfsTmpDir, f.openErr = initTempNode(ctx, f.bootstrap, f.peers)
	if f.openErr != nil {
		t.Fatalf("failed to initialize ipfs node: %s", f.openErr)
	}

	// Start ipfs node
	f.openErr = f.startTempNode(ctx)
	if f.openErr != nil {
		t.Errorf("failed to start ipfs node: %s", f.openErr)
		return
	}

	var stopFuncCalled bool
	stopFunc := f.ipfsStopFunc
	f.ipfsStopFunc = func() {
		stopFuncCalled = true
		stopFunc()
	}

	addrInfo := f.AddrInfo()
	if string(addrInfo.ID) == "" {
		t.Error("AddInfo ID not set")
	}
	if len(addrInfo.Addrs) == 0 {
		t.Error("AddInfo Addrs not set")
	}
	t.Log("Temp node listening on:", addrInfo.Addrs)

	err := f.Close()
	if err != nil {
		t.Fatalf("failed to close fetcher: %s", err)
	}

	if stopFunc != nil && !stopFuncCalled {
		t.Error("Close did not call stop function")
	}

	err = f.Close()
	if err != nil {
		t.Fatalf("failed to close fetcher 2nd time: %s", err)
	}
}

func skipUnlessEpic(t *testing.T) {
	if os.Getenv("IPFS_EPIC_TEST") == "" {
		t.SkipNow()
	}
}

func setupPlugins() error {
	defaultPath, err := migrations.IpfsDir("")
	if err != nil {
		return err
	}

	// Load plugins. This will skip the repo if not available.
	plugins, err := loader.NewPluginLoader(filepath.Join(defaultPath, "plugins"))
	if err != nil {
		return fmt.Errorf("error loading plugins: %w", err)
	}

	if err := plugins.Initialize(); err != nil {
		// Need to ignore errors here because plugins may already be loaded when
		// run from ipfs daemon.
		return fmt.Errorf("error initializing plugins: %w", err)
	}

	if err := plugins.Inject(); err != nil {
		// Need to ignore errors here because plugins may already be loaded when
		// run from ipfs daemon.
		return fmt.Errorf("error injecting plugins: %w", err)
	}

	return nil
}
