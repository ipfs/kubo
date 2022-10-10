package corehttp

import (
	"context"
	"html"
	"net/http"
	"time"

	files "github.com/ipfs/go-ipfs-files"
	ipath "github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/ipfs/kubo/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var unixEpochTime = time.Unix(0, 0)

func (i *gatewayHandler) serveTAR(ctx context.Context, w http.ResponseWriter, r *http.Request, resolvedPath ipath.Resolved, contentPath ipath.Path, begin time.Time, logger *zap.SugaredLogger) {
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
	}
	setContentDispositionHeader(w, name, "attachment")

	// Get Unixfs file
	file, err := i.api.Unixfs().Get(ctx, resolvedPath)
	if err != nil {
		webError(w, "ipfs cat "+html.EscapeString(contentPath.String()), err, http.StatusNotFound)
		return
	}
	defer file.Close()

	// Construct the TAR writer
	tarw, err := files.NewTarWriter(w)
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

	w.Header().Set("Content-Type", "application/x-tar")

	// The TAR has a top-level directory (or file) named by the CID.
	if tarErr := tarw.WriteFile(file, resolvedPath.Cid().String()); tarErr != nil {
		// There are no good ways of showing an error during a stream. Therefore, we try
		// to hijack the connection to forcefully close it, causing a network error.
		hj, ok := w.(http.Hijacker)
		if !ok {
			// If we could not Hijack the connection, we write the original error. This will hopefully
			// corrupt the generated TAR file, such that the client will receive an error unpacking.
			webError(w, "could not build tar archive", tarErr, http.StatusInternalServerError)
			return
		}

		conn, _, err := hj.Hijack()
		if err != nil {
			// Deliberately pass the original tar error here instead of the hijacking error.
			webError(w, "could not build tar archive", tarErr, http.StatusInternalServerError)
			return
		}

		conn.Close()
		return
	}
}
