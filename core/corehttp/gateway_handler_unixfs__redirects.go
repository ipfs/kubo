package corehttp

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	gopath "path"
	"strconv"
	"strings"

	"github.com/fission-suite/go-redirects"
	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/go-path/resolver"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	ipath "github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/ucarion/urlpath"
	"go.uber.org/zap"
)

// Resolve the provided path.
//
// Resolving a UnixFS path involves determining if the provided `path.Path` exists and returning the `path.Resolved` corresponding to that path.
// For UnixFS, path resolution is more involved if a `_redirects`` file exists stored under the root CID of the path.
//
// Example 1:
// If a path exists, usually we should return the `path.Resolved` corresponding to that path.
// However, the `_redirects` file may contain a forced redirect rule corresponding to the path (i.e. a rule with a `!` after the status code).
// Forced redirect rules must be evaluated, even if the path exists, thus overriding the path.
//
// Example 2:
// If a path does not exist, usually we should return a nil resolution path and an error indicating that the path doesn't exist.
// However, the `_redirects` file may contain a redirect rule that redirects that path to a different path.
// We need to evaluate the rule and perform the redirect if present.
//
// Example 3:
// Another possibility is that the path corresponds to a rewrite rule (i.e. a rule with a status of 200).
// In this case, we don't perform a redirect, but do need to return a `path.Resolved` and `path.Path` corresponding to the rewrite destination path.
//
// Note that for security reasons, redirect rules are only processed when the request has origin isolation.
//
func (i *gatewayHandler) handleUnixfsPathResolution(w http.ResponseWriter, r *http.Request, contentPath ipath.Path, logger *zap.SugaredLogger) (ipath.Resolved, ipath.Path, bool) {
	// Attempt to resolve the path for the provided contentPath
	resolvedPath, err := i.api.ResolvePath(r.Context(), contentPath)

	// If path resolved and we have origin isolation, we need to attempt to read redirects file and find a force redirect for the corresponding path.  If found, redirect, but only if force.
	// If path resolved and we do not have origin isolation, no need to attempt to read redirects file.  Just return resolvedPath, contentPath, true.
	// If path didn't resolve, if ErrOffline, write error and return nil, nil, false.
	// If path didn't resolve for any other error, if we have origin isolation, attempt to read redirects file and apply any redirect rules, regardless of force.
	// Fallback to pretty 404 page, and then normal 404

	switch err {
	case nil:
		// TODO: I believe for the force option, we might need to short circuit this, and thus we would need to read the redirects file first
		return resolvedPath, contentPath, true
	case coreiface.ErrOffline:
		webError(w, "ipfs resolve -r "+debugStr(contentPath.String()), err, http.StatusServiceUnavailable)
		return nil, nil, false
	default:
		// If we can't resolve the path
		// Only look for _redirects file if we have Unixfs and Origin isolation
		if hasOriginIsolation(r) {
			// Check for _redirects file and redirect as needed
			// /ipfs/CID/a/b/c/
			// /ipfs/CID/_redirects
			// /ipns/domain/ipfs/CID
			// /ipns/domain
			logger.Debugf("r.URL.Path=%v", r.URL.Path)
			redirectsFile, err := i.getRedirectsFile(r)

			if err != nil {
				switch err.(type) {
				case resolver.ErrNoLink:
					// _redirects files doesn't exist, so don't error
				// case coreiface.ErrResolveFailed.(type):
				// How to get type?
				// 	// Couldn't resolve ipns name when trying to compute root
				// Tests indicate we should return 404, not 500
				// 	internalWebError(w, err)
				// 	return nil, nil, false
				default:
					// Let users know about issues with _redirects file handling
					internalWebError(w, err)
					return nil, nil, false
				}
			} else {
				// _redirects file exists, so parse it and redirect
				redirected, newPath, err := i.handleRedirectsFile(w, r, redirectsFile, logger)
				if err != nil {
					err = fmt.Errorf("trouble processing _redirects file at %q: %w", redirectsFile.String(), err)
					internalWebError(w, err)
					return nil, nil, false
				}

				if redirected {
					return nil, nil, false
				}

				// 200 is treated as a rewrite, so update the path and continue
				if newPath != "" {
					// Reassign contentPath and resolvedPath since the URL was rewritten
					contentPath = ipath.New(newPath)
					resolvedPath, err = i.api.ResolvePath(r.Context(), contentPath)
					if err != nil {
						internalWebError(w, err)
						return nil, nil, false
					}
					logger.Debugf("_redirects: 200 rewrite. newPath=%v", newPath)

					return resolvedPath, contentPath, true
				}
			}
		}

		// if Accept is text/html, see if ipfs-404.html is present
		// This logic isn't documented and will likely be removed at some point.
		// Any 404 logic in _redirects above will have already run by this time, so it's really an extra fall back
		if i.servePretty404IfPresent(w, r, contentPath) {
			logger.Debugw("serve pretty 404 if present")
			return nil, nil, false
		}

		// Fallback
		webError(w, "ipfs resolve -r "+debugStr(contentPath.String()), err, http.StatusNotFound)
		return nil, nil, false
	}
}

