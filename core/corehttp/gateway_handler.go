package corehttp

import (
	"html/template"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"

	core "github.com/jbenet/go-ipfs/core"
	"github.com/jbenet/go-ipfs/importer"
	chunk "github.com/jbenet/go-ipfs/importer/chunk"
	dag "github.com/jbenet/go-ipfs/merkledag"
	"github.com/jbenet/go-ipfs/routing"
	uio "github.com/jbenet/go-ipfs/unixfs/io"
	u "github.com/jbenet/go-ipfs/util"
)

const (
	IpfsPathPrefix = "/ipfs/"
	IpnsPathPrefix = "/ipns/"
)

type gateway interface {
	ResolvePath(string) (*dag.Node, error)
	NewDagFromReader(io.Reader) (*dag.Node, error)
	AddNodeToDAG(nd *dag.Node) (u.Key, error)
	NewDagReader(nd *dag.Node) (uio.ReadSeekCloser, error)
}

// shortcut for templating
type webHandler map[string]interface{}

// struct for directory listing
type directoryItem struct {
	Size uint64
	Name string
	Path string
}

// gatewayHandler is a HTTP handler that serves IPFS objects (accessible by default at /ipfs/<path>)
// (it serves requests like GET /ipfs/QmVRzPKPzNtSrEzBFm2UZfxmPAgnaLke4DMcerbsGGSaFe/link)
type gatewayHandler struct {
	node    *core.IpfsNode
	dirList *template.Template
}

func newGatewayHandler(node *core.IpfsNode) (*gatewayHandler, error) {
	i := &gatewayHandler{
		node: node,
	}
	err := i.loadTemplate()
	if err != nil {
		return nil, err
	}
	return i, nil
}

// Load the directroy list template
func (i *gatewayHandler) loadTemplate() error {
	t, err := template.New("dir").Parse(listingTemplate)
	if err != nil {
		return err
	}
	i.dirList = t
	return nil
}

func (i *gatewayHandler) ResolvePath(ctx context.Context, p string) (*dag.Node, string, error) {
	p = path.Clean(p)

	if strings.HasPrefix(p, IpnsPathPrefix) {
		elements := strings.Split(p[len(IpnsPathPrefix):], "/")
		hash := elements[0]
		k, err := i.node.Namesys.Resolve(ctx, hash)
		if err != nil {
			return nil, "", err
		}

		elements[0] = k.Pretty()
		p = path.Join(elements...)
	}
	if !strings.HasPrefix(p, IpfsPathPrefix) {
		p = path.Join(IpfsPathPrefix, p)
	}

	node, err := i.node.Resolver.ResolvePath(p)
	if err != nil {
		return nil, "", err
	}
	return node, p, err
}

func (i *gatewayHandler) NewDagFromReader(r io.Reader) (*dag.Node, error) {
	return importer.BuildDagFromReader(
		r, i.node.DAG, i.node.Pinning.GetManual(), chunk.DefaultSplitter)
}

func (i *gatewayHandler) AddNodeToDAG(nd *dag.Node) (u.Key, error) {
	return i.node.DAG.Add(nd)
}

func (i *gatewayHandler) NewDagReader(nd *dag.Node) (uio.ReadSeekCloser, error) {
	return uio.NewDagReader(i.node.Context(), nd, i.node.DAG)
}

func (i *gatewayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(i.node.Context())
	defer cancel()

	urlPath := r.URL.Path

	nd, p, err := i.ResolvePath(ctx, urlPath)
	if err != nil {
		if err == routing.ErrNotFound {
			w.WriteHeader(http.StatusNotFound)
		} else if err == context.DeadlineExceeded {
			w.WriteHeader(http.StatusRequestTimeout)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}

		log.Error(err)
		w.Write([]byte(err.Error()))
		return
	}

	etag := path.Base(p)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("Etag", etag)
	w.Header().Set("X-IPFS-Path", p)
	w.Header().Set("Cache-Control", "public, max-age=29030400")

	dr, err := i.NewDagReader(nd)
	if err == nil {
		defer dr.Close()
		_, name := path.Split(urlPath)
		// set modtime to a really long time ago, since files are immutable and should stay cached
		modtime := time.Unix(1, 0)
		http.ServeContent(w, r, name, modtime, dr)
		return
	}

	if err != uio.ErrIsDir {
		// not a directory and still an error
		internalWebError(w, err)
		return
	}

	// storage for directory listing
	var dirListing []directoryItem
	// loop through files
	foundIndex := false
	for _, link := range nd.Links {
		if link.Name == "index.html" {
			if urlPath[len(urlPath)-1] != '/' {
				http.Redirect(w, r, urlPath+"/", 302)
				return
			}

			log.Debug("found index")
			foundIndex = true
			// return index page instead.
			nd, _, err := i.ResolvePath(ctx, urlPath+"/index.html")
			if err != nil {
				internalWebError(w, err)
				return
			}
			dr, err := i.NewDagReader(nd)
			if err != nil {
				internalWebError(w, err)
				return
			}
			defer dr.Close()
			// write to request
			io.Copy(w, dr)
			break
		}

		di := directoryItem{link.Size, link.Name, path.Join(urlPath, link.Name)}
		dirListing = append(dirListing, di)
	}

	if !foundIndex {
		// template and return directory listing
		hndlr := webHandler{
			"listing": dirListing,
			"path":    urlPath,
		}
		if err := i.dirList.Execute(w, hndlr); err != nil {
			internalWebError(w, err)
			return
		}
	}
}

func (i *gatewayHandler) postHandler(w http.ResponseWriter, r *http.Request) {
	nd, err := i.NewDagFromReader(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Error(err)
		w.Write([]byte(err.Error()))
		return
	}

	k, err := i.AddNodeToDAG(nd)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Error(err)
		w.Write([]byte(err.Error()))
		return
	}

	//TODO: return json representation of list instead
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(mh.Multihash(k).B58String()))
}

// return a 500 error and log
func internalWebError(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(err.Error()))
	log.Error("%s", err)
}

// Directory listing template
var listingTemplate = `
<!DOCTYPE html>
<html>
	<head>
		<meta charset="utf-8" />
		<title>{{ .path }}</title>
	</head>
	<body>
	<h2>Index of {{ .path }}</h2>
	<ul>
	<li><a href="./..">..</a></li>
  {{ range .listing }}
	<li><a href="{{ .Path }}">{{ .Name }}</a> - {{ .Size }} bytes</li>
	{{ end }}
	</ul>
	</body>
</html>
`
