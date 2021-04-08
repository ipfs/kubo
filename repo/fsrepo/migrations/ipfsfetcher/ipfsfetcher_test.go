package ipfsfetcher

import (
	"bufio"
	"context"
	"testing"
)

func TestIpfsFetcher(t *testing.T) {
	//t.Skip("manually-run dev test only")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fetcher := NewIpfsFetcher("", 0, nil)
	defer fetcher.Close()

	rc, err := fetcher.Fetch(ctx, "go-ipfs/versions")
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()

	var out []string
	scan := bufio.NewScanner(rc)
	for scan.Scan() {
		out = append(out, scan.Text())
	}
	err = scan.Err()
	if err != nil {
		t.Fatal("could not read versions:", err)
	}

	if len(out) < 6 {
		t.Fatal("do not get all expected data")
	}
	if out[0] != "v0.3.2" {
		t.Fatal("expected v1.0.0 as first line, got", out[0])
	}

	// Check not found
	_, err = fetcher.Fetch(ctx, "/no_such_file")
	if err == nil {
		t.Fatal("expected error 404")
	}

}
