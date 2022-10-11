package corehttp

import (
	"context"
	"fmt"
	"net/http"
	"time"

	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	ipath "github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/ipfs/kubo/tracing"
	gocar "github.com/ipld/go-car"
	selectorparse "github.com/ipld/go-ipld-prime/traversal/selector/parse"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// serveCAR returns a CAR stream for specific DAG+selector
func (i *gatewayHandler) serveCAR(ctx context.Context, w http.ResponseWriter, r *http.Request, resolvedPath ipath.Resolved, contentPath ipath.Path, carVersion string, begin time.Time) {
	ctx, span := tracing.Span(ctx, "Gateway", "ServeCAR", trace.WithAttributes(attribute.String("path", resolvedPath.String())))
	defer span.End()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	switch carVersion {
	case "": // noop, client does not care about version
	case "1": // noop, we support this
	default:
		err := fmt.Errorf("only version=1 is supported")
		webError(w, "unsupported CAR version", err, http.StatusBadRequest)
		return
	}
	rootCid := resolvedPath.Cid()

	// Set Content-Disposition
	var name string
	if urlFilename := r.URL.Query().Get("filename"); urlFilename != "" {
		name = urlFilename
	} else {
		name = rootCid.String() + ".car"
	}
	setContentDispositionHeader(w, name, "attachment")

	// Set Cache-Control (same logic as for a regular files)
	addCacheControlHeaders(w, r, contentPath, rootCid)

	// Weak Etag W/ because we can't guarantee byte-for-byte identical
	// responses, but still want to benefit from HTTP Caching. Two CAR
	// responses for the same CID and selector will be logically equivalent,
	// but when CAR is streamed, then in theory, blocks may arrive from
	// datastore in non-deterministic order.
	etag := `W/` + getEtag(r, rootCid)
	w.Header().Set("Etag", etag)

	// Finish early if Etag match
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	// Make it clear we don't support range-requests over a car stream
	// Partial downloads and resumes should be handled using requests for
	// sub-DAGs and IPLD selectors: https://github.com/ipfs/go-ipfs/issues/8769
	w.Header().Set("Accept-Ranges", "none")

	w.Header().Set("Content-Type", "application/vnd.ipld.car; version=1")
	w.Header().Set("X-Content-Type-Options", "nosniff") // no funny business in the browsers :^)

	// Same go-car settings as dag.export command
	store := dagStore{dag: i.api.Dag(), ctx: ctx}

	// TODO: support selectors passed as request param: https://github.com/ipfs/kubo/issues/8769
	dag := gocar.Dag{Root: rootCid, Selector: selectorparse.CommonSelector_ExploreAllRecursively}
	car := gocar.NewSelectiveCar(ctx, store, []gocar.Dag{dag}, gocar.TraverseLinksOnlyOnce())

	if err := car.Write(w); err != nil {
		closeConnWithError(w, err)
		return
	}

	// Update metrics
	i.carStreamGetMetric.WithLabelValues(contentPath.Namespace()).Observe(time.Since(begin).Seconds())
}

// FIXME(@Jorropo): https://github.com/ipld/go-car/issues/315
type dagStore struct {
	dag coreiface.APIDagService
	ctx context.Context
}

func (ds dagStore) Get(_ context.Context, c cid.Cid) (blocks.Block, error) {
	return ds.dag.Get(ds.ctx, c)
}
