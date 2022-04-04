package corehttp

import (
	"fmt"
	"html"
	"net/http"
	"time"

	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/go-ipfs/tracing"
	ipath "github.com/ipfs/interface-go-ipfs-core/path"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func (i *gatewayHandler) serveUnixFs(w http.ResponseWriter, r *http.Request, resolvedPath ipath.Resolved, contentPath ipath.Path, begin time.Time, logger *zap.SugaredLogger) {
	ctx, span := tracing.Span(r.Context(), "Gateway", "ServeUnixFs", trace.WithAttributes(attribute.String("path", resolvedPath.String())))
	defer span.End()
	// Handling UnixFS
	dr, err := i.api.Unixfs().Get(ctx, resolvedPath)
	if err != nil {
		webError(w, "ipfs cat "+html.EscapeString(contentPath.String()), err, http.StatusNotFound)
		return
	}
	defer dr.Close()

	// Handling Unixfs file
	if f, ok := dr.(files.File); ok {
		logger.Debugw("serving unixfs file", "path", contentPath)
		i.serveFile(w, r, resolvedPath, contentPath, f, begin)
		return
	}

	// Handling Unixfs directory
	dir, ok := dr.(files.Directory)
	if !ok {
		internalWebError(w, fmt.Errorf("unsupported UnixFs type"))
		return
	}
	logger.Debugw("serving unixfs directory", "path", contentPath)
	i.serveDirectory(w, r, resolvedPath, contentPath, dir, begin, logger)
}
