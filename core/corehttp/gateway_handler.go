package corehttp

import (
	"html/template"
	"io"
	"net/http"
	"path"
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

type gateway interface {
	ResolvePath(string) (*dag.Node, error)
	NewDagFromReader(io.Reader) (*dag.Node, error)
	AddNodeToDAG(nd *dag.Node) (u.Key, error)
	NewDagReader(nd *dag.Node) (io.ReadSeeker, error)
}

// shortcut for templating
type webHandler map[string]interface{}

// struct for directory listing
type directoryItem struct {
	Size uint64
	Name string
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

func (i *gatewayHandler) ResolvePath(p string) (*dag.Node, error) {
	return i.node.Resolver.ResolvePath(p)
}

func (i *gatewayHandler) NewDagFromReader(r io.Reader) (*dag.Node, error) {
	return importer.BuildDagFromReader(
		r, i.node.DAG, i.node.Pinning.GetManual(), chunk.DefaultSplitter)
}

func (i *gatewayHandler) AddNodeToDAG(nd *dag.Node) (u.Key, error) {
	return i.node.DAG.Add(nd)
}

func (i *gatewayHandler) NewDagReader(nd *dag.Node) (io.ReadSeeker, error) {
	return uio.NewDagReader(i.node.Context(), nd, i.node.DAG)
}

func (i *gatewayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	urlPath := r.URL.Path[5:]

	nd, err := i.ResolvePath(urlPath)
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

	dr, err := i.NewDagReader(nd)
	if err == nil {
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

	log.Debug("listing directory")
	if urlPath[len(urlPath)-1:] != "/" {
		log.Debug("missing trailing slash, redirect")
		http.Redirect(w, r, "/ipfs/"+urlPath+"/", 307)
		return
	}

	// storage for directory listing
	var dirListing []directoryItem
	// loop through files
	foundIndex := false
	for _, link := range nd.Links {
		if link.Name == "index.html" {
			log.Debug("found index")
			foundIndex = true
			// return index page instead.
			nd, err := i.ResolvePath(urlPath + "/index.html")
			if err != nil {
				internalWebError(w, err)
				return
			}
			dr, err := i.NewDagReader(nd)
			if err != nil {
				internalWebError(w, err)
				return
			}
			// write to request
			io.Copy(w, dr)
			break
		}

		dirListing = append(dirListing, directoryItem{link.Size, link.Name})
	}

	if !foundIndex {
		// template and return directory listing
		hndlr := webHandler{"listing": dirListing, "path": urlPath}
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
  {{ range $item := .listing }}
	<li><a href="./{{ $item.Name }}">{{ $item.Name }}</a> - {{ $item.Size }} bytes</li>
	{{ end }}
	</ul>
	</body>
</html>
`
