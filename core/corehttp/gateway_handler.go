package corehttp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	gopath "path"
	"runtime/debug"
	"strings"
	"time"

	core "github.com/ipfs/go-ipfs/core"
	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	"github.com/ipfs/go-ipfs/importer"
	chunk "github.com/ipfs/go-ipfs/importer/chunk"
	dag "github.com/ipfs/go-ipfs/merkledag"
	"github.com/ipfs/go-ipfs/namesys"
	path "github.com/ipfs/go-ipfs/path"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	recpb "gx/ipfs/QmdM4ohF7cr4MvAECVeD3hRA3HtZrk1ngaek4n8ojVT87h/go-libp2p-record/pb"

	namepb "github.com/ipfs/go-ipfs/namesys/pb"
	"github.com/ipfs/go-ipfs/thirdparty/ds-help"
	humanize "gx/ipfs/QmPSBJL4momYnE7DcUyk2DVhD6rH488ZmHBGLbxNdhU44K/go-humanize"
	node "gx/ipfs/QmRSU5EqqWVZSNdbU51yXmVoF1uNw3JgTNB6RaiL7DZM16/go-ipld-node"
	routing "gx/ipfs/QmbkGVaN9W6RYJK4Ws5FvMKXKDqdRQ5snhtaa92qP6L8eU/go-libp2p-routing"
	"gx/ipfs/QmcTcsTvfaeEBRFo1TkFgT8sRmgi1n1LTZpecfVP8fzpGD/go-cid"
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
	api    coreiface.UnixfsAPI
}

func newGatewayHandler(n *core.IpfsNode, c GatewayConfig, api coreiface.UnixfsAPI) *gatewayHandler {
	i := &gatewayHandler{
		node:   n,
		config: c,
		api:    api,
	}
	return i
}

// TODO(cryptix):  find these helpers somewhere else
func (i *gatewayHandler) newDagFromReader(r io.Reader) (node.Node, error) {
	// TODO(cryptix): change and remove this helper once PR1136 is merged
	// return ufs.AddFromReader(i.node, r.Body)
	return importer.BuildDagFromReader(
		i.node.DAG,
		chunk.DefaultSplitter(r))
}

