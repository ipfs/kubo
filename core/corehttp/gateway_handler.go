package corehttp

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	gopath "path"
	"runtime/debug"
	"strings"
	"time"

	humanize "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/dustin/go-humanize"
	"gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"

	key "github.com/ipfs/go-ipfs/blocks/key"
	core "github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/importer"
	chunk "github.com/ipfs/go-ipfs/importer/chunk"
	dag "github.com/ipfs/go-ipfs/merkledag"
	dagutils "github.com/ipfs/go-ipfs/merkledag/utils"
	path "github.com/ipfs/go-ipfs/path"
	"github.com/ipfs/go-ipfs/routing"
	uio "github.com/ipfs/go-ipfs/unixfs/io"
)

const (
	ipfsPathPrefix = "/ipfs/"
	ipnsPathPrefix = "/ipns/"
)

// gatewayHandler is a HTTP handler that serves IPFS objects (accessible by default at /ipfs/<path>)
// (it serves requests like GET /ipfs/QmVRzPKPzNtSrEzBFm2UZfxmPAgnaLke4DMcerbsGGSaFe/link)
type gatewayHandler struct {
	node   *core.IpfsNode
	config GatewayConfig
}

func newGatewayHandler(node *core.IpfsNode, conf GatewayConfig) (*gatewayHandler, error) {
	i := &gatewayHandler{
		node:   node,
		config: conf,
	}
	return i, nil
}

// TODO(cryptix):  find these helpers somewhere else
func (i *gatewayHandler) newDagFromReader(r io.Reader) (*dag.Node, error) {
	// TODO(cryptix): change and remove this helper once PR1136 is merged
	// return ufs.AddFromReader(i.node, r.Body)
	return importer.BuildDagFromReader(
		i.node.DAG,
		chunk.DefaultSplitter(r))
}

