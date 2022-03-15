package corehttp

import (
	"fmt"
	"html"
	"net/http"

	files "github.com/ipfs/go-ipfs-files"
	ipath "github.com/ipfs/interface-go-ipfs-core/path"
	"go.uber.org/zap"
)

func (i *gatewayHandler) serveUnixFs(w http.ResponseWriter, r *http.Request, resolvedPath ipath.Resolved, contentPath ipath.Path, logger *zap.SugaredLogger) {
	// Handling UnixFS
	dr, err := i.api.Unixfs().Get(r.Context(), resolvedPath)
	if err != nil {
		webError(w, "ipfs cat "+html.EscapeString(contentPath.String()), err, http.StatusNotFound)
		return
	}
	defer dr.Close()

	// Handling Unixfs file
	if f, ok := dr.(files.File); ok {
		logger.Debugw("serving unixfs file", "path", contentPath)
		i.serveFile(w, r, contentPath, resolvedPath.Cid(), f)
		return
	}

	// Handling Unixfs directory
	dir, ok := dr.(files.Directory)
	if !ok {
		internalWebError(w, fmt.Errorf("unsupported UnixFs type"))
		return
	}
	logger.Debugw("serving unixfs directory", "path", contentPath)
	i.serveDirectory(w, r, resolvedPath, contentPath, dir, logger)
}
