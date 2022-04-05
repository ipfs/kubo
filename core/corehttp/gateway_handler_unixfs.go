package corehttp

import (
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	gopath "path"
	"strconv"
	"strings"
	"time"

	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/go-ipfs/tracing"
	"github.com/ipfs/go-path/resolver"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	ipath "github.com/ipfs/interface-go-ipfs-core/path"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func (i *gatewayHandler) getOrHeadHandlerUnixfs(w http.ResponseWriter, r *http.Request, begin time.Time, logger *zap.SugaredLogger) {
	urlPath := r.URL.Path

	// Only look for _redirects file if we have Origin isolation
	if hasOriginIsolation(r) {
		// Check for _redirects file and redirect as needed
		redirectsFile, err := i.getRedirectsFile(r)
		if err != nil {
			switch err.(type) {
			case resolver.ErrNoLink:
				// _redirects files doesn't exist, so don't error
			default:
				// TODO(JJ): During tests we get multibase.ErrUnsupportedEncoding
				// This comes from multibase and I assume is due to a fake or otherwise bad CID being in the test.
				// So for now any errors getting the redirect file are silently ignored.
				// internalWebError(w, err)
				// return
			}
		} else {
			// _redirects file exists, so parse it and redirect
			redirected, newPath, err := i.redirect(w, r, redirectsFile)
			if err != nil {
				// TODO(JJ): How should we handle parse or redirect errors?
				internalWebError(w, err)
				return
			}

			if redirected {
				return
			}

			// 200 is treated as a rewrite, so update the path and continue
			if newPath != "" {
				urlPath = newPath
			}
		}
	}

	contentPath := ipath.New(urlPath)

	resolvedPath, err := i.api.ResolvePath(r.Context(), contentPath)

	switch err {
	case nil:
	case coreiface.ErrOffline:
		webError(w, "ipfs resolve -r "+debugStr(contentPath.String()), err, http.StatusServiceUnavailable)
		return
	default:
		// if Accept is text/html, see if ipfs-404.html is present
		if i.servePretty404IfPresent(w, r, contentPath) {
			logger.Debugw("serve pretty 404 if present")
			return
		}

		webError(w, "ipfs resolve -r "+debugStr(contentPath.String()), err, http.StatusNotFound)
		return
	}

	if i.finishEarlyForMatchingETag(w, r, resolvedPath) {
		return
	}

	if !i.updateGlobalMetrics(w, r, begin, contentPath, resolvedPath) {
		return
	}

	if !i.setHeaders(w, r, contentPath) {
		return
	}

	logger.Debugw("serving unixfs", "path", contentPath)
	i.serveUnixfs(w, r, resolvedPath, contentPath, begin, logger)
	return
}

func (i *gatewayHandler) servePretty404IfPresent(w http.ResponseWriter, r *http.Request, contentPath ipath.Path) bool {
	resolved404Path, ctype, err := i.searchUpTreeFor404(r, contentPath)
	if err != nil {
		return false
	}

	dr, err := i.api.Unixfs().Get(r.Context(), resolved404Path)
	if err != nil {
		return false
	}
	defer dr.Close()

	f, ok := dr.(files.File)
	if !ok {
		return false
	}

	size, err := f.Size()
	if err != nil {
		return false
	}

	log.Debugw("using pretty 404 file", "path", contentPath)
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.WriteHeader(http.StatusNotFound)
	_, err = io.CopyN(w, f, size)
	return err == nil
}

func (i *gatewayHandler) serveUnixfs(w http.ResponseWriter, r *http.Request, resolvedPath ipath.Resolved, contentPath ipath.Path, begin time.Time, logger *zap.SugaredLogger) {
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
		internalWebError(w, fmt.Errorf("unsupported Unixfs type"))
		return
	}
	logger.Debugw("serving unixfs directory", "path", contentPath)
	i.serveDirectory(w, r, resolvedPath, contentPath, dir, begin, logger)
}

