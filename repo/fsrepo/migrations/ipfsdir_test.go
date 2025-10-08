package migrations

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ipfs/kubo/config"
)

func TestRepoDir(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	// On Windows, os.UserHomeDir() uses USERPROFILE, not HOME
	t.Setenv("USERPROFILE", fakeHome)
	fakeIpfs := filepath.Join(fakeHome, ".ipfs")
	t.Setenv(config.EnvDir, fakeIpfs)

	t.Run("testIpfsDir", func(t *testing.T) {
		testIpfsDir(t, fakeIpfs)
	})
	t.Run("testCheckIpfsDir", func(t *testing.T) {
		testCheckIpfsDir(t, fakeIpfs)
	})
	t.Run("testRepoVersion", func(t *testing.T) {
		testRepoVersion(t, fakeIpfs)
	})
}

func testIpfsDir(t *testing.T, fakeIpfs string) {
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
		t.Fatalf("wrong ipfs directory: got %s, expected %s", dir, fakeIpfs)
	}

	t.Setenv(config.EnvDir, "~/.ipfs")
	dir, err = IpfsDir("")
	if err != nil {
		t.Fatal(err)
	}
	if dir != fakeIpfs {
		t.Fatalf("wrong ipfs directory: got %s, expected %s", dir, fakeIpfs)
	}

	_, err = IpfsDir("~somesuer/foo")
	if err == nil {
		t.Fatal("expected error with user-specific home dir")
	}

	t.Setenv(config.EnvDir, "~somesuer/foo")
	_, err = IpfsDir("~somesuer/foo")
	if err == nil {
		t.Fatal("expected error with user-specific home dir")
	}
	err = os.Unsetenv(config.EnvDir)
	if err != nil {
		panic(err)
	}

	dir, err = IpfsDir("~/.ipfs")
	if err != nil {
		t.Fatal(err)
	}
	if dir != fakeIpfs {
		t.Fatalf("wrong ipfs directory: got %s, expected %s", dir, fakeIpfs)
	}

	_, err = IpfsDir("")
	if err != nil {
		t.Fatal(err)
	}
}

func testCheckIpfsDir(t *testing.T, fakeIpfs string) {
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

func testRepoVersion(t *testing.T, fakeIpfs string) {
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
