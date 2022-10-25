package corehttp

import (
	"fmt"
	"io"
	"net/http"
	gopath "path"
	"strconv"
	"strings"

	files "github.com/ipfs/go-ipfs-files"
	redirects "github.com/ipfs/go-ipfs-redirects-file"
	ipath "github.com/ipfs/interface-go-ipfs-core/path"
	"go.uber.org/zap"
)

// Resolving a UnixFS path involves determining if the provided `path.Path` exists and returning the `path.Resolved`
// corresponding to that path. For UnixFS, path resolution is more involved.
//
// When a path under requested CID does not exist, Gateway will check if a `_redirects` file exists
// underneath the root CID of the path, and apply rules defined there.
// See sepcification introduced in: https://github.com/ipfs/specs/pull/290
//
// Scenario 1:
// If a path exists, we always return the `path.Resolved` corresponding to that path, regardless of the existence of a `_redirects` file.
//
// Scenario 2:
// If a path does not exist, usually we should return a `nil` resolution path and an error indicating that the path
// doesn't exist.  However, a `_redirects` file may exist and contain a redirect rule that redirects that path to a different path.
// We need to evaluate the rule and perform the redirect if present.
//
// Scenario 3:
// Another possibility is that the path corresponds to a rewrite rule (i.e. a rule with a status of 200).
// In this case, we don't perform a redirect, but do need to return a `path.Resolved` and `path.Path` corresponding to
// the rewrite destination path.
//
// Note that for security reasons, redirect rules are only processed when the request has origin isolation.
// See https://github.com/ipfs/specs/pull/290 for more information.
func (i *gatewayHandler) serveRedirectsIfPresent(w http.ResponseWriter, r *http.Request, resolvedPath ipath.Resolved, contentPath ipath.Path, logger *zap.SugaredLogger) (newResolvedPath ipath.Resolved, newContentPath ipath.Path, continueProcessing bool, hadMatchingRule bool) {
	redirectsFile := i.getRedirectsFile(r, contentPath, logger)
	if redirectsFile != nil {
		redirectRules, err := i.getRedirectRules(r, redirectsFile)
		if err != nil {
			internalWebError(w, err)
			return nil, nil, false, true
		}

		redirected, newPath, err := i.handleRedirectsFileRules(w, r, contentPath, redirectRules)
		if err != nil {
			err = fmt.Errorf("trouble processing _redirects file at %q: %w", redirectsFile.String(), err)
			internalWebError(w, err)
			return nil, nil, false, true
		}

		if redirected {
			return nil, nil, false, true
		}

		// 200 is treated as a rewrite, so update the path and continue
		if newPath != "" {
			// Reassign contentPath and resolvedPath since the URL was rewritten
			contentPath = ipath.New(newPath)
			resolvedPath, err = i.api.ResolvePath(r.Context(), contentPath)
			if err != nil {
				internalWebError(w, err)
				return nil, nil, false, true
			}

			return resolvedPath, contentPath, true, true
		}
	}
	// No matching rule, paths remain the same, continue regular processing
	return resolvedPath, contentPath, true, false
}

func (i *gatewayHandler) handleRedirectsFileRules(w http.ResponseWriter, r *http.Request, contentPath ipath.Path, redirectRules []redirects.Rule) (redirected bool, newContentPath string, err error) {
	// Attempt to match a rule to the URL path, and perform the corresponding redirect or rewrite
	pathParts := strings.Split(contentPath.String(), "/")
	if len(pathParts) > 3 {
		// All paths should start with /ipfs/cid/, so get the path after that
		urlPath := "/" + strings.Join(pathParts[3:], "/")
		rootPath := strings.Join(pathParts[:3], "/")
		// Trim off the trailing /
		urlPath = strings.TrimSuffix(urlPath, "/")

		for _, rule := range redirectRules {
			// Error right away if the rule is invalid
			if !rule.MatchAndExpandPlaceholders(urlPath) {
				continue
			}

			// We have a match!

			// Rewrite
			if rule.Status == 200 {
				// Prepend the rootPath
				toPath := rootPath + rule.To
				return false, toPath, nil
			}

			// Or 4xx
			if rule.Status == 404 || rule.Status == 410 || rule.Status == 451 {
				toPath := rootPath + rule.To
				content4xxPath := ipath.New(toPath)
				err := i.serve4xx(w, r, content4xxPath, rule.Status)
				return true, toPath, err
			}

			// Or redirect
			if rule.Status >= 301 && rule.Status <= 308 {
				http.Redirect(w, r, rule.To, rule.Status)
				return true, "", nil
			}
		}
	}

	// No redirects matched
	return false, "", nil
}

