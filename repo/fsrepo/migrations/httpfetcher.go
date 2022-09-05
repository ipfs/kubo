package migrations

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path"
	"strings"

	bservice "github.com/ipfs/go-blockservice"
	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	files "github.com/ipfs/go-ipfs-files"
	dag "github.com/ipfs/go-merkledag"
	unixfile "github.com/ipfs/go-unixfs/file"
	gocarv2 "github.com/ipld/go-car/v2"
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
	req.Header.Set("Accept", "application/vnd.ipld.car")

	if f.userAgent != "" {
		req.Header.Set("User-Agent", f.userAgent)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http.DefaultClient.Do error: %s", err)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		mes, err := io.ReadAll(resp.Body)
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

	return carStreamToFileBytes(ctx, rc)
}

func (f *HttpFetcher) Close() error {
	return nil
}

func carStreamToFileBytes(ctx context.Context, r io.Reader) ([]byte, error) {
	// Create temporary block datastore and dag service.
	db := dssync.MutexWrap(ds.NewMapDatastore())
	bs := bstore.NewBlockstore(db)
	ds := dag.NewDAGService(bservice.New(bs, offline.Exchange(bs)))

	defer ds.Blocks.Close()
	defer db.Close()

	// Create CAR reader
	car, err := gocarv2.NewBlockReader(r)
	if err != nil {
		return nil, fmt.Errorf("error creating car reader: %s", err)
	}

	// Add all blocks to the blockstore.
	for {
		block, err := car.Next()
		if err != nil && err != io.EOF {
			return nil, err
		} else if block == nil {
			break
		}

		err = bs.Put(ctx, block)
		if err != nil {
			return nil, err
		}
	}

	if len(car.Roots) != 1 {
		return nil, errors.New("multiple-root CAR unexpected")
	}

	// Get node from DAG service with the file.
	node, err := ds.Get(ctx, car.Roots[0])
	if err != nil {
		return nil, err
	}

	// Make UnixFS file out of the node.
	uf, err := unixfile.NewUnixfsFile(ctx, ds, node)
	if err != nil {
		return nil, err
	}

	// Check if it's a file and return.
	if f, ok := uf.(files.File); ok {
		return ioutil.ReadAll(f)
	}

	return nil, errors.New("unexpected unixfs node type")
}