// redirect returns redirected, newPath (if rewrite), error
func (i *gatewayHandler) redirect(w http.ResponseWriter, r *http.Request, path ipath.Resolved) (bool, string, error) {
	node, err := i.api.Unixfs().Get(r.Context(), path)
	if err != nil {
		return false, "", fmt.Errorf("could not get redirects file: %v", err)
	}

	defer node.Close()

	f, ok := node.(files.File)

	if !ok {
		return false, "", fmt.Errorf("redirect, could not convert node to file")
	}

	redirs := newRedirs(f)

	// extract "file" part of URL, typically the part after /ipfs/CID/...
	g := strings.Split(r.URL.Path, "/")

	if len(g) > 3 {
		filePartPath := "/" + strings.Join(g[3:], "/")

		to, code := redirs.search(filePartPath)
		if code > 0 {
			if code == http.StatusOK {
				// rewrite
				newPath := strings.Join(g[0:3], "/") + "/" + to
				return false, newPath, nil
			}

			// redirect
			http.Redirect(w, r, to, code)
			return true, "", nil
		}
	}

	return false, "", nil
}

// Returns a resolved path to the _redirects file located in the root CID path of the requested path
func (i *gatewayHandler) getRedirectsFile(r *http.Request) (ipath.Resolved, error) {
	// r.URL.Path is the full ipfs path to the requested resource,
	// regardless of whether path or subdomain resolution is used.
	rootPath, err := getRootPath(r.URL.Path)
	if err != nil {
		return nil, err
	}

	path := ipath.New(gopath.Join(rootPath, "_redirects"))
	resolvedPath, err := i.api.ResolvePath(r.Context(), path)
	if err != nil {
		return nil, err
	}
	return resolvedPath, nil
}

// Returns the root CID path for the given path
func getRootPath(path string) (string, error) {
	if strings.HasPrefix(path, ipfsPathPrefix) && strings.Count(gopath.Clean(path), "/") >= 2 {
		parts := strings.Split(path, "/")
		return gopath.Join(ipfsPathPrefix, parts[2]), nil
	} else {
		return "", errors.New("failed to get root CID path")
	}
}

// TODO(JJ): I was thinking about changing this to just look at the root path as well, but the docs say it searches up
func (i *gatewayHandler) searchUpTreeFor404(r *http.Request, contentPath ipath.Path) (ipath.Resolved, string, error) {
	filename404, ctype, err := preferred404Filename(r.Header.Values("Accept"))
	if err != nil {
		return nil, "", err
	}

	pathComponents := strings.Split(contentPath.String(), "/")

	for idx := len(pathComponents); idx >= 3; idx-- {
		pretty404 := gopath.Join(append(pathComponents[0:idx], filename404)...)
		parsed404Path := ipath.New("/" + pretty404)
		if parsed404Path.IsValid() != nil {
			break
		}
		resolvedPath, err := i.api.ResolvePath(r.Context(), parsed404Path)
		if err != nil {
			continue
		}
		return resolvedPath, ctype, nil
	}

	return nil, "", fmt.Errorf("no pretty 404 in any parent folder")
}

func preferred404Filename(acceptHeaders []string) (string, string, error) {
	// If we ever want to offer a 404 file for a different content type
	// then this function will need to parse q weightings, but for now
	// the presence of anything matching HTML is enough.
	for _, acceptHeader := range acceptHeaders {
		accepted := strings.Split(acceptHeader, ",")
		for _, spec := range accepted {
			contentType := strings.SplitN(spec, ";", 1)[0]
			switch contentType {
			case "*/*", "text/*", "text/html":
				return "ipfs-404.html", "text/html", nil
			}
		}
	}

	return "", "", fmt.Errorf("there is no 404 file for the requested content types")
}

// TODO(JJ): Pretty sure this is incorrect.  Validate the correct approach.
func hasOriginIsolation(r *http.Request) bool {
	if _, ok := r.Context().Value("gw-hostname").(string); ok {
		return true
	} else {
		return false
	}
}
