package migrations

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

const (
	// Current distribution to fetch migrations from.
	CurrentIpfsDist = "/ipfs/QmRzRGJEjYDfbHHaALnHBuhzzrkXGdwcPMrgd5fgM7hqbe" // fs-repo-15-to-16 v1.0.1
	// Latest distribution path.  Default for fetchers.
	LatestIpfsDist = "/ipns/dist.ipfs.tech"

	// Distribution environ variable.
	envIpfsDistPath = "IPFS_DIST_PATH"

	// maxMultiFetcherFullLoopFailures caps how many times a MultiFetcher
	// may exhaust every fetcher before it gives up for the rest of its
	// lifetime. Without a cap, a fully unreachable network would keep
	// every Fetch call paying the full per-gateway timeout once per call.
	maxMultiFetcherFullLoopFailures = 3
)

// ErrMultiFetcherExhausted is returned by MultiFetcher.Fetch after every
// fetcher has failed maxMultiFetcherFullLoopFailures full rotations in a row.
// The message points the user at the recovery path: replacing the gateway
// list via Migration.DownloadSources in the Kubo config.
var ErrMultiFetcherExhausted = errors.New("migration download exhausted: every configured gateway failed; add a reachable HTTPS gateway to Migration.DownloadSources in your Kubo config and retry")

type Fetcher interface {
	// Fetch attempts to fetch the file at the given ipfs path.
	Fetch(ctx context.Context, filePath string) ([]byte, error)
	// Close performs any cleanup after the fetcher is not longer needed.
	Close() error
}

// MultiFetcher tries each Fetcher in turn until one succeeds. A fetcher that
// has already failed in this MultiFetcher's lifetime moves to the back of the
// rotation; if every healthy fetcher fails, the quarantined ones run as a
// fallback so a transient outage can self-heal. If every fetcher fails in a
// single call, the quarantine resets and the next call starts fresh, but
// only up to maxMultiFetcherFullLoopFailures times: after that the
// MultiFetcher returns ErrMultiFetcherExhausted without trying again.
//
// This acts as a session-scoped circuit breaker: when migrations issue many
// parallel downloads through one MultiFetcher, the first failure drops a
// dead gateway from rotation for the rest of the session instead of charging
// every goroutine the full HTTP timeout against it.
type MultiFetcher struct {
	fetchers []Fetcher

	mu           sync.Mutex
	failed       map[int]struct{}
	loopFailures int
	exhausted    error
}

type limitReadCloser struct {
	io.Reader
	io.Closer
}

// NewMultiFetcher creates a MultiFetcher with the given Fetchers.  The
// Fetchers are tried in order, then passed to this function.
func NewMultiFetcher(f ...Fetcher) *MultiFetcher {
	mf := &MultiFetcher{
		fetchers: make([]Fetcher, len(f)),
		failed:   make(map[int]struct{}),
	}
	copy(mf.fetchers, f)
	return mf
}

// Fetch tries each fetcher until one succeeds. Fetchers that have already
// failed in this session are tried last. Once every fetcher has failed
// maxMultiFetcherFullLoopFailures full loops in a row, Fetch returns
// ErrMultiFetcherExhausted without further attempts.
func (f *MultiFetcher) Fetch(ctx context.Context, ipfsPath string) ([]byte, error) {
	if err := f.exhaustedErr(); err != nil {
		return nil, err
	}

	var errs []error
	for _, i := range f.tryOrder() {
		out, err := f.fetchers[i].Fetch(ctx, ipfsPath)
		if err == nil {
			f.markOutcome(i, true)
			return out, nil
		}
		// A cancelled or timed-out context is not the gateway's fault.
		// Returning early avoids quarantining every fetcher and bumping
		// the loop-failure counter, which could latch the exhaustion
		// breaker after a few user-initiated cancellations.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		fmt.Printf("Error fetching: %s\n", err.Error())
		errs = append(errs, err)
		f.markOutcome(i, false)
	}

	// Every fetcher failed in this call. Bump the loop-failure counter
	// and decide whether to give up entirely or let the next call retry.
	if err := f.recordFullLoopFailure(errs); err != nil {
		return nil, err
	}
	return nil, errors.Join(errs...)
}

// tryOrder returns the indices of all fetchers in the order they should be
// tried this call: never-failed fetchers first, previously-failed ones last,
// each group keeping its original order.
func (f *MultiFetcher) tryOrder() []int {
	f.mu.Lock()
	defer f.mu.Unlock()
	order := make([]int, 0, len(f.fetchers))
	var quarantined []int
	for i := range f.fetchers {
		if _, bad := f.failed[i]; bad {
			quarantined = append(quarantined, i)
		} else {
			order = append(order, i)
		}
	}
	return append(order, quarantined...)
}

// markOutcome records the result of a single fetcher attempt: a success
// clears any quarantine bit on i and resets the loop-failure streak, while a
// failure puts i in quarantine. Both operations are idempotent.
func (f *MultiFetcher) markOutcome(i int, success bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if success {
		delete(f.failed, i)
		f.loopFailures = 0
		return
	}
	f.failed[i] = struct{}{}
}

// recordFullLoopFailure increments the full-loop counter. If the cap is
// reached, the MultiFetcher latches into the exhausted state and returns
// ErrMultiFetcherExhausted (wrapping the last batch of errors). Otherwise
// the quarantine is cleared so the next call retries every fetcher fresh.
func (f *MultiFetcher) recordFullLoopFailure(errs []error) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.loopFailures++
	if f.loopFailures >= maxMultiFetcherFullLoopFailures {
		f.exhausted = fmt.Errorf("%w: %w", ErrMultiFetcherExhausted, errors.Join(errs...))
		return f.exhausted
	}
	clear(f.failed)
	return nil
}

// exhaustedErr returns the latched exhaustion error, or nil if the
// MultiFetcher is still in service.
func (f *MultiFetcher) exhaustedErr() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.exhausted
}

func (f *MultiFetcher) Close() error {
	var errs error
	for _, fetcher := range f.fetchers {
		if err := fetcher.Close(); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	return errs
}

func (f *MultiFetcher) Len() int {
	return len(f.fetchers)
}

func (f *MultiFetcher) Fetchers() []Fetcher {
	return f.fetchers
}

// NewLimitReadCloser returns a new io.ReadCloser with the reader wrapped in a
// io.LimitedReader limited to reading the amount specified.
func NewLimitReadCloser(rc io.ReadCloser, limit int64) io.ReadCloser {
	return limitReadCloser{
		Reader: io.LimitReader(rc, limit),
		Closer: rc,
	}
}

// GetDistPathEnv returns the IPFS path to the distribution site, using
// the value of environ variable specified by envIpfsDistPath.  If the environ
// variable is not set, then returns the provided distPath, and if that is not set
// then returns the IPNS path.
//
// To get the IPFS path of the latest distribution, if not overridden by the
// environ variable: GetDistPathEnv(CurrentIpfsDist).
func GetDistPathEnv(distPath string) string {
	if dist := os.Getenv(envIpfsDistPath); dist != "" {
		return dist
	}
	if distPath == "" {
		return LatestIpfsDist
	}
	return distPath
}