// TODO(btc): break this apart into separate handlers using a more expressive muxer
func (i *gatewayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			log.Error("A panic occurred in the gateway handler!")
			log.Error(r)
			debug.PrintStack()
		}
	}()

	if i.config.Writable {
		switch r.Method {
		case "POST":
			i.postHandler(w, r)
			return
		case "PUT":
			i.putHandler(w, r)
			return
		case "DELETE":
			i.deleteHandler(w, r)
			return
		}
	}

	if r.Method == "GET" || r.Method == "HEAD" {
		i.getOrHeadHandler(w, r)
		return
	}

	if r.Method == "OPTIONS" {
		i.optionsHandler(w, r)
		return
	}

	errmsg := "Method " + r.Method + " not allowed: "
	if !i.config.Writable {
		w.WriteHeader(http.StatusMethodNotAllowed)
		errmsg = errmsg + "read only access"
	} else {
		w.WriteHeader(http.StatusBadRequest)
		errmsg = errmsg + "bad request for " + r.URL.Path
	}
	fmt.Fprint(w, errmsg)
	log.Error(errmsg) // TODO(cryptix): log errors until we have a better way to expose these (counter metrics maybe)
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
	ctx, cancel := context.WithTimeout(i.node.Context(), time.Hour)
	// the hour is a hard fallback, we don't expect it to happen, but just in case
	defer cancel()

	if cn, ok := w.(http.CloseNotifier); ok {
		clientGone := cn.CloseNotify()
		go func() {
			select {
			case <-clientGone:
			case <-ctx.Done():
			}
			cancel()
		}()
	}

	urlPath := r.URL.Path

	// If the gateway is behind a reverse proxy and mounted at a sub-path,
	// the prefix header can be set to signal this sub-path.
	// It will be prepended to links in directory listings and the index.html redirect.
	prefix := ""
	if prefixHdr := r.Header["X-Ipfs-Gateway-Prefix"]; len(prefixHdr) > 0 {
		prfx := prefixHdr[0]
		for _, p := range i.config.PathPrefixes {
			if prfx == p || strings.HasPrefix(prfx, p+"/") {
				prefix = prfx
				break
			}
		}
	}

	// IPNSHostnameOption might have constructed an IPNS path using the Host header.
	// In this case, we need the original path for constructing redirects
	// and links that match the requested URL.
	// For example, http://example.net would become /ipns/example.net, and
	// the redirects and links would end up as http://example.net/ipns/example.net
	originalUrlPath := prefix + urlPath
	ipnsHostname := false
	if hdr := r.Header["X-Ipns-Original-Path"]; len(hdr) > 0 {
		originalUrlPath = prefix + hdr[0]
		ipnsHostname = true
	}

	if i.config.BlockList != nil && i.config.BlockList.ShouldBlock(urlPath) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("403 - Forbidden"))
		return
	}

	nd, err := core.Resolve(ctx, i.node, path.Path(urlPath))
	// If node is in offline mode the error code and message should be different
	if err == core.ErrNoNamesys && !i.node.OnlineMode() {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, "Could not resolve path. Node is in offline mode.")
		return
	} else if err != nil {
		webError(w, "Path Resolve error", err, http.StatusBadRequest)
		return
	}

	etag := gopath.Base(urlPath)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	i.addUserHeaders(w) // ok, _now_ write user's headers.
	w.Header().Set("X-IPFS-Path", urlPath)

	// set 'allowed' headers
	w.Header().Set("Access-Control-Allow-Headers", "X-Stream-Output, X-Chunked-Output")
	// expose those headers
	w.Header().Set("Access-Control-Expose-Headers", "X-Stream-Output, X-Chunked-Output")

	// Suborigin header, sandboxes apps from each other in the browser (even
	// though they are served from the same gateway domain).
	//
	// Omited if the path was treated by IPNSHostnameOption(), for example
	// a request for http://example.net/ would be changed to /ipns/example.net/,
	// which would turn into an incorrect Suborigin: example.net header.
	//
	// NOTE: This is not yet widely supported by browsers.
	if !ipnsHostname {
		pathRoot := strings.SplitN(urlPath, "/", 4)[2]
		w.Header().Set("Suborigin", pathRoot)
	}

	dr, err := uio.NewDagReader(ctx, nd, i.node.DAG)
	if err != nil && err != uio.ErrIsDir {
		// not a directory and still an error
		internalWebError(w, err)
		return
	}

	// set these headers _after_ the error, for we may just not have it
	// and dont want the client to cache a 500 response...
	// and only if it's /ipfs!
	// TODO: break this out when we split /ipfs /ipns routes.
	modtime := time.Now()
	if strings.HasPrefix(urlPath, ipfsPathPrefix) {
		w.Header().Set("Etag", etag)
		w.Header().Set("Cache-Control", "public, max-age=29030400, immutable")

		// set modtime to a really long time ago, since files are immutable and should stay cached
		modtime = time.Unix(1, 0)
	}

	if err == nil {
		defer dr.Close()
		name := gopath.Base(urlPath)
		http.ServeContent(w, r, name, modtime, dr)
		return
	}

	// storage for directory listing
	var dirListing []directoryItem
	// loop through files
	foundIndex := false
	for _, link := range nd.Links {
		if link.Name == "index.html" {
			log.Debugf("found index.html link for %s", urlPath)
			foundIndex = true

			if urlPath[len(urlPath)-1] != '/' {
				// See comment above where originalUrlPath is declared.
				http.Redirect(w, r, originalUrlPath+"/", 302)
				log.Debugf("redirect to %s", originalUrlPath+"/")
				return
			}

			// return index page instead.
			nd, err := core.Resolve(ctx, i.node, path.Path(urlPath+"/index.html"))
			if err != nil {
				internalWebError(w, err)
				return
			}
			dr, err := uio.NewDagReader(ctx, nd, i.node.DAG)
			if err != nil {
				internalWebError(w, err)
				return
			}
			defer dr.Close()

			// write to request
			http.ServeContent(w, r, "index.html", modtime, dr)
			break
		}

		// See comment above where originalUrlPath is declared.
		di := directoryItem{humanize.Bytes(link.Size), link.Name, gopath.Join(originalUrlPath, link.Name)}
		dirListing = append(dirListing, di)
	}

	if !foundIndex {
		if r.Method != "HEAD" {
			// construct the correct back link
			// https://github.com/ipfs/go-ipfs/issues/1365
			var backLink string = prefix + urlPath

			// don't go further up than /ipfs/$hash/
			pathSplit := path.SplitList(backLink)
			switch {
			// keep backlink
			case len(pathSplit) == 3: // url: /ipfs/$hash

			// keep backlink
			case len(pathSplit) == 4 && pathSplit[3] == "": // url: /ipfs/$hash/

			// add the correct link depending on wether the path ends with a slash
			default:
				if strings.HasSuffix(backLink, "/") {
					backLink += "./.."
				} else {
					backLink += "/.."
				}
			}

			// strip /ipfs/$hash from backlink if IPNSHostnameOption touched the path.
			if ipnsHostname {
				backLink = prefix + "/"
				if len(pathSplit) > 5 {
					// also strip the trailing segment, because it's a backlink
					backLinkParts := pathSplit[3 : len(pathSplit)-2]
					backLink += path.Join(backLinkParts) + "/"
				}
			}

			// See comment above where originalUrlPath is declared.
			tplData := listingTemplateData{
				Listing:  dirListing,
				Path:     originalUrlPath,
				BackLink: backLink,
			}
			err := listingTemplate.Execute(w, tplData)
			if err != nil {
				internalWebError(w, err)
				return
			}
		}
	}
}

