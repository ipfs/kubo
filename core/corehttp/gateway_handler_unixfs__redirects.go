package corehttp

import (
	"fmt"
	"io"
	"net/http"
	gopath "path"
	"strconv"
	"strings"

	redirects "github.com/ipfs-shipyard/go-ipfs-redirects"
	files "github.com/ipfs/go-ipfs-files"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	ipath "github.com/ipfs/interface-go-ipfs-core/path"
	"go.uber.org/zap"
)

// Resolve the provided path.
//
// Resolving a UnixFS path involves determining if the provided `path.Path` exists and returning the `path.Resolved`
// corresponding to that path. For UnixFS, path resolution is more involved if a `_redirects`` file exists, stored
// underneath the root CID of the path.
//
// Example 1:
// If a path exists, we always return the `path.Resolved` corresponding to that path, regardless of the existence of a `_redirects` file.
//
// Example 2:
// If a path does not exist, usually we should return a `nil` resolution path and an error indicating that the path
// doesn't exist.  However, a `_redirects` file may exist and contain a redirect rule that redirects that path to a different path.
// We need to evaluate the rule and perform the redirect if present.
//
// Example 3:
// Another possibility is that the path corresponds to a rewrite rule (i.e. a rule with a status of 200).
// In this case, we don't perform a redirect, but do need to return a `path.Resolved` and `path.Path` corresponding to
// the rewrite destination path.
//
// Note that for security reasons, redirect rules are only processed when the request has origin isolation.
func (i *gatewayHandler) handlePathResolution(w http.ResponseWriter, r *http.Request, responseFormat string, contentPath ipath.Path, logger *zap.SugaredLogger) (ipath.Resolved, ipath.Path, bool) {
	// Attempt to resolve the provided path.
	resolvedPath, err := i.api.ResolvePath(r.Context(), contentPath)

	switch err {
	case nil:
		return resolvedPath, contentPath, true
	case coreiface.ErrOffline:
		webError(w, "ipfs resolve -r "+debugStr(contentPath.String()), err, http.StatusServiceUnavailable)
		return nil, nil, false
	default:
		if isUnixfsResponseFormat(responseFormat) {
			// The path can't be resolved.
			// If we have origin isolation, attempt to handle any redirect rules.
			if hasOriginIsolation(r) {
				redirectsFile := i.getRedirectsFile(r, contentPath, logger)
				if redirectsFile != nil {
					redirectRules, err := i.getRedirectRules(r, redirectsFile)
					if err != nil {
						internalWebError(w, err)
						return nil, nil, false
					}

					redirected, newPath, err := i.handleRedirectsFileRules(w, r, contentPath, redirectRules)
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
			webError(w, "ipfs resolve -r "+debugStr(contentPath.String()), err, http.StatusBadRequest)
			return nil, nil, false
		} else {
			webError(w, "ipfs resolve -r "+debugStr(contentPath.String()), err, http.StatusNotFound)
			return nil, nil, false
		}
	}
}

func (i *gatewayHandler) handleRedirectsFileRules(w http.ResponseWriter, r *http.Request, contentPath ipath.Path, redirectRules []redirects.Rule) (bool, string, error) {
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
			if !isValidCode(rule.Status) {
				return false, "", fmt.Errorf("unsupported redirect status code: %d", rule.Status)
			}

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

			// Or 400s
			if rule.Status == 404 {
				toPath := rootPath + rule.To
				content404Path := ipath.New(toPath)
				err := i.serve404(w, r, content404Path)
				return true, toPath, err
			}

			if rule.Status == 410 {
				webError(w, "ipfs resolve -r "+debugStr(contentPath.String()), coreiface.ErrResolveFailed, http.StatusGone)
				return true, rule.To, nil
			}

			if rule.Status == 451 {
				webError(w, "ipfs resolve -r "+debugStr(contentPath.String()), coreiface.ErrResolveFailed, http.StatusUnavailableForLegalReasons)
				return true, rule.To, nil
			}

			// Or redirect
			if rule.Status >= 301 && rule.Status <= 308 {
				http.Redirect(w, r, rule.To, rule.Status)
				return true, rule.To, nil
			}
		}
	}

	// No redirects matched
	return false, "", nil
}

func isValidCode(code int) bool {
	validCodes := []int{200, 301, 302, 303, 307, 308, 404, 410, 451}
	for _, validCode := range validCodes {
		if validCode == code {
			return true
		}
	}
	return false
}

func (i *gatewayHandler) getRedirectRules(r *http.Request, redirectsFilePath ipath.Resolved) ([]redirects.Rule, error) {
	// Convert the path into a file node
	node, err := i.api.Unixfs().Get(r.Context(), redirectsFilePath)
	if err != nil {
		return nil, fmt.Errorf("could not get _redirects node: %v", err)
	}
	defer node.Close()

	// Convert the node into a file
	f, ok := node.(files.File)
	if !ok {
		return nil, fmt.Errorf("could not parse _redirects: %v", err)
	}

	// Parse redirect rules from file
	redirectRules, err := redirects.Parse(f)
	if err != nil {
		return nil, fmt.Errorf("could not parse _redirects: %v", err)
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
