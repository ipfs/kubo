package migrations

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"
)

func TestFindMigrations(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "migratetest")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	migs, bins, err := findMigrations(ctx, 0, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(migs) != 5 {
		t.Fatal("expected 5 migrations")
	}
	if len(bins) != 0 {
		t.Fatal("should not have found migrations")
	}

	for i := 1; i < 6; i++ {
		createFakeBin(i-1, i, tmpDir)
	}

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir)
	defer os.Setenv("PATH", origPath)

	migs, bins, err = findMigrations(ctx, 0, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(migs) != 5 {
		t.Fatal("expected 5 migrations")
	}
	if len(bins) != len(migs) {
		t.Fatal("missing", len(migs)-len(bins), "migrations")
	}

	os.Remove(bins[migs[2]])

	migs, bins, err = findMigrations(ctx, 0, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(bins) != len(migs)-1 {
		t.Fatal("should be missing one migration bin")
	}
}

func TestFindMigrationsReverse(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "migratetest")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	migs, bins, err := findMigrations(ctx, 5, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(migs) != 5 {
		t.Fatal("expected 5 migrations")
	}
	if len(bins) != 0 {
		t.Fatal("should not have found migrations")
	}

	for i := 1; i < 6; i++ {
		createFakeBin(i-1, i, tmpDir)
	}

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir)
	defer os.Setenv("PATH", origPath)

	migs, bins, err = findMigrations(ctx, 5, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(migs) != 5 {
		t.Fatal("expected 5 migrations")
	}
	if len(bins) != len(migs) {
		t.Fatal("missing", len(migs)-len(bins), "migrations:", migs)
	}

	os.Remove(bins[migs[2]])

	migs, bins, err = findMigrations(ctx, 5, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(bins) != len(migs)-1 {
		t.Fatal("should be missing one migration bin")
	}
}

func TestFetchMigrations(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ts := createTestServer()
	defer ts.Close()
	fetcher := NewHttpFetcher(CurrentIpfsDist, ts.URL, "", 0)

	tmpDir, err := ioutil.TempDir("", "migratetest")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	needed := []string{"ipfs-1-to-2", "ipfs-2-to-3"}
	fetched, err := fetchMigrations(ctx, fetcher, needed, tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	for _, bin := range fetched {
		_, err = os.Stat(bin)
		if os.IsNotExist(err) {
			t.Error("expected file to exist:", bin)
		}
	}
}

func TestRunMigrations(t *testing.T) {
	var err error
	fakeHome, err = ioutil.TempDir("", "testhome")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(fakeHome)

	os.Setenv("HOME", fakeHome)
	fakeIpfs := path.Join(fakeHome, ".ipfs")

	err = os.Mkdir(fakeIpfs, os.ModePerm)
	if err != nil {
		panic(err)
	}

	testVer := 11
	err = WriteRepoVersion(fakeIpfs, testVer)
	if err != nil {
		t.Fatal(err)
	}

	ts := createTestServer()
	defer ts.Close()
	fetcher := NewHttpFetcher(CurrentIpfsDist, ts.URL, "", 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	targetVer := 9

	err = RunMigration(ctx, fetcher, targetVer, fakeIpfs, false)
	if err == nil || !strings.HasPrefix(err.Error(), "downgrade not allowed") {
		t.Fatal("expected 'downgrade not alloed' error")
	}

	err = RunMigration(ctx, fetcher, targetVer, fakeIpfs, true)
	if err != nil {
		if !strings.HasPrefix(err.Error(), "migration ipfs-10-to-11 failed") {
			t.Fatal(err)
		}
	}
}

func createFakeBin(from, to int, tmpDir string) {
	migPath := path.Join(tmpDir, ExeName(migrationName(from, to)))
	emptyFile, err := os.Create(migPath)
	if err != nil {
		panic(err)
	}
	emptyFile.Close()
	err = os.Chmod(migPath, 0755)
	if err != nil {
		panic(err)
	}
}