// TODO(btc): break this apart into separate handlers using a more expressive muxer
func (i *gatewayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	defer func() {
		if r := recover(); r != nil {
			log.Error("A panic occurred in the gateway handler!")
			log.Error(r)
			debug.PrintStack()
		}
	}()

	if i.config.Authenticated {
		if i.config.Username == "" || i.config.Password == "" {
			cookie, err := r.Cookie("OpenBazaar_Auth_Cookie")
			if err != nil {
				w.WriteHeader(http.StatusForbidden)
				fmt.Fprint(w, "403 - Forbidden")
				return
			}
			if i.config.Cookie.Value != cookie.Value {
				w.WriteHeader(http.StatusForbidden)
				fmt.Fprint(w, "403 - Forbidden")
				return
			}
		} else {
			username, password, ok := r.BasicAuth()
			if !ok || username != i.config.Username || password != i.config.Password {
				w.WriteHeader(http.StatusForbidden)
				fmt.Fprint(w, "403 - Forbidden")
				return
			}
		}
	}

	if i.config.Writable {
		switch r.Method {
		case "POST":
			i.postHandler(ctx, w, r)
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
		i.getOrHeadHandler(ctx, w, r)
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

func (i *gatewayHandler) getOrHeadHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {

	// If this is an ipns query and the user passed in a blockchain ID handle, let's resolve it into a peer ID.
	var paths []string = strings.Split(r.URL.Path, "/")
	if paths[1] == "ipns" && paths[2][0:1] == "@" {
		peerID, err := i.config.Resolver.Resolve(paths[2])
		if err != nil {
			webError(w, "Path Resolve error", err, http.StatusBadRequest)
			return
		}
		r.URL.Path = strings.Replace(r.URL.Path, paths[2], peerID, 1)
	}

	unmodifiedURLPath := r.URL.Path

	// If this is an ipns query let's check to see if it's using our own peer ID.
	// If so let's resolve it locally instead of going out to the network.
	var ownID bool = false
	if paths[1] == "ipns" && paths[2] == i.node.Identity.Pretty() {
		id := i.node.Identity
		_, ipnskey := namesys.IpnsKeysForID(id)
		ival, hasherr := i.node.Repo.Datastore().Get(dshelp.NewKeyFromBinary([]byte(ipnskey)))
		if hasherr != nil {
			webError(w, "Error fetching own IPNS mapping", hasherr, http.StatusInternalServerError)
			return
		}
		val := ival.([]byte)
		dhtrec := new(recpb.Record)
		proto.Unmarshal(val, dhtrec)
		e := new(namepb.IpnsEntry)
		proto.Unmarshal(dhtrec.GetValue(), e)
		pth := path.Path(e.Value).String()
		for _, p := range paths[3:] {
			pth += "/" + p
		}
		r.URL.Path = pth
		ownID = true
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
	fmt.Println(urlPath)
	dr, err := i.api.Cat(ctx, urlPath)
	dir := false
	switch err {
	case nil:
		// core.Resolve worked
		defer dr.Close()
	case coreiface.ErrIsDir:
		dir = true
	case namesys.ErrResolveFailed:
		// Don't log that error as it is just noise
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Path Resolve error: %s", err.Error())
		log.Info("Path Resolve error: %s", err.Error())
		return
	case coreiface.ErrOffline:
		if !i.node.OnlineMode() {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, "Could not resolve path. Node is in offline mode.")
			return
		}
		fallthrough
	default:
		// all other erros
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

	// set these headers _after_ the error, for we may just not have it
	// and dont want the client to cache a 500 response...
	// and only if it's /ipfs!
	// TODO: break this out when we split /ipfs /ipns routes.
	modtime := time.Now()
	if strings.HasPrefix(unmodifiedURLPath, ipfsPathPrefix) {
		w.Header().Set("Etag", etag)
		w.Header().Set("Cache-Control", "public, max-age=29030400, immutable")
		// set modtime to a really long time ago, since files are immutable and should stay cached
		modtime = time.Unix(1, 0)
	} else if strings.HasPrefix(unmodifiedURLPath, ipnsPathPrefix) && !ownID { // cache ipns returns for 10 minutes
		w.Header().Set("Cache-Control", "public, max-age=600, immutable")
	}

	if !dir {
		name := gopath.Base(urlPath)
		http.ServeContent(w, r, name, modtime, dr)
		return
	}

	links, err := i.api.Ls(ctx, urlPath)
	if err != nil {
		internalWebError(w, err)
		return
	}

	// storage for directory listing
	var dirListing []directoryItem
	// loop through files
	foundIndex := false
	for _, link := range links {
		if link.Name == "index.html" {
			log.Debugf("found index.html link for %s", urlPath)
			foundIndex = true

			if urlPath[len(urlPath)-1] != '/' {
				// See comment above where originalUrlPath is declared.
				http.Redirect(w, r, originalUrlPath+"/", 302)
				log.Debugf("redirect to %s", originalUrlPath+"/")
				return
			}

			p, err := path.ParsePath(urlPath + "/index.html")
			if err != nil {
				internalWebError(w, err)
				return
			}

			// return index page instead.
			dr, err := i.api.Cat(ctx, p.String())
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

func (i *gatewayHandler) postHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	k, err := i.api.Add(ctx, r.Body)
	if err != nil {
		internalWebError(w, err)
		return
	}

	i.addUserHeaders(w) // ok, _now_ write user's headers.
	w.Header().Set("IPFS-Hash", k.String())
	http.Redirect(w, r, ipfsPathPrefix+k.String(), http.StatusCreated)
}

func (i *gatewayHandler) putHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(i.node.Context(), time.Second*300)

	var paths []string = strings.Split(r.URL.Path, "/")
	if paths[1] != "ipfs" {
		webError(w, "Cannot put to IPNS", errors.New("Cannot put to IPNS"), http.StatusInternalServerError)
		cancel()
		return
	}
	if len(paths) != 3 {
		webError(w, "Path must contain only one hash", errors.New("Path must contain only one hash"), http.StatusInternalServerError)
		cancel()
		return
	}

	go func() {
		k, err := cid.Decode(paths[2])
		if err != nil {
			return
		}
		dag.FetchGraph(ctx, k, i.node.DAG)
	}()

	i.addUserHeaders(w)
	return
}

func (i *gatewayHandler) deleteHandler(w http.ResponseWriter, r *http.Request) {
	urlPath := r.URL.Path
	ctx, cancel := context.WithCancel(i.node.Context())
	defer cancel()

	p, err := path.ParsePath(urlPath)
	if err != nil {
		webError(w, "failed to parse path", err, http.StatusBadRequest)
		return
	}

	c, components, err := path.SplitAbsPath(p)
	if err != nil {
		webError(w, "Could not split path", err, http.StatusInternalServerError)
		return
	}

	tctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	rootnd, err := i.node.Resolver.DAG.Get(tctx, c)
	if err != nil {
		webError(w, "Could not resolve root object", err, http.StatusBadRequest)
		return
	}

	pathNodes, err := i.node.Resolver.ResolveLinks(tctx, rootnd, components[:len(components)-1])
	if err != nil {
		webError(w, "Could not resolve parent object", err, http.StatusBadRequest)
		return
	}

	pbnd, ok := pathNodes[len(pathNodes)-1].(*dag.ProtoNode)
	if !ok {
		webError(w, "Cannot read non protobuf nodes through gateway", dag.ErrNotProtobuf, http.StatusBadRequest)
		return
	}

	// TODO(cyrptix): assumes len(pathNodes) > 1 - not found is an error above?
	err = pbnd.RemoveNodeLink(components[len(components)-1])
	if err != nil {
		webError(w, "Could not delete link", err, http.StatusBadRequest)
		return
	}

	var newnode *dag.ProtoNode = pbnd
	for j := len(pathNodes) - 2; j >= 0; j-- {
		if _, err := i.node.DAG.Add(newnode); err != nil {
			webError(w, "Could not add node", err, http.StatusInternalServerError)
			return
		}

		pathpb, ok := pathNodes[j].(*dag.ProtoNode)
		if !ok {
			webError(w, "Cannot read non protobuf nodes through gateway", dag.ErrNotProtobuf, http.StatusBadRequest)
			return
		}

		newnode, err = pathpb.UpdateNodeLink(components[j], newnode)
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
	ncid := newnode.Cid()

	i.addUserHeaders(w) // ok, _now_ write user's headers.
	w.Header().Set("IPFS-Hash", ncid.String())
	http.Redirect(w, r, gopath.Join(ipfsPathPrefix+ncid.String(), path.Join(components[:len(components)-1])), http.StatusCreated)
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

	log.Errorf("%s: %s", message, err) // TODO(cryptix): log until we have a better way to expose these (counter metrics maybe)
	fmt.Fprintf(w, "%s: %s", message, err)
}

// return a 500 error and log
func internalWebError(w http.ResponseWriter, err error) {
	webErrorWithCode(w, "internalWebError", err, http.StatusInternalServerError)
}
