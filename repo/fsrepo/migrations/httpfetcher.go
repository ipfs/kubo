package migrations

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	gopath "path"
	"strings"
	"time"

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
	config "github.com/ipfs/kubo/config"
	gocarv2 "github.com/ipld/go-car/v2"
	dagpb "github.com/ipld/go-codec-dagpb"
	madns "github.com/multiformats/go-multiaddr-dns"
)

const (
	// defaultGatewayURL is used when no gateway is configured. Some ISPs
	// block ipfs.io, so we use a different hostname.
	defaultGatewayURL = "https://trustless-gateway.link"
	// Default maximum download size.
	defaultFetchLimit = 1024 * 1024 * 512

	// Sized for users on slow / high-latency networks (e.g. VPNs through
	// congested links): a 3-RTT TLS handshake at 1-2s RTT plus packet
	// loss can legitimately approach 10s. 15s leaves headroom while
	// still failing fast against truly dead gateways.
	dialTimeout         = 15 * time.Second
	tlsHandshakeTimeout = 15 * time.Second
)

// defaultMigrationGateways is a last-resort fallback used when
// Migration.DownloadSources expands the "HTTPS" alias. The first entry
// (trustless-gateway.link) serves nearly all users; the rest are tried
// only when it is blocked or unreachable.
//
// Including third-party gateways is safe: each block is fetched as CAR
// and verified against the requested CID's multihash, so a malicious
// operator cannot substitute different content.
//
// TODO: replace this static list with a dynamic source, either the public
// gateway checker list at
// https://github.com/ipfs/public-gateway-checker/raw/refs/heads/main/gateways.json
// or AutoConf. Not done yet because this code path only runs for repos
// from go-ipfs or Kubo older than v0.27 (roughly 2020 vintage). Modern
// Kubo ships embedded migrations and never reaches it, so the impact and
// risk of leaving the list hard-coded are both low.
var defaultMigrationGateways = []string{
	defaultGatewayURL,
	"https://gateway.pinata.cloud",
	"https://ipfs.filebase.io",
	"https://4everland.io",
	"https://dget.top",
}

// migrationHTTPClient is the HTTP client for migration downloads. Its timeouts
// fail fast on unreachable or stalled gateways so MultiFetcher can rotate.
var migrationHTTPClient = &http.Client{
	Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout: dialTimeout,
		}).DialContext,
		TLSHandshakeTimeout: tlsHandshakeTimeout,
		// ResponseHeaderTimeout matches boxo's server-side
		// DefaultRetrievalTimeout (re-exported via Kubo config): a healthy
		// gateway returns the first byte within this budget or 504s.
		// Mirroring it avoids drift if boxo retunes.
		ResponseHeaderTimeout: config.DefaultRetrievalTimeout,
		IdleConnTimeout:       90 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
	},
	// No overall Timeout: cancellation flows from the request context, so
	// streaming bodies run to completion under context control.
}

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
		distPath:  LatestIpfsDist,
		gateway:   defaultGatewayURL,
		limit:     defaultFetchLimit,
		userAgent: userAgent,
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

	rc, err := f.httpRequest(ctx, imPath, "application/vnd.ipld.car", "car")
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
	rc, err := f.httpRequest(ctx, name.AsPath(), "application/vnd.ipfs.ipns-record", "ipns-record")
	if err != nil {
		return path.ImmutablePath{}, err
	}
	defer rc.Close()

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

func (f *HttpFetcher) httpRequest(ctx context.Context, p path.Path, accept, format string) (io.ReadCloser, error) {
	url := f.gateway + p.String()
	// Pass the format hint as both an Accept header and a ?format= query
	// parameter. The trustless gateway spec defines both as valid
	// signaling mechanisms, and some gateway implementations honor one
	// but not the other; sending both maximizes compatibility.
	if format != "" {
		url += "?format=" + format
	}
	fmt.Printf("Fetching with HTTP: %q\n", url)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("http.NewRequest error: %w", err)
	}
	req.Header.Set("Accept", accept)

	if f.userAgent != "" {
		req.Header.Set("User-Agent", f.userAgent)
	}

	resp, err := migrationHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("migration http request error: %w", err)
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
