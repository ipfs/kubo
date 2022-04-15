package corehttp

import (
	"context"
	"net/http"
	"net/url"
	gopath "path"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	cid "github.com/ipfs/go-cid"
	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/go-ipfs/assets"
	"github.com/ipfs/go-ipfs/tracing"
	path "github.com/ipfs/go-path"
	"github.com/ipfs/go-path/resolver"
	unixfile "github.com/ipfs/go-unixfs/file"
	options "github.com/ipfs/interface-go-ipfs-core/options"
	ipath "github.com/ipfs/interface-go-ipfs-core/path"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// serveDirectory returns the best representation of UnixFS directory
//
// It will return index.html if present, or generate directory listing otherwise.
func (i *gatewayHandler) serveDirectory(ctx context.Context, w http.ResponseWriter, r *http.Request, resolvedPath ipath.Resolved, contentPath ipath.Path, dir files.Directory, begin time.Time, logger *zap.SugaredLogger) {
	ctx, span := tracing.Span(ctx, "Gateway", "ServeDirectory", trace.WithAttributes(attribute.String("path", resolvedPath.String())))
	defer span.End()

	// HostnameOption might have constructed an IPNS/IPFS path using the Host header.
	// In this case, we need the original path for constructing redirects
	// and links that match the requested URL.
	// For example, http://example.net would become /ipns/example.net, and
	// the redirects and links would end up as http://example.net/ipns/example.net
	requestURI, err := url.ParseRequestURI(r.RequestURI)
	if err != nil {
		webError(w, "failed to parse request path", err, http.StatusInternalServerError)
		return
	}
	originalUrlPath := requestURI.Path

	// Check if directory has index.html, if so, serveFile
	idxPath := ipath.Join(resolvedPath, "index.html")
	idx, err := i.api.Unixfs().Get(ctx, idxPath)
	switch err.(type) {
	case nil:
		cpath := contentPath.String()
		dirwithoutslash := cpath[len(cpath)-1] != '/'
		goget := r.URL.Query().Get("go-get") == "1"
		if dirwithoutslash && !goget {
			// See comment above where originalUrlPath is declared.
			suffix := "/"
			if r.URL.RawQuery != "" {
				// preserve query parameters
				suffix = suffix + "?" + r.URL.RawQuery
			}

			redirectURL := originalUrlPath + suffix
			logger.Debugw("serving index.html file", "to", redirectURL, "status", http.StatusFound, "path", idxPath)
			http.Redirect(w, r, redirectURL, http.StatusFound)
			return
		}

		f, ok := idx.(files.File)
		if !ok {
			internalWebError(w, files.ErrNotReader)
			return
		}

		logger.Debugw("serving index.html file", "path", idxPath)
		// write to request
		i.serveFile(ctx, w, r, resolvedPath, idxPath, f, begin)
		return
	case resolver.ErrNoLink:
		logger.Debugw("no index.html; noop", "path", idxPath)
	default:
		internalWebError(w, err)
		return
	}

	// See statusResponseWriter.WriteHeader
	// and https://github.com/ipfs/go-ipfs/issues/7164
	// Note: this needs to occur before listingTemplate.Execute otherwise we get
	// superfluous response.WriteHeader call from prometheus/client_golang
	if w.Header().Get("Location") != "" {
		logger.Debugw("location moved permanently", "status", http.StatusMovedPermanently)
		w.WriteHeader(http.StatusMovedPermanently)
		return
	}

	// A HTML directory index will be presented, be sure to set the correct
	// type instead of relying on autodetection (which may fail).
	w.Header().Set("Content-Type", "text/html")

	// Generated dir index requires custom Etag (it may change between go-ipfs versions)
	if assets.AssetHash != "" {
		dirEtag := getDirListingEtag(resolvedPath.Cid())
		w.Header().Set("Etag", dirEtag)
		if r.Header.Get("If-None-Match") == dirEtag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	if r.Method == http.MethodHead {
		logger.Debug("return as request's HTTP method is HEAD")
		return
	}

	// storage for directory listing
	var dirListing []directoryItem
	type entryRequest struct {
		entryIndex int
		cid cid.Cid
	}
	var entryRequests []*entryRequest

	// Optimization 1:
	// List children without fetching their root blocks (fast, but no size info)
	results, err := i.api.Unixfs().Ls(ctx, resolvedPath, options.Unixfs.ResolveChildren(false))
	if err != nil {
		internalWebError(w, err)
		return
	}
	for link := range results {
		if link.Err != nil {
			internalWebError(w, err)
			return
		}
		hash := link.Cid.String()
		di := directoryItem{
			Size:      "", // no size because we did not fetch child nodes
			Name:      link.Name,
			Path:      gopath.Join(originalUrlPath, link.Name),
			Hash:      hash,
			ShortHash: shortHash(hash),
		}
		dirListing = append(dirListing, di)
		entryRequests = append(entryRequests, &entryRequest{
			entryIndex: len(dirListing) - 1,
			cid:       link.Cid,
		})
	}

	// Optimization 2:
	// Fetch in parallel entry's metadata up to FastDirIndexThreshold.
	// Code inspired from go-merkledag.Walk().
	// FIXME: Review number and (maybe) export it.
	const defaultConcurrentFetch = 32
	fetchersCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	feed := make(chan *entryRequest)
	go func() {
		for _, r := range entryRequests {
			select {
			case feed <- r:
			case <-fetchersCtx.Done():
				return
			}
		}
	}()
	type entryInfo struct {
		entryIndex int
		size string
	}
	out := make(chan entryInfo)
	errChan := make(chan error)

	for c := 0; c < defaultConcurrentFetch; c++ {
		go func() {
			for r := range feed {
				nd, err := i.api.Dag().Get(fetchersCtx, r.cid)
				if err != nil {
					select {
					case errChan <- err:
					case <-fetchersCtx.Done():
					}
					return
				}
				file, err := unixfile.NewUnixfsFile(fetchersCtx, i.api.Dag(), nd)
				if err != nil {
					select {
					case errChan <- err:
					case <-fetchersCtx.Done():
					}
					return
				}
				size := "?"
				if s, err := file.Size(); err == nil {
					// Size may not be defined/supported. Continue anyways.
					// FIXME: Check above. The UnixFS files we're iterating
					//  (because we use the UnixFS API) should always support
					//  this.
					size = humanize.Bytes(uint64(s))
				}

				outInfo := entryInfo {
					entryIndex: r.entryIndex,
					size: size,
				}
				select {
				case out <- outInfo:
				case <-fetchersCtx.Done():
					return
				}
			}
		}()
	}

	entriesFetch := 0
	for (i.config.FastDirIndexThreshold == 0 || entriesFetch < i.config.FastDirIndexThreshold) &&
		entriesFetch < len(dirListing) {
		select {
		case outInfo := <-out:
			dirListing[outInfo.entryIndex].Size = outInfo.size
			entriesFetch++
		case err := <-errChan:
			internalWebError(w, err)
			return
		case <-ctx.Done():
			if ctx.Err() != nil {
				internalWebError(w, ctx.Err())
			}
			return
		}
	}

	// construct the correct back link
	// https://github.com/ipfs/go-ipfs/issues/1365
	var backLink string = originalUrlPath

	// don't go further up than /ipfs/$hash/
	pathSplit := path.SplitList(contentPath.String())
	switch {
	// keep backlink
	case len(pathSplit) == 3: // url: /ipfs/$hash

	// keep backlink
	case len(pathSplit) == 4 && pathSplit[3] == "": // url: /ipfs/$hash/

	// add the correct link depending on whether the path ends with a slash
	default:
		if strings.HasSuffix(backLink, "/") {
			backLink += "./.."
		} else {
			backLink += "/.."
		}
	}

	size := "?"
	if s, err := dir.Size(); err == nil {
		// Size may not be defined/supported. Continue anyways.
		size = humanize.Bytes(uint64(s))
	}

	hash := resolvedPath.Cid().String()

	// Gateway root URL to be used when linking to other rootIDs.
	// This will be blank unless subdomain or DNSLink resolution is being used
	// for this request.
	var gwURL string

	// Get gateway hostname and build gateway URL.
	if h, ok := r.Context().Value("gw-hostname").(string); ok {
		gwURL = "//" + h
	} else {
		gwURL = ""
	}

	dnslink := hasDNSLinkOrigin(gwURL, contentPath.String())

	// See comment above where originalUrlPath is declared.
	tplData := listingTemplateData{
		GatewayURL:            gwURL,
		DNSLink:               dnslink,
		Listing:               dirListing,
		Size:                  size,
		Path:                  contentPath.String(),
		Breadcrumbs:           breadcrumbs(contentPath.String(), dnslink),
		BackLink:              backLink,
		Hash:                  hash,
		FastDirIndexThreshold: i.config.FastDirIndexThreshold,
	}

	logger.Debugw("request processed", "tplDataDNSLink", dnslink, "tplDataSize", size, "tplDataBackLink", backLink, "tplDataHash", hash)

	if err := listingTemplate.Execute(w, tplData); err != nil {
		internalWebError(w, err)
		return
	}

	// Update metrics
	i.unixfsGenDirGetMetric.WithLabelValues(contentPath.Namespace()).Observe(time.Since(begin).Seconds())
}

func getDirListingEtag(dirCid cid.Cid) string {
	return `"DirIndex-` + assets.AssetHash + `_CID-` + dirCid.String() + `"`
}
