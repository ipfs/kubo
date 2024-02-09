package migrations

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	gopath "path"
	"strings"

	"github.com/ipfs/boxo/blockservice"
	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/exchange/offline"
	bsfetcher "github.com/ipfs/boxo/fetcher/impl/blockservice"
	files "github.com/ipfs/boxo/files"
	"github.com/ipfs/boxo/ipld/merkledag"
	unixfile "github.com/ipfs/boxo/ipld/unixfs/file"
	"github.com/ipfs/boxo/ipns"
	"github.com/ipfs/boxo/namesys"
	"github.com/ipfs/boxo/path"
	"github.com/ipfs/boxo/path/resolver"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/ipfs/go-unixfsnode"
	gocarv2 "github.com/ipld/go-car/v2"
	dagpb "github.com/ipld/go-codec-dagpb"
	madns "github.com/multiformats/go-multiaddr-dns"
)

const (
	// default is different name than ipfs.io which is being blocked by some ISPs
	defaultGatewayURL = "https://trustless-gateway.link"
	// Default maximum download size.
	defaultFetchLimit = 1024 * 1024 * 512
)

// HttpFetcher fetches files over HTTP using verifiable CAR archives.
type HttpFetcher struct { //nolint
	distPath  string
	gateway   string
	limit     int64
	userAgent string
}

var _ Fetcher = (*HttpFetcher)(nil)

// NewHttpFetcher creates a new [HttpFetcher].
//
// Specifying "" for distPath sets the default IPNS path.
// Specifying "" for gateway sets the default.
// Specifying 0 for fetchLimit sets the default, -1 means no limit.
func NewHttpFetcher(distPath, gateway, userAgent string, fetchLimit int64) *HttpFetcher { //nolint
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
	imPath, err := f.resolvePath(ctx, gopath.Join(f.distPath, filePath))
	if err != nil {
		return nil, fmt.Errorf("path could not be resolved: %w", err)
	}

	rc, err := f.httpRequest(ctx, imPath, "application/vnd.ipld.car")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch CAR: %w", err)
	}

	return carStreamToFileBytes(ctx, rc, imPath)
}

func (f *HttpFetcher) Close() error {
	return nil
}

func (f *HttpFetcher) resolvePath(ctx context.Context, pathStr string) (path.ImmutablePath, error) {
	p, err := path.NewPath(pathStr)
	if err != nil {
		return path.ImmutablePath{}, fmt.Errorf("path is invalid: %w", err)
	}

	for p.Mutable() {
		// Download IPNS record and verify through the gateway, or resolve the
		// DNSLink with the default DNS resolver.
		name, err := ipns.NameFromString(p.Segments()[1])
		if err == nil {
			p, err = f.resolveIPNS(ctx, name)
		} else {
			p, err = f.resolveDNSLink(ctx, p)
		}

		if err != nil {
			return path.ImmutablePath{}, err
		}
	}

	return path.NewImmutablePath(p)
}

func (f *HttpFetcher) resolveIPNS(ctx context.Context, name ipns.Name) (path.Path, error) {
	rc, err := f.httpRequest(ctx, name.AsPath(), "application/vnd.ipfs.ipns-record")
	if err != nil {
		return path.ImmutablePath{}, err
	}

	rc = NewLimitReadCloser(rc, int64(ipns.MaxRecordSize))
	rawRecord, err := io.ReadAll(rc)
	if err != nil {
		return path.ImmutablePath{}, err
	}

	rec, err := ipns.UnmarshalRecord(rawRecord)
	if err != nil {
		return path.ImmutablePath{}, err
	}

	err = ipns.ValidateWithName(rec, name)
	if err != nil {
		return path.ImmutablePath{}, err
	}

	return rec.Value()
}

func (f *HttpFetcher) resolveDNSLink(ctx context.Context, p path.Path) (path.Path, error) {
	dnsResolver := namesys.NewDNSResolver(madns.DefaultResolver.LookupTXT)
	res, err := dnsResolver.Resolve(ctx, p)
	if err != nil {
		return nil, err
	}
	return res.Path, nil
}

func (f *HttpFetcher) httpRequest(ctx context.Context, p path.Path, accept string) (io.ReadCloser, error) {
	url := f.gateway + p.String()
	fmt.Printf("Fetching with HTTP: %q\n", url)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("http.NewRequest error: %w", err)
	}
	req.Header.Set("Accept", accept)

	if f.userAgent != "" {
		req.Header.Set("User-Agent", f.userAgent)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http.DefaultClient.Do error: %w", err)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		mes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("error reading error body: %w", err)
		}
		return nil, fmt.Errorf("GET %s error: %s: %s", url, resp.Status, string(mes))
	}

	var rc io.ReadCloser
	if f.limit != 0 {
		rc = NewLimitReadCloser(resp.Body, f.limit)
	} else {
		rc = resp.Body
	}

	return rc, nil
}

func carStreamToFileBytes(ctx context.Context, r io.ReadCloser, imPath path.ImmutablePath) ([]byte, error) {
	defer r.Close()

	// Create temporary block datastore and dag service.
	dataStore := dssync.MutexWrap(datastore.NewMapDatastore())
	blockStore := blockstore.NewBlockstore(dataStore)
	blockService := blockservice.New(blockStore, offline.Exchange(blockStore))
	dagService := merkledag.NewDAGService(blockService)

	defer dagService.Blocks.Close()
	defer dataStore.Close()

	// Create CAR reader
	car, err := gocarv2.NewBlockReader(r)
	if err != nil {
		fmt.Println(err)
		return nil, fmt.Errorf("error creating car reader: %s", err)
	}

	// Add all blocks to the blockstore.
	for {
		block, err := car.Next()
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("error reading block from car: %s", err)
		} else if block == nil {
			break
		}

		err = blockStore.Put(ctx, block)
		if err != nil {
			return nil, fmt.Errorf("error putting block in blockstore: %s", err)
		}
	}

	fetcherCfg := bsfetcher.NewFetcherConfig(blockService)
	fetcherCfg.PrototypeChooser = dagpb.AddSupportToChooser(bsfetcher.DefaultPrototypeChooser)
	fetcher := fetcherCfg.WithReifier(unixfsnode.Reify)
	resolver := resolver.NewBasicResolver(fetcher)

	cid, _, err := resolver.ResolveToLastNode(ctx, imPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve: %w", err)
	}

	nd, err := dagService.Get(ctx, cid)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve: %w", err)
	}

	// Make UnixFS file out of the node.
	uf, err := unixfile.NewUnixfsFile(ctx, dagService, nd)
	if err != nil {
		return nil, fmt.Errorf("error building unixfs file: %s", err)
	}

	// Check if it's a file and return.
	if f, ok := uf.(files.File); ok {
		return io.ReadAll(f)
	}

	return nil, errors.New("unexpected unixfs node type")
}