func (i *gatewayHandler) getRedirectRules(r *http.Request, redirectsFilePath ipath.Resolved) ([]redirects.Rule, error) {
	// Convert the path into a file node
	node, err := i.api.Unixfs().Get(r.Context(), redirectsFilePath)
	if err != nil {
		return nil, fmt.Errorf("could not get _redirects: %w", err)
	}
	defer node.Close()

	// Convert the node into a file
	f, ok := node.(files.File)
	if !ok {
		return nil, fmt.Errorf("could not parse _redirects: %w", err)
	}

	// Parse redirect rules from file
	redirectRules, err := redirects.Parse(f)
	if err != nil {
		return nil, fmt.Errorf("could not parse _redirects: %w", err)
	}

	return redirectRules, nil
}

// Returns a resolved path to the _redirects file located in the root CID path of the requested path
func (i *gatewayHandler) getRedirectsFile(r *http.Request, contentPath ipath.Path, logger *zap.SugaredLogger) ipath.Resolved {
	// contentPath is the full ipfs path to the requested resource,
	// regardless of whether path or subdomain resolution is used.
	rootPath := getRootPath(contentPath)

	// Check for _redirects file.
	// Any path resolution failures are ignored and we just assume there's no _redirects file.
	// Note that ignoring these errors also ensures that the use of the empty CID (bafkqaaa) in tests doesn't fail.
	path := ipath.Join(rootPath, "_redirects")
	resolvedPath, err := i.api.ResolvePath(r.Context(), path)
	if err != nil {
		return nil
	}
	return resolvedPath
}

// Returns the root CID Path for the given path
func getRootPath(path ipath.Path) ipath.Path {
	parts := strings.Split(path.String(), "/")
	return ipath.New(gopath.Join("/", path.Namespace(), parts[2]))
}

func (i *gatewayHandler) serve4xx(w http.ResponseWriter, r *http.Request, content4xxPath ipath.Path, status int) error {
	resolved4xxPath, err := i.api.ResolvePath(r.Context(), content4xxPath)
	if err != nil {
		return err
	}

	node, err := i.api.Unixfs().Get(r.Context(), resolved4xxPath)
	if err != nil {
		return err
	}
	defer node.Close()

	f, ok := node.(files.File)
	if !ok {
		return fmt.Errorf("could not convert node for %d page to file", status)
	}

	size, err := f.Size()
	if err != nil {
		return fmt.Errorf("could not get size of %d page", status)
	}

	log.Debugf("using _redirects: custom %d file at %q", status, content4xxPath)
	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	addCacheControlHeaders(w, r, content4xxPath, resolved4xxPath.Cid())
	w.WriteHeader(status)
	_, err = io.CopyN(w, f, size)
	return err
}

func hasOriginIsolation(r *http.Request) bool {
	_, gw := r.Context().Value(requestContextKey("gw-hostname")).(string)
	_, dnslink := r.Context().Value("dnslink-hostname").(string)

	if gw || dnslink {
		return true
	}

	return false
}

func isUnixfsResponseFormat(responseFormat string) bool {
	// The implicit response format is UnixFS
	return responseFormat == ""
}

// Deprecated: legacy ipfs-404.html files are superseded by _redirects file
// This is provided only for backward-compatibility, until websites migrate
// to 404s managed via _redirects file (https://github.com/ipfs/specs/pull/290)
func (i *gatewayHandler) serveLegacy404IfPresent(w http.ResponseWriter, r *http.Request, contentPath ipath.Path) bool {
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
