package corehttp

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	gopath "path"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/gabriel-vasile/mimetype"
	"github.com/ipfs/go-cid"
	files "github.com/ipfs/go-ipfs-files"
	dag "github.com/ipfs/go-merkledag"
	mfs "github.com/ipfs/go-mfs"
	path "github.com/ipfs/go-path"
	"github.com/ipfs/go-path/resolver"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	ipath "github.com/ipfs/interface-go-ipfs-core/path"
	routing "github.com/libp2p/go-libp2p-core/routing"
)

const (
	ipfsPathPrefix = "/ipfs/"
	ipnsPathPrefix = "/ipns/"
)

// gatewayHandler is a HTTP handler that serves IPFS objects (accessible by default at /ipfs/<path>)
// (it serves requests like GET /ipfs/QmVRzPKPzNtSrEzBFm2UZfxmPAgnaLke4DMcerbsGGSaFe/link)
type gatewayHandler struct {
	config GatewayConfig
	api    coreiface.CoreAPI
}

// StatusResponseWriter enables us to override HTTP Status Code passed to
// WriteHeader function inside of http.ServeContent.  Decision is based on
// presence of HTTP Headers such as Location.
type statusResponseWriter struct {
	http.ResponseWriter
}

func (sw *statusResponseWriter) WriteHeader(code int) {
	// Check if we need to adjust Status Code to account for scheduled redirect
	// This enables us to return payload along with HTTP 301
	// for subdomain redirect in web browsers while also returning body for cli
	// tools which do not follow redirects by default (curl, wget).
	redirect := sw.ResponseWriter.Header().Get("Location")
	if redirect != "" && code == http.StatusOK {
		code = http.StatusMovedPermanently
	}
	sw.ResponseWriter.WriteHeader(code)
}

func newGatewayHandler(c GatewayConfig, api coreiface.CoreAPI) *gatewayHandler {
	i := &gatewayHandler{
		config: c,
		api:    api,
	}
	return i
}

func parseIpfsPath(p string) (cid.Cid, string, error) {
	rootPath, err := path.ParsePath(p)
	if err != nil {
		return cid.Cid{}, "", err
	}

	// Check the path.
	rsegs := rootPath.Segments()
	if rsegs[0] != "ipfs" {
		return cid.Cid{}, "", fmt.Errorf("WritableGateway: only ipfs paths supported")
	}

	rootCid, err := cid.Decode(rsegs[1])
	if err != nil {
		return cid.Cid{}, "", err
	}

	return rootCid, path.Join(rsegs[2:]), nil
}

func (i *gatewayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// the hour is a hard fallback, we don't expect it to happen, but just in case
	ctx, cancel := context.WithTimeout(r.Context(), time.Hour)
	defer cancel()
	r = r.WithContext(ctx)

	defer func() {
		if r := recover(); r != nil {
			log.Error("A panic occurred in the gateway handler!")
			log.Error(r)
			debug.PrintStack()
		}
	}()

	if i.config.Writable {
		switch r.Method {
		case http.MethodPost:
			i.postHandler(w, r)
			return
		case http.MethodPut:
			i.putHandler(w, r)
			return
		case http.MethodDelete:
			i.deleteHandler(w, r)
			return
		}
	}

	switch r.Method {
	case http.MethodGet, http.MethodHead:
		i.getOrHeadHandler(w, r)
		return
	case http.MethodOptions:
		i.optionsHandler(w, r)
		return
	}

	errmsg := "Method " + r.Method + " not allowed: "
	var status int
	if !i.config.Writable {
		status = http.StatusMethodNotAllowed
		errmsg = errmsg + "read only access"
		w.Header().Add("Allow", http.MethodGet)
		w.Header().Add("Allow", http.MethodHead)
		w.Header().Add("Allow", http.MethodOptions)
	} else {
		status = http.StatusBadRequest
		errmsg = errmsg + "bad request for " + r.URL.Path
	}
	http.Error(w, errmsg, status)
}

