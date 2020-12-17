package migrations

import (
	"io/ioutil"
	"os"
	"path"
	"testing"
)

var (
	fakeHome string
	fakeIpfs string
)

func init() {
	CacheIpfsDir(false)
}

func TestRepoDir(t *testing.T) {
	var err error
	fakeHome, err = ioutil.TempDir("", "testhome")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(fakeHome)

	os.Setenv("HOME", fakeHome)
	fakeIpfs = path.Join(fakeHome, ".ipfs")

	t.Run("testFindIpfsDir", testFindIpfsDir)
	t.Run("testCheckIpfsDir", testCheckIpfsDir)
	t.Run("testRepoVersion", testRepoVersion)
}

func testFindIpfsDir(t *testing.T) {
	_, err := findIpfsDir()
	if err == nil {
		t.Fatal("expected error when no .ipfs directory to find")
	}

	err = os.Mkdir(fakeIpfs, os.ModePerm)
	if err != nil {
		panic(err)
	}

	dir, err := findIpfsDir()
	if err != nil {
		t.Fatal(err)
	}
	if dir != fakeIpfs {
		t.Fatal("wrong ipfs directory:", dir)
	}

	os.Setenv("IPFS_PATH", "~/.ipfs")
	dir, err = findIpfsDir()
	if err != nil {
		t.Fatal(err)
	}
	if dir != fakeIpfs {
		t.Fatal("wrong ipfs directory:", dir)
	}
}

func testCheckIpfsDir(t *testing.T) {
	_, err := checkIpfsDir("~/no_such_dir")
	if err == nil {
		t.Fatal("expected error from nonexistent directory")
	}

	dir, err := checkIpfsDir("~/.ipfs")
	if err != nil {
		t.Fatal(err)
	}
	if dir != fakeIpfs {
		t.Fatal("wrong ipfs directory:", dir)
	}
}

func testRepoVersion(t *testing.T) {
	ver, err := RepoVersion(fakeIpfs)
	if err != nil {
		t.Fatal(err)
	}
	if ver != 0 {
		t.Fatal("expected version 0 when no version file")
	}

	testVer := 42
	err = WriteRepoVersion(fakeIpfs, testVer)
	if err != nil {
		t.Fatal(err)
	}

	ver, err = RepoVersion(fakeIpfs)
	if err != nil {
		t.Fatal(err)
	}
	if ver != testVer {
		t.Fatalf("expected version %d, got %d", testVer, ver)
	}
}

func TestApiEndpoint(t *testing.T) {
	var err error
	fakeHome, err = ioutil.TempDir("", "testhome")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(fakeHome)
	defer os.Unsetenv("HOME")

	os.Setenv("HOME", fakeHome)
	fakeIpfs = path.Join(fakeHome, ".ipfs")

	err = os.Mkdir(fakeIpfs, os.ModePerm)
	if err != nil {
		panic(err)
	}

	_, err = ApiEndpoint("")
	if err == nil {
		t.Fatal("expected error when missing api file")
	}

	apiPath := path.Join(fakeIpfs, apiFile)
	err = ioutil.WriteFile(apiPath, []byte("bad-data"), 0644)
	if err != nil {
		panic(err)
	}

	_, err = ApiEndpoint("")
	if err == nil {
		t.Fatal("expected error when bad data")
	}

	err = ioutil.WriteFile(apiPath, []byte("/ip4/127.0.0.1/tcp/5001"), 0644)
	if err != nil {
		panic(err)
	}

	val, err := ApiEndpoint("")
	if err != nil {
		t.Fatal(err)
	}
	if val != "127.0.0.1:5001" {
		t.Fatal("got unexpected value:", val)
	}

	val2, err := ApiEndpoint(fakeIpfs)
	if err != nil {
		t.Fatal(err)
	}
	if val2 != val {
		t.Fatal("expected", val, "got", val2)
	}
}
