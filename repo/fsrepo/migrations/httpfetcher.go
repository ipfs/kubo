package migrations

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"
)

const (
	defaultGatewayURL = "https://ipfs.io"
	defaultFetchLimit = 1024 * 1024 * 512
)

// HttpFetcher fetches files over HTTP
type HttpFetcher struct {
	gateway  string
	distPath string
	limit    int64
}

var _ Fetcher = (*HttpFetcher)(nil)

// NewHttpFetcher creates a new HttpFetcher
func NewHttpFetcher() *HttpFetcher {
	return &HttpFetcher{
		gateway:  defaultGatewayURL,
		distPath: IpnsIpfsDist,
		limit:    defaultFetchLimit,
	}
}

// SetGateway sets the gateway URL
func (f *HttpFetcher) SetGateway(gatewayURL string) error {
	gwURL, err := url.Parse(gatewayURL)
	if err != nil {
		return err
	}
	f.gateway = gwURL.String()
	return nil
}

// SetDistPath sets the path to the distribution site.
func (f *HttpFetcher) SetDistPath(distPath string) {
	if !strings.HasPrefix(distPath, "/") {
		distPath = "/" + distPath
	}
	f.distPath = distPath
}

// SetFetchLimit sets the download size limit. A value of 0 means no limit.
func (f *HttpFetcher) SetFetchLimit(limit int64) {
	f.limit = limit
}

// Fetch attempts to fetch the file at the given path, from the distribution
// site configured for this HttpFetcher.  Returns io.ReadCloser on success,
// which caller must close.
func (f *HttpFetcher) Fetch(ctx context.Context, filePath string) (io.ReadCloser, error) {
	gwURL := f.gateway + path.Join(f.distPath, filePath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, gwURL, nil)
	if err != nil {
		return nil, fmt.Errorf("http.NewRequest error: %s", err)
	}

	req.Header.Set("User-Agent", "go-ipfs")

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

	if f.limit != 0 {
		return NewLimitReadCloser(resp.Body, f.limit), nil
	}
	return resp.Body, nil
}