func (i *gatewayHandler) optionsHandler(w http.ResponseWriter, r *http.Request) {
	/*
		OPTIONS is a noop request that is used by the browsers to check
		if server accepts cross-site XMLHttpRequest (indicated by the presence of CORS headers)
		https://developer.mozilla.org/en-US/docs/Web/HTTP/Access_control_CORS#Preflighted_requests
	*/
	i.addUserHeaders(w) // return all custom headers (including CORS ones, if set)
}

func (i *gatewayHandler) getOrHeadHandler(w http.ResponseWriter, r *http.Request) {
	begin := time.Now()
	urlPath := r.URL.Path
	escapedURLPath := r.URL.EscapedPath()

	// If the gateway is behind a reverse proxy and mounted at a sub-path,
	// the prefix header can be set to signal this sub-path.
	// It will be prepended to links in directory listings and the index.html redirect.
	prefix := ""
	if prfx := r.Header.Get("X-Ipfs-Gateway-Prefix"); len(prfx) > 0 {
		for _, p := range i.config.PathPrefixes {
			if prfx == p || strings.HasPrefix(prfx, p+"/") {
				prefix = prfx
				break
			}
		}
	}

	// HostnameOption might have constructed an IPNS/IPFS path using the Host header.
	// In this case, we need the original path for constructing redirects
	// and links that match the requested URL.
	// For example, http://example.net would become /ipns/example.net, and
	// the redirects and links would end up as http://example.net/ipns/example.net
	requestURI, err := url.ParseRequestURI(r.RequestURI)
	if err != nil {
		webError(w, "failed to parse request path", err, http.StatusInternalServerError)
		return
	}
	originalUrlPath := prefix + requestURI.Path

	// Service Worker registration request
	if r.Header.Get("Service-Worker") == "script" {
		// Disallow Service Worker registration on namespace roots
		// https://github.com/ipfs/go-ipfs/issues/4025
		matched, _ := regexp.MatchString(`^/ip[fn]s/[^/]+$`, r.URL.Path)
		if matched {
			err := fmt.Errorf("registration is not allowed for this scope")
			webError(w, "navigator.serviceWorker", err, http.StatusBadRequest)
			return
		}
	}

	parsedPath := ipath.New(urlPath)
	if err := parsedPath.IsValid(); err != nil {
		webError(w, "invalid ipfs path", err, http.StatusBadRequest)
		return
	}

	// Resolve path to the final DAG node for the ETag
	resolvedPath, err := i.api.ResolvePath(r.Context(), parsedPath)
	switch err {
	case nil:
	case coreiface.ErrOffline:
		webError(w, "ipfs resolve -r "+escapedURLPath, err, http.StatusServiceUnavailable)
		return
	default:
		webError(w, "ipfs resolve -r "+escapedURLPath, err, http.StatusNotFound)
		return
	}

	dr, err := i.api.Unixfs().Get(r.Context(), resolvedPath)
	if err != nil {
		webError(w, "ipfs cat "+escapedURLPath, err, http.StatusNotFound)
		return
	}

	unixfsGetMetric.WithLabelValues(parsedPath.Namespace()).Observe(time.Since(begin).Seconds())

	defer dr.Close()

	// Check etag send back to us
	etag := "\"" + resolvedPath.Cid().String() + "\""
	if r.Header.Get("If-None-Match") == etag || r.Header.Get("If-None-Match") == "W/"+etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	i.addUserHeaders(w) // ok, _now_ write user's headers.
	w.Header().Set("X-IPFS-Path", urlPath)
	w.Header().Set("Etag", etag)

	// set these headers _after_ the error, for we may just not have it
	// and don't want the client to cache a 500 response...
	// and only if it's /ipfs!
	// TODO: break this out when we split /ipfs /ipns routes.
	modtime := time.Now()

	if f, ok := dr.(files.File); ok {
		if strings.HasPrefix(urlPath, ipfsPathPrefix) {
			w.Header().Set("Cache-Control", "public, max-age=29030400, immutable")

			// set modtime to a really long time ago, since files are immutable and should stay cached
			modtime = time.Unix(1, 0)
		}

		urlFilename := r.URL.Query().Get("filename")
		var name string
		if urlFilename != "" {
			w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename*=UTF-8''%s", url.PathEscape(urlFilename)))
			name = urlFilename
		} else {
			name = getFilename(urlPath)
		}
		i.serveFile(w, r, name, modtime, f)
		return
	}
	dir, ok := dr.(files.Directory)
	if !ok {
		internalWebError(w, fmt.Errorf("unsupported file type"))
		return
	}

	idx, err := i.api.Unixfs().Get(r.Context(), ipath.Join(resolvedPath, "index.html"))
	switch err.(type) {
	case nil:
		dirwithoutslash := urlPath[len(urlPath)-1] != '/'
		goget := r.URL.Query().Get("go-get") == "1"
		if dirwithoutslash && !goget {
			// See comment above where originalUrlPath is declared.
			http.Redirect(w, r, originalUrlPath+"/", 302)
			return
		}

		f, ok := idx.(files.File)
		if !ok {
			internalWebError(w, files.ErrNotReader)
			return
		}

		// write to request
		i.serveFile(w, r, "index.html", modtime, f)
		return
	case resolver.ErrNoLink:
		// no index.html; noop
	default:
		internalWebError(w, err)
		return
	}

	if r.Method == http.MethodHead {
		return
	}

	// storage for directory listing
	var dirListing []directoryItem
	dirit := dir.Entries()
	for dirit.Next() {
		size := "?"
		if s, err := dirit.Node().Size(); err == nil {
			// Size may not be defined/supported. Continue anyways.
			size = humanize.Bytes(uint64(s))
		}

		// See comment above where originalUrlPath is declared.
		di := directoryItem{size, dirit.Name(), gopath.Join(originalUrlPath, dirit.Name())}
		dirListing = append(dirListing, di)
	}
	if dirit.Err() != nil {
		internalWebError(w, dirit.Err())
		return
	}

	// construct the correct back link
	// https://github.com/ipfs/go-ipfs/issues/1365
	var backLink string = originalUrlPath

	// don't go further up than /ipfs/$hash/
	pathSplit := path.SplitList(urlPath)
	switch {
	// keep backlink
	case len(pathSplit) == 3: // url: /ipfs/$hash

	// keep backlink
	case len(pathSplit) == 4 && pathSplit[3] == "": // url: /ipfs/$hash/

	// add the correct link depending on whether the path ends with a slash
	default:
		if strings.HasSuffix(backLink, "/") {
			backLink += "./.."
		} else {
			backLink += "/.."
		}
	}

	hash := resolvedPath.Cid().String()

	// See comment above where originalUrlPath is declared.
	tplData := listingTemplateData{
		Listing:  dirListing,
		Path:     urlPath,
		BackLink: backLink,
		Hash:     hash,
	}

	// See statusResponseWriter.WriteHeader
	// and https://github.com/ipfs/go-ipfs/issues/7164
	// Note: this needs to occur before listingTemplate.Execute otherwise we get
	// superfluous response.WriteHeader call from prometheus/client_golang
	if w.Header().Get("Location") != "" {
		w.WriteHeader(http.StatusMovedPermanently)
		return
	}

	err = listingTemplate.Execute(w, tplData)
	if err != nil {
		internalWebError(w, err)
		return
	}
}

