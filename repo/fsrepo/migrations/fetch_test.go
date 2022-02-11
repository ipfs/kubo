package migrations

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func createTestServer() *httptest.Server {
	reqHandler := func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if strings.Contains(r.URL.Path, "not-here") {
			http.NotFound(w, r)
		} else if strings.HasSuffix(r.URL.Path, "versions") {
			fmt.Fprint(w, "v1.0.0\nv1.1.0\nv1.1.2\nv2.0.0-rc1\n2.0.0\nv2.0.1\n")
		} else if strings.HasSuffix(r.URL.Path, ".tar.gz") {
			createFakeArchive(r.URL.Path, false, w)
		} else if strings.HasSuffix(r.URL.Path, "zip") {
			createFakeArchive(r.URL.Path, true, w)
		} else {
			http.NotFound(w, r)
		}
	}
	return httptest.NewServer(http.HandlerFunc(reqHandler))
}

func createFakeArchive(name string, archZip bool, w io.Writer) {
	fileName := strings.Split(path.Base(name), "_")[0]
	root := path.Base(path.Dir(path.Dir(name)))

	// Simulate fetching go-ipfs, which has "ipfs" as the name in the archive.
	if fileName == "go-ipfs" {
		fileName = "ipfs"
	}
	fileName = ExeName(fileName)

	var err error
	if archZip {
		err = writeZip(root, fileName, "FAKE DATA", w)
	} else {
		err = writeTarGzip(root, fileName, "FAKE DATA", w)
	}
	if err != nil {
		panic(err)
	}
}

func TestGetDistPath(t *testing.T) {
	os.Unsetenv(envIpfsDistPath)
	distPath := GetDistPathEnv("")
	if distPath != LatestIpfsDist {
		t.Error("did not set default dist path")
	}

	testDist := "/unit/test/dist"
	err := os.Setenv(envIpfsDistPath, testDist)
	if err != nil {
		panic(err)
	}
	defer func() {
		os.Unsetenv(envIpfsDistPath)
	}()

	distPath = GetDistPathEnv("")
	if distPath != testDist {
		t.Error("did not set dist path from environ")
	}
	distPath = GetDistPathEnv("ignored")
	if distPath != testDist {
		t.Error("did not set dist path from environ")
	}

	testDist = "/unit/test/dist2"
	fetcher := NewHttpFetcher(testDist, "", "", 0)
	if fetcher.distPath != testDist {
		t.Error("did not set dist path")
	}
}

func TestHttpFetch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ts := createTestServer()
	defer ts.Close()

	fetcher := NewHttpFetcher("", ts.URL, "", 0)

	out, err := fetcher.Fetch(ctx, "/versions")
	if err != nil {
		t.Fatal(err)
	}

	var lines []string
	scan := bufio.NewScanner(bytes.NewReader(out))
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
	if lines[0] != "v1.0.0" {
		t.Fatal("expected v1.0.0 as first line, got", lines[0])
	}

	// Check not found
	_, err = fetcher.Fetch(ctx, "/no_such_file")
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatal("expected error 404")
	}
}

func TestFetchBinary(t *testing.T) {
	tmpDir := t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ts := createTestServer()
	defer ts.Close()

	fetcher := NewHttpFetcher("", ts.URL, "", 0)

	vers, err := DistVersions(ctx, fetcher, distFSRM, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("latest version of", distFSRM, "is", vers[len(vers)-1])

	bin, err := FetchBinary(ctx, fetcher, distFSRM, vers[0], "", tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	fi, err := os.Stat(bin)
	if os.IsNotExist(err) {
		t.Error("expected file to exist:", bin)
	}

	t.Log("downloaded and unpacked", fi.Size(), "byte file:", fi.Name())

	bin, err = FetchBinary(ctx, fetcher, "go-ipfs", "v0.3.5", "ipfs", tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	fi, err = os.Stat(bin)
	if os.IsNotExist(err) {
		t.Error("expected file to exist:", bin)
	}

	t.Log("downloaded and unpacked", fi.Size(), "byte file:", fi.Name())

	// Check error is destination already exists and is not directory
	_, err = FetchBinary(ctx, fetcher, "go-ipfs", "v0.3.5", "ipfs", bin)
	if !os.IsExist(err) {
		t.Fatal("expected 'exists' error, got", err)
	}

	_, err = FetchBinary(ctx, fetcher, "go-ipfs", "v0.3.5", "ipfs", tmpDir)
	if !os.IsExist(err) {
		t.Error("expected 'exists' error, got:", err)
	}

	os.Remove(filepath.Join(tmpDir, ExeName("ipfs")))

	// Check error creating temp download directory
	//
	// Windows doesn't have read-only directories https://github.com/golang/go/issues/35042 this would need to be
	// tested another way
	if runtime.GOOS != "windows" {
		err = os.Chmod(tmpDir, 0555)
		if err != nil {
			panic(err)
		}
		err = os.Setenv("TMPDIR", tmpDir)
		if err != nil {
			panic(err)
		}
		_, err = FetchBinary(ctx, fetcher, "go-ipfs", "v0.3.5", "ipfs", tmpDir)
		if !os.IsPermission(err) {
			t.Error("expected 'permission' error, got:", err)
		}
		err = os.Setenv("TMPDIR", "/tmp")
		if err != nil {
			panic(err)
		}
		err = os.Chmod(tmpDir, 0755)
		if err != nil {
			panic(err)
		}
	}

	// Check error if failure to fetch due to bad dist
	_, err = FetchBinary(ctx, fetcher, "not-here", "v0.3.5", "ipfs", tmpDir)
	if err == nil || !strings.Contains(err.Error(), "Not Found") {
		t.Error("expected 'Not Found' error, got:", err)
	}

	// Check error if failure to unpack archive
	_, err = FetchBinary(ctx, fetcher, "go-ipfs", "v0.3.5", "not-such-bin", tmpDir)
	if err == nil || err.Error() != "no binary found in archive" {
		t.Error("expected 'no binary found in archive' error")
	}
}

func TestMultiFetcher(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ts := createTestServer()
	defer ts.Close()

	badFetcher := NewHttpFetcher("", "bad-url", "", 0)
	fetcher := NewHttpFetcher("", ts.URL, "", 0)

	mf := NewMultiFetcher(badFetcher, fetcher)

	vers, err := mf.Fetch(ctx, "/versions")
	if err != nil {
		t.Fatal(err)
	}

	if len(vers) < 45 {
		fmt.Println("unexpected more data")
	}
}
