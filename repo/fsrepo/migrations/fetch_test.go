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

func TestSetIpfsDistPath(t *testing.T) {
	os.Unsetenv(envIpfsDistPath)
	SetIpfsDistPath("")
	if ipfsDistPath != ipfsDist {
		t.Error("did not set default dist path")
	}

	testDist := "/unit/test/dist"
	err := os.Setenv(envIpfsDistPath, testDist)
	if err != nil {
		panic(err)
	}
	defer func() {
		os.Unsetenv(envIpfsDistPath)
		SetIpfsDistPath("")
	}()

	SetIpfsDistPath("")
	if ipfsDistPath != testDist {
		t.Error("did not set dist path from environ")
	}

	testDist = "/unit/test/dist2"
	SetIpfsDistPath(testDist)
	if ipfsDistPath != testDist {
		t.Error("did not set dist path")
	}
}

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
	_, err = httpFetch(ctx, "")
	if err == nil {
		t.Fatal("expected error")
	}

	// Check unreachable URL
	_, err = httpFetch(ctx, "http://127.0.0.123:65510")
	if err == nil || !strings.HasSuffix(err.Error(), "connection refused") {
		t.Fatal("expected 'connection refused' error")
	}

	// Check not found
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

	bin, err := FetchBinary(ctx, distFSRM, vers[0], distFSRM, "", tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	fi, err := os.Stat(bin)
	if os.IsNotExist(err) {
		t.Error("expected file to exist:", bin)
	}

	t.Log("downloaded and unpacked", fi.Size(), "byte file:", fi.Name())

	bin, err = FetchBinary(ctx, "go-ipfs", "v0.3.5", "", "ipfs", tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	fi, err = os.Stat(bin)
	if os.IsNotExist(err) {
		t.Error("expected file to exist:", bin)
	}

	t.Log("downloaded and unpacked", fi.Size(), "byte file:", fi.Name())

	// Check error is destination already exists and is not directory
	_, err = FetchBinary(ctx, "go-ipfs", "v0.3.5", "", "ipfs", bin)
	if !os.IsExist(err) {
		t.Fatal("expected 'exists' error")
	}

	// Check error creating temp download directory
	err = os.Chmod(tmpDir, 0555)
	if err != nil {
		panic(err)
	}
	err = os.Setenv("TMPDIR", tmpDir)
	if err != nil {
		panic(err)
	}
	_, err = FetchBinary(ctx, "go-ipfs", "v0.3.5", "", "ipfs", tmpDir)
	if !os.IsPermission(err) {
		t.Error("expected 'permission'error")
	}
	err = os.Setenv("TMPDIR", "/tmp")
	if err != nil {
		panic(err)
	}
	err = os.Chmod(tmpDir, 0755)
	if err != nil {
		panic(err)
	}

	// Check error if failure to fetch due to bad dist
	_, err = FetchBinary(ctx, "no-such-dist", "v0.3.5", "", "ipfs", tmpDir)
	if err == nil || !strings.Contains(err.Error(), "Not Found") {
		t.Error("expected 'Not Found' error")
	}

	// Check error if failure to unpack archive
	_, err = FetchBinary(ctx, "go-ipfs", "v0.3.5", "", "not-such-bin", tmpDir)
	if err == nil || err.Error() != "no binary found in archive" {
		t.Error("expected 'no binary found in archive' error")
	}
}