func (i *gatewayHandler) serveFile(w http.ResponseWriter, req *http.Request, name string, modtime time.Time, file files.File) {
	size, err := file.Size()
	if err != nil {
		http.Error(w, "cannot serve files with unknown sizes", http.StatusBadGateway)
		return
	}

	content := &lazySeeker{
		size:   size,
		reader: file,
	}

	var ctype string
	if _, isSymlink := file.(*files.Symlink); isSymlink {
		// We should be smarter about resolving symlinks but this is the
		// "most correct" we can be without doing that.
		ctype = "inode/symlink"
	} else {
		ctype = mime.TypeByExtension(gopath.Ext(name))
		if ctype == "" {
			// uses https://github.com/gabriel-vasile/mimetype library to determine the content type.
			// Fixes https://github.com/ipfs/go-ipfs/issues/7252
			mimeType, err := mimetype.DetectReader(content)
			if err != nil {
				http.Error(w, fmt.Sprintf("cannot detect content-type: %s", err.Error()), http.StatusInternalServerError)
				return
			}

			ctype = mimeType.String()
			_, err = content.Seek(0, io.SeekStart)
			if err != nil {
				http.Error(w, "seeker can't seek", http.StatusInternalServerError)
				return
			}
		}
		// Strip the encoding from the HTML Content-Type header and let the
		// browser figure it out.
		//
		// Fixes https://github.com/ipfs/go-ipfs/issues/2203
		if strings.HasPrefix(ctype, "text/html;") {
			ctype = "text/html"
		}
	}
	w.Header().Set("Content-Type", ctype)

	w = &statusResponseWriter{w}
	http.ServeContent(w, req, name, modtime, content)
}

