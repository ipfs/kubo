package migrations

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/hashicorp/go-multierror"
)

const (
	// Current distribution to fetch migrations from.
	CurrentIpfsDist = "/ipfs/QmRzRGJEjYDfbHHaALnHBuhzzrkXGdwcPMrgd5fgM7hqbe" // fs-repo-15-to-16 v1.0.1
	// Latest distribution path.  Default for fetchers.
	LatestIpfsDist = "/ipns/dist.ipfs.tech"

	// Distribution environ variable.
	envIpfsDistPath = "IPFS_DIST_PATH"
)

type Fetcher interface {
	// Fetch attempts to fetch the file at the given ipfs path.
	Fetch(ctx context.Context, filePath string) ([]byte, error)
	// Close performs any cleanup after the fetcher is not longer needed.
	Close() error
}

// MultiFetcher holds multiple Fetchers and provides a Fetch that tries each
// until one succeeds.
type MultiFetcher struct {
	fetchers []Fetcher
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
	}
	copy(mf.fetchers, f)
	return mf
}

// Fetch attempts to fetch the file at each of its fetchers until one succeeds.
func (f *MultiFetcher) Fetch(ctx context.Context, ipfsPath string) ([]byte, error) {
	var errs error
	for _, fetcher := range f.fetchers {
		out, err := fetcher.Fetch(ctx, ipfsPath)
		if err == nil {
			return out, nil
		}
		fmt.Printf("Error fetching: %s\n", err.Error())
		errs = multierror.Append(errs, err)
	}
	return nil, errs
}

func (f *MultiFetcher) Close() error {
	var errs error
	for _, fetcher := range f.fetchers {
		if err := fetcher.Close(); err != nil {
			errs = multierror.Append(errs, err)
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

// NewLimitReadCloser returns a new io.ReadCloser with the reader wrappen in a
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
// To get the IPFS path of the latest distribution, if not overriddin by the
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
