package ipfsfetcher

import (
	"bufio"
	"context"
	"flag"
	"testing"
)

var runIpfsTest = flag.Bool("ipfstest", false, "Run IpfsFetcher tests")

func TestIpfsFetcher(t *testing.T) {
	if !*runIpfsTest {
		t.Skip("manually-run dev test, use the '-ipfstest' flag to run")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fetcher := NewIpfsFetcher("", 0, nil)
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

	f := NewIpfsFetcher("", 0, nil)
	defer f.Close()

	// Init ipfs repo
	f.ipfsTmpDir, f.openErr = initTempNode(ctx)
	if f.openErr != nil {
		t.Fatalf("failed to init ipfs node: %s", f.openErr)
	}

	// Start ipfs node
	f.openErr = f.startTempNode(ctx, nil)
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