func (i *gatewayHandler) postHandler(w http.ResponseWriter, r *http.Request) {
	p, err := i.api.Unixfs().Add(r.Context(), files.NewReaderFile(r.Body))
	if err != nil {
		internalWebError(w, err)
		return
	}

	i.addUserHeaders(w) // ok, _now_ write user's headers.
	w.Header().Set("IPFS-Hash", p.Cid().String())
	http.Redirect(w, r, p.String(), http.StatusCreated)
}

func (i *gatewayHandler) putHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ds := i.api.Dag()

	// Parse the path
	rootCid, newPath, err := parseIpfsPath(r.URL.Path)
	if err != nil {
		webError(w, "WritableGateway: failed to parse the path", err, http.StatusBadRequest)
		return
	}
	if newPath == "" || newPath == "/" {
		http.Error(w, "WritableGateway: empty path", http.StatusBadRequest)
		return
	}
	newDirectory, newFileName := gopath.Split(newPath)

	// Resolve the old root.

	rnode, err := ds.Get(ctx, rootCid)
	if err != nil {
		webError(w, "WritableGateway: Could not create DAG from request", err, http.StatusInternalServerError)
		return
	}

	pbnd, ok := rnode.(*dag.ProtoNode)
	if !ok {
		webError(w, "Cannot read non protobuf nodes through gateway", dag.ErrNotProtobuf, http.StatusBadRequest)
		return
	}

	// Create the new file.
	newFilePath, err := i.api.Unixfs().Add(ctx, files.NewReaderFile(r.Body))
	if err != nil {
		webError(w, "WritableGateway: could not create DAG from request", err, http.StatusInternalServerError)
		return
	}

	newFile, err := ds.Get(ctx, newFilePath.Cid())
	if err != nil {
		webError(w, "WritableGateway: failed to resolve new file", err, http.StatusInternalServerError)
		return
	}

	// Patch the new file into the old root.

	root, err := mfs.NewRoot(ctx, ds, pbnd, nil)
	if err != nil {
		webError(w, "WritableGateway: failed to create MFS root", err, http.StatusBadRequest)
		return
	}

	if newDirectory != "" {
		err := mfs.Mkdir(root, newDirectory, mfs.MkdirOpts{Mkparents: true, Flush: false})
		if err != nil {
			webError(w, "WritableGateway: failed to create MFS directory", err, http.StatusInternalServerError)
			return
		}
	}
	dirNode, err := mfs.Lookup(root, newDirectory)
	if err != nil {
		webError(w, "WritableGateway: failed to lookup directory", err, http.StatusInternalServerError)
		return
	}
	dir, ok := dirNode.(*mfs.Directory)
	if !ok {
		http.Error(w, "WritableGateway: target directory is not a directory", http.StatusBadRequest)
		return
	}
	err = dir.Unlink(newFileName)
	switch err {
	case os.ErrNotExist, nil:
	default:
		webError(w, "WritableGateway: failed to replace existing file", err, http.StatusBadRequest)
		return
	}
	err = dir.AddChild(newFileName, newFile)
	if err != nil {
		webError(w, "WritableGateway: failed to link file into directory", err, http.StatusInternalServerError)
		return
	}
	nnode, err := root.GetDirectory().GetNode()
	if err != nil {
		webError(w, "WritableGateway: failed to finalize", err, http.StatusInternalServerError)
		return
	}
	newcid := nnode.Cid()

	i.addUserHeaders(w) // ok, _now_ write user's headers.
	w.Header().Set("IPFS-Hash", newcid.String())
	http.Redirect(w, r, gopath.Join(ipfsPathPrefix, newcid.String(), newPath), http.StatusCreated)
}