func (i *gatewayHandler) postHandler(w http.ResponseWriter, r *http.Request) {
	nd, err := i.newDagFromReader(r.Body)
	if err != nil {
		internalWebError(w, err)
		return
	}

	k, err := i.node.DAG.Add(nd)
	if err != nil {
		internalWebError(w, err)
		return
	}

	i.addUserHeaders(w) // ok, _now_ write user's headers.
	w.Header().Set("IPFS-Hash", k.String())
	http.Redirect(w, r, ipfsPathPrefix+k.String(), http.StatusCreated)
}

func (i *gatewayHandler) putHandler(w http.ResponseWriter, r *http.Request) {
	// TODO(cryptix): move me to ServeHTTP and pass into all handlers
	ctx, cancel := context.WithCancel(i.node.Context())
	defer cancel()

	rootPath, err := path.ParsePath(r.URL.Path)
	if err != nil {
		webError(w, "putHandler: ipfs path not valid", err, http.StatusBadRequest)
		return
	}

	rsegs := rootPath.Segments()
	if rsegs[0] == ipnsPathPrefix {
		webError(w, "putHandler: updating named entries not supported", errors.New("WritableGateway: ipns put not supported"), http.StatusBadRequest)
		return
	}

	var newnode *dag.Node
	if rsegs[len(rsegs)-1] == "QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn" {
		newnode = uio.NewEmptyDirectory()
	} else {
		putNode, err := i.newDagFromReader(r.Body)
		if err != nil {
			webError(w, "putHandler: Could not create DAG from request", err, http.StatusInternalServerError)
			return
		}
		newnode = putNode
	}

	var newPath string
	if len(rsegs) > 1 {
		newPath = path.Join(rsegs[2:])
	}

	var newkey key.Key
	rnode, err := core.Resolve(ctx, i.node, rootPath)
	switch ev := err.(type) {
	case path.ErrNoLink:
		// ev.Node < node where resolve failed
		// ev.Name < new link
		// but we need to patch from the root
		rnode, err := i.node.DAG.Get(ctx, key.B58KeyDecode(rsegs[1]))
		if err != nil {
			webError(w, "putHandler: Could not create DAG from request", err, http.StatusInternalServerError)
			return
		}

		e := dagutils.NewDagEditor(rnode, i.node.DAG)
		err = e.InsertNodeAtPath(ctx, newPath, newnode, uio.NewEmptyDirectory)
		if err != nil {
			webError(w, "putHandler: InsertNodeAtPath failed", err, http.StatusInternalServerError)
			return
		}

		nnode, err := e.Finalize(i.node.DAG)
		if err != nil {
			webError(w, "putHandler: could not get node", err, http.StatusInternalServerError)
			return
		}

		newkey, err = nnode.Key()
		if err != nil {
			webError(w, "putHandler: could not get key of edited node", err, http.StatusInternalServerError)
			return
		}

	case nil:
		// object set-data case
		rnode.Data = newnode.Data

		newkey, err = i.node.DAG.Add(rnode)
		if err != nil {
			nnk, _ := newnode.Key()
			rk, _ := rnode.Key()
			webError(w, fmt.Sprintf("putHandler: Could not add newnode(%q) to root(%q)", nnk.B58String(), rk.B58String()), err, http.StatusInternalServerError)
			return
		}
	default:
		log.Warningf("putHandler: unhandled resolve error %T", ev)
		webError(w, "could not resolve root DAG", ev, http.StatusInternalServerError)
		return
	}

	i.addUserHeaders(w) // ok, _now_ write user's headers.
	w.Header().Set("IPFS-Hash", newkey.String())
	http.Redirect(w, r, gopath.Join(ipfsPathPrefix, newkey.String(), newPath), http.StatusCreated)
}

