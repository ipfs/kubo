package migrations

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestGetDistPath(t *testing.T) {
	os.Unsetenv(envIpfsDistPath)
	distPath := GetDistPathEnv("")
	if distPath != LatestIpfsDist {
		t.Error("did not set default dist path")
	}

	testDist := "/unit/test/dist"
	t.Setenv(envIpfsDistPath, testDist)
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
	ctx := t.Context()

	fetcher := NewHttpFetcher(testIpfsDist, testServer.URL, "", 0)

	out, err := fetcher.Fetch(ctx, "/kubo/versions")
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
	if err == nil || !strings.Contains(err.Error(), "no link") {
		t.Fatal("expected error 404")
	}
}

func TestFetchBinary(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := t.Context()

	fetcher := NewHttpFetcher(testIpfsDist, testServer.URL, "", 0)

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

	bin, err = FetchBinary(ctx, fetcher, "go-ipfs", "v1.0.0", "ipfs", tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	fi, err = os.Stat(bin)
	if os.IsNotExist(err) {
		t.Error("expected file to exist:", bin)
	}

	t.Log("downloaded and unpacked", fi.Size(), "byte file:", fi.Name())

	// Check error is destination already exists and is not directory
	_, err = FetchBinary(ctx, fetcher, "go-ipfs", "v1.0.0", "ipfs", bin)
	if !os.IsExist(err) {
		t.Fatal("expected 'exists' error, got", err)
	}

	_, err = FetchBinary(ctx, fetcher, "go-ipfs", "v1.0.0", "ipfs", tmpDir)
	if !os.IsExist(err) {
		t.Error("expected 'exists' error, got:", err)
	}

	os.Remove(filepath.Join(tmpDir, ExeName("ipfs")))

	// Check error creating temp download directory
	//
	// Windows doesn't have read-only directories https://github.com/golang/go/issues/35042 this would need to be
	// tested another way
	if runtime.GOOS != "windows" {
		err = os.Chmod(tmpDir, 0o555)
		if err != nil {
			panic(err)
		}
		t.Setenv("TMPDIR", tmpDir)
		_, err = FetchBinary(ctx, fetcher, "go-ipfs", "v1.0.0", "ipfs", tmpDir)
		if !os.IsPermission(err) {
			t.Error("expected 'permission' error, got:", err)
		}
		t.Setenv("TMPDIR", "/tmp")
		err = os.Chmod(tmpDir, 0o755)
		if err != nil {
			panic(err)
		}
	}

	// Check error if failure to fetch due to bad dist
	_, err = FetchBinary(ctx, fetcher, "not-here", "v1.0.0", "ipfs", tmpDir)
	if err == nil || !strings.Contains(err.Error(), "no link") {
		t.Error("expected 'Not Found' error, got:", err)
	}

	// Check error if failure to unpack archive
	_, err = FetchBinary(ctx, fetcher, "go-ipfs", "v1.0.0", "not-such-bin", tmpDir)
	if err == nil || err.Error() != "no binary found in archive" {
		t.Error("expected 'no binary found in archive' error")
	}
}

// TestHttpFetcherUserAgent guards against a regression where NewHttpFetcher
// accepts a userAgent parameter but forgets to store it on the struct,
// silently sending Go's default "Go-http-client/1.1" instead of the
// migration agent string.
func TestHttpFetcherUserAgent(t *testing.T) {
	const wantUA = "kubo/migration"

	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	fetcher := NewHttpFetcher("/ipfs/bafyreigh2akiscaildcqabsyg3dfr6chu3fgpregiymsck7e7aqa4s52zy", srv.URL, wantUA, 0)
	_, _ = fetcher.Fetch(t.Context(), "/anything")

	if gotUA != wantUA {
		t.Fatalf("User-Agent: got %q, want %q", gotUA, wantUA)
	}
}

// TestMigrationDownloadSourcesFailover is an end-to-end check that two
// gateways listed in Migration.DownloadSources (here passed straight into
// GetMigrationFetcher, the same path ReadMigrationConfig feeds) cooperate via
// MultiFetcher: when the first gateway either errors with 404 or returns
// bytes that don't parse as a CAR, the second gateway is attempted and the
// migration data flows through.
func TestMigrationDownloadSourcesFailover(t *testing.T) {
	ctx := t.Context()

	t.Run("first gateway returns 404", func(t *testing.T) {
		var badHits atomic.Int64
		bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			badHits.Add(1)
			http.Error(w, "not found", http.StatusNotFound)
		}))
		defer bad.Close()

		// Migration.DownloadSources order: bad first, real test gateway second.
		fetcher, err := GetMigrationFetcher([]string{bad.URL, testServer.URL}, testIpfsDist, nil)
		if err != nil {
			t.Fatalf("GetMigrationFetcher: %v", err)
		}
		defer fetcher.Close()

		out, err := fetcher.Fetch(ctx, "/kubo/versions")
		if err != nil {
			t.Fatalf("expected failover to second gateway, got: %v", err)
		}
		if len(out) < 6 {
			t.Fatalf("second gateway should have served the versions file, got %d bytes", len(out))
		}
		if badHits.Load() == 0 {
			t.Fatal("first gateway was never tried; the failover path did not actually run")
		}
	})

	t.Run("first gateway returns invalid CAR bytes", func(t *testing.T) {
		var badHits atomic.Int64
		bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			badHits.Add(1)
			w.Header().Set("Content-Type", "application/vnd.ipld.car")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("this is definitely not a valid CAR file"))
		}))
		defer bad.Close()

		fetcher, err := GetMigrationFetcher([]string{bad.URL, testServer.URL}, testIpfsDist, nil)
		if err != nil {
			t.Fatalf("GetMigrationFetcher: %v", err)
		}
		defer fetcher.Close()

		out, err := fetcher.Fetch(ctx, "/kubo/versions")
		if err != nil {
			t.Fatalf("expected failover to second gateway, got: %v", err)
		}
		if len(out) < 6 {
			t.Fatalf("second gateway should have served the versions file, got %d bytes", len(out))
		}
		if badHits.Load() == 0 {
			t.Fatal("first gateway was never tried; the failover path did not actually run")
		}
	})
}

