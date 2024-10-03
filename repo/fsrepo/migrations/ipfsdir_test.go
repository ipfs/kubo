package migrations

import (
	"os"
	"path/filepath"
	"testing"
)

var (
	fakeHome string
	fakeIpfs string
)

func TestRepoDir(t *testing.T) {
	fakeHome = t.TempDir()
	os.Setenv("HOME", fakeHome)
	fakeIpfs = filepath.Join(fakeHome, ".ipfs")

	t.Run("testIpfsDir", testIpfsDir)
	t.Run("testCheckIpfsDir", testCheckIpfsDir)
	t.Run("testRepoVersion", testRepoVersion)
}

func testIpfsDir(t *testing.T) {
	_, err := CheckIpfsDir("")
	if err == nil {
		t.Fatal("expected error when no .ipfs directory to find")
	}

	err = os.Mkdir(fakeIpfs, os.ModePerm)
	if err != nil {
		panic(err)
	}

	dir, err := IpfsDir("")
	if err != nil {
		t.Fatal(err)
	}
	if dir != fakeIpfs {
		t.Fatal("wrong ipfs directory:", dir)
	}

	os.Setenv(envIpfsPath, "~/.ipfs")
	dir, err = IpfsDir("")
	if err != nil {
		t.Fatal(err)
	}
	if dir != fakeIpfs {
		t.Fatal("wrong ipfs directory:", dir)
	}

	_, err = IpfsDir("~somesuer/foo")
	if err == nil {
		t.Fatal("expected error with user-specific home dir")
	}

	err = os.Setenv(envIpfsPath, "~somesuer/foo")
	if err != nil {
		panic(err)
	}
	_, err = IpfsDir("~somesuer/foo")
	if err == nil {
		t.Fatal("expected error with user-specific home dir")
	}
	err = os.Unsetenv(envIpfsPath)
	if err != nil {
		panic(err)
	}

	dir, err = IpfsDir("~/.ipfs")
	if err != nil {
		t.Fatal(err)
	}
	if dir != fakeIpfs {
		t.Fatal("wrong ipfs directory:", dir)
	}

	_, err = IpfsDir("")
	if err != nil {
		t.Fatal(err)
	}
}

func testCheckIpfsDir(t *testing.T) {
	_, err := CheckIpfsDir("~somesuer/foo")
	if err == nil {
		t.Fatal("expected error with user-specific home dir")
	}

	_, err = CheckIpfsDir("~/no_such_dir")
	if err == nil {
		t.Fatal("expected error from nonexistent directory")
	}

	dir, err := CheckIpfsDir("~/.ipfs")
	if err != nil {
		t.Fatal(err)
	}
	if dir != fakeIpfs {
		t.Fatal("wrong ipfs directory:", dir)
	}
}

func testRepoVersion(t *testing.T) {
	badDir := "~somesuer/foo"
	_, err := RepoVersion(badDir)
	if err == nil {
		t.Fatal("expected error with user-specific home dir")
	}

	_, err = RepoVersion(fakeIpfs)
	if !os.IsNotExist(err) {
		t.Fatal("expected not-exist error")
	}

	testVer := 42
	err = WriteRepoVersion(fakeIpfs, testVer)
	if err != nil {
		t.Fatal(err)
	}

	var ver int
	ver, err = RepoVersion(fakeIpfs)
	if err != nil {
		t.Fatal(err)
	}
	if ver != testVer {
		t.Fatalf("expected version %d, got %d", testVer, ver)
	}

	err = WriteRepoVersion(badDir, testVer)
	if err == nil {
		t.Fatal("expected error with user-specific home dir")
	}

	ipfsDir, err := IpfsDir(fakeIpfs)
	if err != nil {
		t.Fatal(err)
	}
	vFilePath := filepath.Join(ipfsDir, versionFile)
	err = os.WriteFile(vFilePath, []byte("bad-version-data\n"), 0o644)
	if err != nil {
		panic(err)
	}
	_, err = RepoVersion(fakeIpfs)
	if err == nil || err.Error() != "invalid data in repo version file" {
		t.Fatal("expected 'invalid data' error")
	}
	err = WriteRepoVersion(fakeIpfs, testVer)
	if err != nil {
		t.Fatal(err)
	}
}