func (i *gatewayHandler) deleteHandler(w http.ResponseWriter, r *http.Request) {
	urlPath := r.URL.Path
	ctx, cancel := context.WithCancel(i.node.Context())
	defer cancel()

	ipfsNode, err := core.Resolve(ctx, i.node, path.Path(urlPath))
	if err != nil {
		// FIXME HTTP error code
		webError(w, "Could not resolve name", err, http.StatusInternalServerError)
		return
	}

	k, err := ipfsNode.Key()
	if err != nil {
		webError(w, "Could not get key from resolved node", err, http.StatusInternalServerError)
		return
	}

	h, components, err := path.SplitAbsPath(path.FromKey(k))
	if err != nil {
		webError(w, "Could not split path", err, http.StatusInternalServerError)
		return
	}

	tctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	rootnd, err := i.node.Resolver.DAG.Get(tctx, key.Key(h))
	if err != nil {
		webError(w, "Could not resolve root object", err, http.StatusBadRequest)
		return
	}

	pathNodes, err := i.node.Resolver.ResolveLinks(tctx, rootnd, components[:len(components)-1])
	if err != nil {
		webError(w, "Could not resolve parent object", err, http.StatusBadRequest)
		return
	}

	// TODO(cyrptix): assumes len(pathNodes) > 1 - not found is an error above?
	err = pathNodes[len(pathNodes)-1].RemoveNodeLink(components[len(components)-1])
	if err != nil {
		webError(w, "Could not delete link", err, http.StatusBadRequest)
		return
	}

	newnode := pathNodes[len(pathNodes)-1]
	for j := len(pathNodes) - 2; j >= 0; j-- {
		if _, err := i.node.DAG.Add(newnode); err != nil {
			webError(w, "Could not add node", err, http.StatusInternalServerError)
			return
		}
		newnode, err = pathNodes[j].UpdateNodeLink(components[j], newnode)
		if err != nil {
			webError(w, "Could not update node links", err, http.StatusInternalServerError)
			return
		}
	}

	if _, err := i.node.DAG.Add(newnode); err != nil {
		webError(w, "Could not add root node", err, http.StatusInternalServerError)
		return
	}

	// Redirect to new path
	key, err := newnode.Key()
	if err != nil {
		webError(w, "Could not get key of new node", err, http.StatusInternalServerError)
		return
	}

	i.addUserHeaders(w) // ok, _now_ write user's headers.
	w.Header().Set("IPFS-Hash", key.String())
	http.Redirect(w, r, gopath.Join(ipfsPathPrefix+key.String(), path.Join(components[:len(components)-1])), http.StatusCreated)
}

func (i *gatewayHandler) addUserHeaders(w http.ResponseWriter) {
	for k, v := range i.config.Headers {
		w.Header()[k] = v
	}
}

func webError(w http.ResponseWriter, message string, err error, defaultCode int) {
	if _, ok := err.(path.ErrNoLink); ok {
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
	w.WriteHeader(code)
	log.Errorf("%s: %s", message, err) // TODO(cryptix): log errors until we have a better way to expose these (counter metrics maybe)
	fmt.Fprintf(w, "%s: %s", message, err)
}

// return a 500 error and log
func internalWebError(w http.ResponseWriter, err error) {
	webErrorWithCode(w, "internalWebError", err, http.StatusInternalServerError)
}