func (i *gatewayHandler) handleRedirectsFile(w http.ResponseWriter, r *http.Request, redirectsFilePath ipath.Resolved, logger *zap.SugaredLogger) (bool, string, error) {
	// Convert the path into a file node
	node, err := i.api.Unixfs().Get(r.Context(), redirectsFilePath)
	if err != nil {
		return false, "", fmt.Errorf("could not get _redirects node: %v", err)
	}
	defer node.Close()

	// Convert the node into a file
	f, ok := node.(files.File)
	if !ok {
		return false, "", fmt.Errorf("could not convert _redirects node to file")
	}

	// Parse redirect rules from file
	redirectRules, err := redirects.Parse(f)
	if err != nil {
		return false, "", fmt.Errorf("could not parse redirect rules: %v", err)
	}
	logger.Debugf("redirectRules=%v", redirectRules)

	// Attempt to match a rule to the URL path, and perform the corresponding redirect or rewrite
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) > 3 {
		// All paths should start with /ipfs/cid/, so get the path after that
		urlPath := "/" + strings.Join(pathParts[3:], "/")
		rootPath := strings.Join(pathParts[:3], "/")
		// Trim off the trailing /
		urlPath = strings.TrimSuffix(urlPath, "/")

		logger.Debugf("_redirects: urlPath=", urlPath)
		for _, rule := range redirectRules {
			// get rule.From, trim trailing slash, ...
			fromPath := urlpath.New(strings.TrimSuffix(rule.From, "/"))
			logger.Debugf("_redirects: fromPath=%v", strings.TrimSuffix(rule.From, "/"))
			match, ok := fromPath.Match(urlPath)
			if !ok {
				continue
			}

			// We have a match!  Perform substitutions.
			toPath := rule.To
			toPath = replacePlaceholders(toPath, match)
			toPath = replaceSplat(toPath, match)

			logger.Debugf("_redirects: toPath=%v", toPath)

			// Rewrite
			if rule.Status == 200 {
				// Prepend the rootPath
				toPath = rootPath + rule.To
				return false, toPath, nil
			}

			// Or 404
			if rule.Status == 404 {
				toPath = rootPath + rule.To
				content404Path := ipath.New(toPath)
				err = i.serve404(w, r, content404Path)
				return true, toPath, err
			}

			// Or redirect
			http.Redirect(w, r, toPath, rule.Status)
			return true, toPath, nil
		}
	}

	// No redirects matched
	return false, "", nil
}

func replacePlaceholders(to string, match urlpath.Match) string {
	if len(match.Params) > 0 {
		for key, value := range match.Params {
			to = strings.ReplaceAll(to, ":"+key, value)
		}
	}

	return to
}

func replaceSplat(to string, match urlpath.Match) string {
	return strings.ReplaceAll(to, ":splat", match.Trailing)
}

// Returns a resolved path to the _redirects file located in the root CID path of the requested path
func (i *gatewayHandler) getRedirectsFile(r *http.Request) (ipath.Resolved, error) {
	// r.URL.Path is the full ipfs path to the requested resource,
	// regardless of whether path or subdomain resolution is used.
	rootPath, err := getRootPath(r.URL.Path)
	if err != nil {
		return nil, err
	}

	path := ipath.Join(rootPath, "_redirects")
	resolvedPath, err := i.api.ResolvePath(r.Context(), path)
	if err != nil {
		return nil, err
	}
	return resolvedPath, nil
}

// Returns the root CID Path for the given path
//   /ipfs/CID/*
//     CID is the root CID
//   /ipns/domain/*
//     Need to resolve domain ipns path to get CID
//   /ipns/domain/ipfs/CID
//     Is this legit?  If so, we should use CID?
func getRootPath(path string) (ipath.Path, error) {
	if isIpfsPath(path) {
		parts := strings.Split(path, "/")
		return ipath.New(gopath.Join(ipfsPathPrefix, parts[2])), nil
	}

	if isIpnsPath(path) {
		parts := strings.Split(path, "/")
		return ipath.New(gopath.Join(ipnsPathPrefix, parts[2])), nil
	}

	return ipath.New(""), errors.New("failed to get root CID path")
}

func (i *gatewayHandler) serve404(w http.ResponseWriter, r *http.Request, content404Path ipath.Path) error {
	resolved404Path, err := i.api.ResolvePath(r.Context(), content404Path)
	if err != nil {
		return err
	}

	node, err := i.api.Unixfs().Get(r.Context(), resolved404Path)
	if err != nil {
		return err
	}
	defer node.Close()

	f, ok := node.(files.File)
	if !ok {
		return fmt.Errorf("could not convert node for 404 page to file")
	}

	size, err := f.Size()
	if err != nil {
		return fmt.Errorf("could not get size of 404 page")
	}

	log.Debugw("using _redirects 404 file", "path", content404Path)
	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.WriteHeader(http.StatusNotFound)
	_, err = io.CopyN(w, f, size)
	return err
}

// TODO(JJ): Confirm approach
func hasOriginIsolation(r *http.Request) bool {
	_, gw := r.Context().Value("gw-hostname").(string)
	_, dnslink := r.Context().Value("dnslink-hostname").(string)

	if gw || dnslink {
		return true
	}

	return false
}

func isIpfsPath(path string) bool {
	if strings.HasPrefix(path, ipfsPathPrefix) && strings.Count(gopath.Clean(path), "/") >= 2 {
		return true
	}

	return false
}

func isIpnsPath(path string) bool {
	if strings.HasPrefix(path, ipnsPathPrefix) && strings.Count(gopath.Clean(path), "/") >= 2 {
		return true
	}

	return false
}

func isUnixfsResponseFormat(responseFormat string) bool {
	// The implicit response format is UnixFS
	return responseFormat == ""
}
