package corehttp

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"time"

	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/go-ipfs/tracing"
	"github.com/ipfs/interface-go-ipfs-core/options"
	ipath "github.com/ipfs/interface-go-ipfs-core/path"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func (i *gatewayHandler) serveUnixFS(ctx context.Context, w http.ResponseWriter, r *http.Request, resolvedPath ipath.Resolved, contentPath ipath.Path, begin time.Time, logger *zap.SugaredLogger) {
	ctx, span := tracing.Span(ctx, "Gateway", "ServeUnixFS", trace.WithAttributes(attribute.String("path", resolvedPath.String())))
	defer span.End()

	// Decouple (potential) directory context to cancel it in case it's too big.
	directoryContext, cancelDirContext := context.WithCancel(ctx)

	// Handling UnixFS
	// FIXME: We should be using `Unixfs().Ls()` not only here but as much as
	//  possible in this function.
	dr, err := i.api.Unixfs().Get(directoryContext, resolvedPath)
	if err != nil {
		webError(w, "ipfs cat "+html.EscapeString(contentPath.String()), err, http.StatusNotFound)
		return
	}
	defer dr.Close()

	// Handling Unixfs file
	if f, ok := dr.(files.File); ok {
		logger.Debugw("serving unixfs file", "path", contentPath)
		i.serveFile(ctx, w, r, resolvedPath, contentPath, f, begin)
		return
	}

	// Handling Unixfs directory
	dir, ok := dr.(files.Directory)
	if !ok {
		internalWebError(w, fmt.Errorf("unsupported UnixFS type"))
		return
	}

	// Preemptively count directory entries to stop listing directories too big.
	directoryTooBig := make(chan struct{})
	if i.config.MaxDirectorySize > 0 {
		dirEntryChan, err := i.api.Unixfs().Ls(ctx, resolvedPath, options.Unixfs.ResolveChildren(false))
		if err != nil {
			internalWebError(w, err)
			return
		}
		directoryEntriesNumber := 0
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				select {
				case <-ctx.Done():
					return
				case <-dirEntryChan:
					directoryEntriesNumber++
					if directoryEntriesNumber >= i.config.MaxDirectorySize {
						close(directoryTooBig)
						cancelDirContext()
						return
					}
				}
			}
		}()
	}

	logger.Debugw("serving unixfs directory", "path", contentPath)
	i.serveDirectory(ctx, w, r, resolvedPath, contentPath, dir, begin, logger, directoryTooBig)
}