func TestMultiFetcher(t *testing.T) {
	ctx := t.Context()

	badFetcher := NewHttpFetcher("", "bad-url", "", 0)
	fetcher := NewHttpFetcher(testIpfsDist, testServer.URL, "", 0)

	mf := NewMultiFetcher(badFetcher, fetcher)

	vers, err := mf.Fetch(ctx, "/kubo/versions")
	if err != nil {
		t.Fatal(err)
	}

	if len(vers) < 45 {
		fmt.Println("unexpected more data")
	}
}

// TestMultiFetcherQuarantine verifies that a fetcher which fails once is
// moved to the back of the rotation on subsequent calls, so a dead gateway
// does not cost the full HTTP timeout on every parallel migration download.
func TestMultiFetcherQuarantine(t *testing.T) {
	ctx := t.Context()

	tracker := &countingFetcher{}
	good := NewHttpFetcher(testIpfsDist, testServer.URL, "", 0)
	mf := NewMultiFetcher(tracker, good)

	// First call: tracker is healthy, gets tried first, fails. good takes over.
	if _, err := mf.Fetch(ctx, "/kubo/versions"); err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	if got := tracker.calls.Load(); got != 1 {
		t.Fatalf("expected tracker to be tried once, got %d", got)
	}

	// Second call: tracker is quarantined and must not be tried while good
	// is still healthy.
	if _, err := mf.Fetch(ctx, "/kubo/versions"); err != nil {
		t.Fatalf("second fetch: %v", err)
	}
	if got := tracker.calls.Load(); got != 1 {
		t.Fatalf("expected tracker to stay quarantined, got %d calls", got)
	}
}

// TestMultiFetcherQuarantineReset verifies that when every fetcher fails in a
// single Fetch call, the quarantine resets so the next call retries all
// fetchers from scratch rather than inheriting a fully poisoned set.
func TestMultiFetcherQuarantineReset(t *testing.T) {
	ctx := t.Context()

	a := &countingFetcher{}
	b := &countingFetcher{}
	mf := NewMultiFetcher(a, b)

	if _, err := mf.Fetch(ctx, "/anything"); err == nil {
		t.Fatal("expected error when all fetchers fail")
	}
	if ca, cb := a.calls.Load(), b.calls.Load(); ca != 1 || cb != 1 {
		t.Fatalf("first call: expected each fetcher tried once, got a=%d b=%d", ca, cb)
	}

	if _, err := mf.Fetch(ctx, "/anything"); err == nil {
		t.Fatal("expected error when all fetchers fail")
	}
	// After the total wipeout, both should have been retried fresh, not
	// skipped as quarantined.
	if ca, cb := a.calls.Load(), b.calls.Load(); ca != 2 || cb != 2 {
		t.Fatalf("second call after reset: expected each fetcher tried again, got a=%d b=%d", ca, cb)
	}
}

