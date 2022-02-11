package migrations

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path"
	"strings"
)

const (
	defaultGatewayURL = "https://ipfs.io"
	// Default maximum download size
	defaultFetchLimit = 1024 * 1024 * 512
)

// HttpFetcher fetches files over HTTP
type HttpFetcher struct {
	distPath  string
	gateway   string
	limit     int64
	userAgent string
}

var _ Fetcher = (*HttpFetcher)(nil)

// NewHttpFetcher creates a new HttpFetcher
//
// Specifying "" for distPath sets the default IPNS path.
// Specifying "" for gateway sets the default.
// Specifying 0 for fetchLimit sets the default, -1 means no limit.
func NewHttpFetcher(distPath, gateway, userAgent string, fetchLimit int64) *HttpFetcher {
	f := &HttpFetcher{
		distPath: LatestIpfsDist,
		gateway:  defaultGatewayURL,
		limit:    defaultFetchLimit,
	}

	if distPath != "" {
		if !strings.HasPrefix(distPath, "/") {
			distPath = "/" + distPath
		}
		f.distPath = distPath
	}

	if gateway != "" {
		f.gateway = strings.TrimRight(gateway, "/")
	}

	if fetchLimit != 0 {
		if fetchLimit < 0 {
			fetchLimit = 0
		}
		f.limit = fetchLimit
	}

	return f
}

// Fetch attempts to fetch the file at the given path, from the distribution
// site configured for this HttpFetcher.
func (f *HttpFetcher) Fetch(ctx context.Context, filePath string) ([]byte, error) {
	gwURL := f.gateway + path.Join(f.distPath, filePath)
	fmt.Printf("Fetching with HTTP: %q\n", gwURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, gwURL, nil)
	if err != nil {
		return nil, fmt.Errorf("http.NewRequest error: %s", err)
	}

	if f.userAgent != "" {
		req.Header.Set("User-Agent", f.userAgent)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http.DefaultClient.Do error: %s", err)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		mes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("error reading error body: %s", err)
		}
		return nil, fmt.Errorf("GET %s error: %s: %s", gwURL, resp.Status, string(mes))
	}

	var rc io.ReadCloser
	if f.limit != 0 {
		rc = NewLimitReadCloser(resp.Body, f.limit)
	} else {
		rc = resp.Body
	}
	defer rc.Close()

	return ioutil.ReadAll(rc)
}

func (f *HttpFetcher) Close() error {
	return nil
}
