package migrations

import (
	"bufio"
	"context"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"
)

func TestHttpFetch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	url := gatewayURL + path.Join(ipfsDistPath, distFSRM, distVersions)
	rc, err := httpFetch(ctx, url)
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

	if len(out) < 14 {
		t.Fatal("do not get all expected data")
	}
	if out[0] != "v1.0.0" {
		t.Fatal("expected v1.0.0 as first line, got", out[0])
	}

	// Check bad URL
	url = gatewayURL + path.Join(ipfsDistPath, distFSRM, "no_such_file")
	_, err = httpFetch(ctx, url)
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatal("expected error 404")
	}
}

func TestIpfsFetch(t *testing.T) {
	_, err := ApiEndpoint("")
	if err != nil {
		t.Skip("skipped - local ipfs daemon not available")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	url := path.Join(ipfsDistPath, distFSRM, distVersions)
	rc, err := ipfsFetch(ctx, url)
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

	if len(out) < 14 {
		t.Fatal("do not get all expected data")
	}
	if out[0] != "v1.0.0" {
		t.Fatal("expected v1.0.0 as first line, got", out[0])
	}

	// Check bad URL
	url = path.Join(ipfsDistPath, distFSRM, "no_such_file")
	_, err = ipfsFetch(ctx, url)
	if err == nil || !strings.Contains(err.Error(), "no link") {
		t.Fatal("expected 'no link' error, got:", err)
	}
}

func TestFetchBinary(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "fetchtest")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	vers, err := DistVersions(ctx, distFSRM, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("latest version of", distFSRM, "is", vers[len(vers)-1])

	bin, err := FetchBinary(ctx, distFSRM, vers[0], distFSRM, distFSRM, tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	fi, err := os.Stat(bin)
	if os.IsNotExist(err) {
		t.Error("expected file to exist:", bin)
	}

	t.Log("downloaded and unpacked", fi.Size(), "byte file:", fi.Name())

	bin, err = FetchBinary(ctx, "go-ipfs", "v0.3.5", "go-ipfs", "ipfs", tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	fi, err = os.Stat(bin)
	if os.IsNotExist(err) {
		t.Error("expected file to exist:", bin)
	}

	t.Log("downloaded and unpacked", fi.Size(), "byte file:", fi.Name())
}