func (i *gatewayHandler) deleteHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// parse the path

	rootCid, newPath, err := parseIpfsPath(r.URL.Path)
	if err != nil {
		webError(w, "WritableGateway: failed to parse the path", err, http.StatusBadRequest)
		return
	}
	if newPath == "" || newPath == "/" {
		http.Error(w, "WritableGateway: empty path", http.StatusBadRequest)
		return
	}
	directory, filename := gopath.Split(newPath)

	// lookup the root

	rootNodeIPLD, err := i.api.Dag().Get(ctx, rootCid)
	if err != nil {
		webError(w, "WritableGateway: failed to resolve root CID", err, http.StatusInternalServerError)
		return
	}
	rootNode, ok := rootNodeIPLD.(*dag.ProtoNode)
	if !ok {
		http.Error(w, "WritableGateway: empty path", http.StatusInternalServerError)
		return
	}

	// construct the mfs root

	root, err := mfs.NewRoot(ctx, i.api.Dag(), rootNode, nil)
	if err != nil {
		webError(w, "WritableGateway: failed to construct the MFS root", err, http.StatusBadRequest)
		return
	}

	// lookup the parent directory

	parentNode, err := mfs.Lookup(root, directory)
	if err != nil {
		webError(w, "WritableGateway: failed to look up parent", err, http.StatusInternalServerError)
		return
	}

	parent, ok := parentNode.(*mfs.Directory)
	if !ok {
		http.Error(w, "WritableGateway: parent is not a directory", http.StatusInternalServerError)
		return
	}

	// delete the file

	switch parent.Unlink(filename) {
	case nil, os.ErrNotExist:
	default:
		webError(w, "WritableGateway: failed to remove file", err, http.StatusInternalServerError)
		return
	}

	nnode, err := root.GetDirectory().GetNode()
	if err != nil {
		webError(w, "WritableGateway: failed to finalize", err, http.StatusInternalServerError)
	}
	ncid := nnode.Cid()

	i.addUserHeaders(w) // ok, _now_ write user's headers.
	w.Header().Set("IPFS-Hash", ncid.String())
	// note: StatusCreated is technically correct here as we created a new resource.
	http.Redirect(w, r, gopath.Join(ipfsPathPrefix+ncid.String(), directory), http.StatusCreated)
}

func (i *gatewayHandler) addUserHeaders(w http.ResponseWriter) {
	for k, v := range i.config.Headers {
		w.Header()[k] = v
	}
}

func webError(w http.ResponseWriter, message string, err error, defaultCode int) {
	if _, ok := err.(resolver.ErrNoLink); ok {
		webErrorWithCode(w, message, err, http.StatusNotFound)
	} else if err == routing.ErrNotFound {
		webErrorWithCode(w, message, err, http.StatusNotFound)
	} else if err == context.DeadlineExceeded {
		webErrorWithCode(w, message, err, http.StatusRequestTimeout)
	} else {
		webErrorWithCode(w, message, err, defaultCode)
	}
}

func webErrorWithCode(w http.ResponseWriter, message string, err error, code int) {
	http.Error(w, fmt.Sprintf("%s: %s", message, err), code)
	if code >= 500 {
		log.Warnf("server error: %s: %s", err)
	}
}

// return a 500 error and log
func internalWebError(w http.ResponseWriter, err error) {
	webErrorWithCode(w, "internalWebError", err, http.StatusInternalServerError)
}

func getFilename(s string) string {
	if (strings.HasPrefix(s, ipfsPathPrefix) || strings.HasPrefix(s, ipnsPathPrefix)) && strings.Count(gopath.Clean(s), "/") <= 2 {
		// Don't want to treat ipfs.io in /ipns/ipfs.io as a filename.
		return ""
	}
	return gopath.Base(s)
}
