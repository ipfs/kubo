package main

import (
	"testing"

	"github.com/ipfs/go-ipfs/repo/fsrepo/migrations"
	"github.com/ipfs/go-ipfs/repo/fsrepo/migrations/ipfsfetcher"
)

func TestGetMigrationFetcher(t *testing.T) {
	var f migrations.Fetcher
	var err error
	f, err = getMigrationFetcher(nil, "ftp://bad.gateway.io")
	if err == nil {
		t.Fatal("Expected bad URL scheme error")
	}

	f, err = getMigrationFetcher(nil, "ipfs")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := f.(*ipfsfetcher.IpfsFetcher); !ok {
		t.Fatal("expected IpfsFetcher")
	}

	f, err = getMigrationFetcher(nil, "http")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := f.(*migrations.HttpFetcher); !ok {
		t.Fatal("expected HttpFetcher")
	}

	f, err = getMigrationFetcher(nil, "")
	if err != nil {
		t.Fatal(err)
	}
	mf, ok := f.(*migrations.MultiFetcher)
	if !ok {
		t.Fatal("expected MultiFetcher")
	}
	if mf.Len() != 2 {
		t.Fatal("expected 2 fetchers in MultiFetcher")
	}

	f, err = getMigrationFetcher(nil, "ipfs,http,some.domain.io")
	if err != nil {
		t.Fatal(err)
	}
	mf, ok = f.(*migrations.MultiFetcher)
	if !ok {
		t.Fatal("expected MultiFetcher")
	}
	if mf.Len() != 3 {
		t.Fatal("expected3 fetchers in MultiFetcher")
	}
}
