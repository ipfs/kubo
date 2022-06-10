package corehttp

import (
	"compress/gzip"
	"context"
	"html"
	"io"
	"net/http"
	"time"

	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/go-ipfs/tracing"
	ipath "github.com/ipfs/interface-go-ipfs-core/path"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var unixEpochTime = time.Unix(0, 0)

func (i *gatewayHandler) serveTAR(ctx context.Context, w http.ResponseWriter, r *http.Request, resolvedPath ipath.Resolved, contentPath ipath.Path, begin time.Time, logger *zap.SugaredLogger, compressed bool) {
	ctx, span := tracing.Span(ctx, "Gateway", "ServeTAR", trace.WithAttributes(attribute.String("path", resolvedPath.String())))
	defer span.End()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Set Cache-Control and read optional Last-Modified time
	modtime := addCacheControlHeaders(w, r, contentPath, resolvedPath.Cid())

	// Finish early if Etag match
	if r.Header.Get("If-None-Match") == getEtag(r, resolvedPath.Cid()) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	// Set Content-Disposition
	var name string
	if urlFilename := r.URL.Query().Get("filename"); urlFilename != "" {
		name = urlFilename
	} else {
		name = resolvedPath.Cid().String() + ".tar"
		if compressed {
			name += ".gz"
		}
	}
	setContentDispositionHeader(w, name, "attachment")

	// Get Unixfs file
	file, err := i.api.Unixfs().Get(ctx, resolvedPath)
	if err != nil {
		webError(w, "ipfs cat "+html.EscapeString(contentPath.String()), err, http.StatusNotFound)
		return
	}
	defer file.Close()

	// Define the output writer, maybe build a Gzip writer
	var dstw io.Writer
	if compressed {
		gzipw := gzip.NewWriter(w)
		defer gzipw.Close()

		dstw = gzipw
	} else {
		dstw = w
	}

	// Construct the TAR writer
	tarw, err := files.NewTarWriter(dstw)
	if err != nil {
		webError(w, "could not build tar writer", err, http.StatusInternalServerError)
		return
	}
	defer tarw.Close()

	// Sets correct Last-Modified header. This code is borrowed from the standard
	// library (net/http/server.go) as we cannot use serveFile without throwing the entire
	// TAR into the memory first.
	if !(modtime.IsZero() || modtime.Equal(unixEpochTime)) {
		w.Header().Set("Last-Modified", modtime.UTC().Format(http.TimeFormat))
	}

	responseFormat, _, _ := customResponseFormat(r)
	w.Header().Set("Content-Type", responseFormat)

	if err := tarw.WriteFile(file, name); err != nil {
		// We return error as a trailer, however it is not something browsers can access
		// (https://github.com/mdn/browser-compat-data/issues/14703)
		// Due to this, we suggest client always verify that
		// the received CAR stream response is matching requested DAG selector
		w.Header().Set("X-Stream-Error", err.Error())
		return
	}
}