// TestMultiFetcherExhaustionCap verifies the MultiFetcher gives up after
// maxMultiFetcherFullLoopFailures full rotations, returning
// ErrMultiFetcherExhausted without trying inner fetchers again.
func TestMultiFetcherExhaustionCap(t *testing.T) {
	ctx := t.Context()

	a := &countingFetcher{}
	b := &countingFetcher{}
	mf := NewMultiFetcher(a, b)

	// Three failed full rotations should latch the breaker.
	for i := range maxMultiFetcherFullLoopFailures {
		if _, err := mf.Fetch(ctx, "/x"); err == nil {
			t.Fatalf("rotation %d: expected error", i+1)
		}
	}

	expectedCalls := int64(maxMultiFetcherFullLoopFailures)
	if ca, cb := a.calls.Load(), b.calls.Load(); ca != expectedCalls || cb != expectedCalls {
		t.Fatalf("after %d rotations: expected each fetcher called %d times, got a=%d b=%d",
			maxMultiFetcherFullLoopFailures, expectedCalls, ca, cb)
	}

	// Subsequent calls must hard-error with ErrMultiFetcherExhausted and
	// must not invoke the inner fetchers again.
	_, err := mf.Fetch(ctx, "/x")
	if !errors.Is(err, ErrMultiFetcherExhausted) {
		t.Fatalf("expected ErrMultiFetcherExhausted, got %v", err)
	}
	if ca, cb := a.calls.Load(), b.calls.Load(); ca != expectedCalls || cb != expectedCalls {
		t.Fatalf("inner fetchers called after exhaustion: a=%d b=%d", ca, cb)
	}
}

// TestMultiFetcherConcurrent exercises the locking paths under -race by
// hammering one MultiFetcher from many goroutines, mirroring how
// fetchMigrations spawns parallel downloads against a shared fetcher.
func TestMultiFetcherConcurrent(t *testing.T) {
	ctx := t.Context()

	bad := &countingFetcher{}
	good := NewHttpFetcher(testIpfsDist, testServer.URL, "", 0)
	mf := NewMultiFetcher(bad, good)

	const goroutines = 16
	const callsPerGoroutine = 8

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			for range callsPerGoroutine {
				if _, err := mf.Fetch(ctx, "/kubo/versions"); err != nil {
					t.Errorf("unexpected fetch error: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

// TestMultiFetcherSuccessResetsCounter verifies that any successful Fetch
// resets the loop-failure counter, so transient blips during a long session
// don't accumulate toward the exhaustion cap.
func TestMultiFetcherSuccessResetsCounter(t *testing.T) {
	ctx := t.Context()

	bad := &countingFetcher{}
	good := NewHttpFetcher(testIpfsDist, testServer.URL, "", 0)
	mf := NewMultiFetcher(bad, good)

	// Many alternating success calls must not trip the breaker.
	for i := range maxMultiFetcherFullLoopFailures * 3 {
		if _, err := mf.Fetch(ctx, "/kubo/versions"); err != nil {
			t.Fatalf("call %d: %v", i+1, err)
		}
	}
	if err := mf.exhaustedErr(); err != nil {
		t.Fatalf("breaker tripped despite repeated successes: %v", err)
	}
}

// TestMultiFetcherContextCancelled verifies that a cancelled context exits
// the rotation early without quarantining every fetcher or counting toward
// the exhaustion cap. Otherwise three user Ctrl-Cs in a row would latch
// ErrMultiFetcherExhausted on a perfectly healthy gateway list.
func TestMultiFetcherContextCancelled(t *testing.T) {
	a := &countingFetcher{}
	b := &countingFetcher{}
	mf := NewMultiFetcher(a, b)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	for i := range maxMultiFetcherFullLoopFailures + 2 {
		_, err := mf.Fetch(ctx, "/x")
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("call %d: expected context.Canceled, got %v", i+1, err)
		}
	}

	// Each call should have exited after the first fetcher returned the
	// cancellation error, so b is never tried and the breaker never latches.
	if got := b.calls.Load(); got != 0 {
		t.Fatalf("second fetcher should not be tried after cancellation, got %d calls", got)
	}
	if err := mf.exhaustedErr(); err != nil {
		t.Fatalf("breaker latched on cancelled-context loops: %v", err)
	}
}

// countingFetcher always errors and records how many times it was called.
// The counter is atomic so the fetcher is safe to share across goroutines
// during -race tests.
type countingFetcher struct {
	calls atomic.Int64
}

func (c *countingFetcher) Fetch(ctx context.Context, _ string) ([]byte, error) {
	c.calls.Add(1)
	return nil, fmt.Errorf("countingFetcher always fails")
}

func (c *countingFetcher) Close() error { return nil }
