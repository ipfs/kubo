package migrations

import (
	"context"
	"testing"

	"github.com/blang/semver/v4"
)

const testDist = "go-ipfs"

func TestDistVersions(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fetcher := NewHttpFetcher(testIpfsDist, testServer.URL, "", 0)

	vers, err := DistVersions(ctx, fetcher, testDist, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(vers) == 0 {
		t.Fatal("no versions of", testDist)
	}
	t.Log("There are", len(vers), "versions of", testDist)
	t.Log("Latest 5 are:", vers[:5])
}

func TestLatestDistVersion(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fetcher := NewHttpFetcher(testIpfsDist, testServer.URL, "", 0)

	latest, err := LatestDistVersion(ctx, fetcher, testDist, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(latest) < 6 {
		t.Fatal("latest version string too short", latest)
	}
	_, err = semver.New(latest[1:])
	if err != nil {
		t.Fatal("latest version has invalid format:", latest)
	}
	t.Log("Latest version of", testDist, "is", latest)
}
