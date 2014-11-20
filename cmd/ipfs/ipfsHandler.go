package main

import (
	"html/template"
	"io"
	"net/http"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"

	core "github.com/jbenet/go-ipfs/core"
	"github.com/jbenet/go-ipfs/importer"
	dag "github.com/jbenet/go-ipfs/merkledag"
	"github.com/jbenet/go-ipfs/routing"
	uio "github.com/jbenet/go-ipfs/unixfs/io"
	u "github.com/jbenet/go-ipfs/util"
)

type ipfs interface {
	ResolvePath(string) (*dag.Node, error)
	NewDagFromReader(io.Reader) (*dag.Node, error)
	AddNodeToDAG(nd *dag.Node) (u.Key, error)
	NewDagReader(nd *dag.Node) (io.Reader, error)
}

// shortcut for templating
type H map[string]interface{}

// struct for directory listing
type directoryItem struct {
	Size uint64
	Name string
}

// ipfsHandler is a HTTP handler that serves IPFS objects (accessible by default at /ipfs/<path>)
// (it serves requests like GET /ipfs/QmVRzPKPzNtSrEzBFm2UZfxmPAgnaLke4DMcerbsGGSaFe/link)
type ipfsHandler struct {
	node    *core.IpfsNode
	dirList *template.Template
}

// Load the directroy list template
func (i *ipfsHandler) LoadTemplate() {
	t, err := template.New("dir").Parse(listingTemplate)
	if err != nil {
		log.Error(err)
	}
	i.dirList = t
}

func (i *ipfsHandler) ResolvePath(path string) (*dag.Node, error) {
	return i.node.Resolver.ResolvePath(path)
}

func (i *ipfsHandler) NewDagFromReader(r io.Reader) (*dag.Node, error) {
	return importer.NewDagFromReader(r)
}

func (i *ipfsHandler) AddNodeToDAG(nd *dag.Node) (u.Key, error) {
	return i.node.DAG.Add(nd)
}

func (i *ipfsHandler) NewDagReader(nd *dag.Node) (io.Reader, error) {
	return uio.NewDagReader(nd, i.node.DAG)
}

func (i *ipfsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[5:]

	nd, err := i.ResolvePath(path)
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

	if err != nil {
		if err == uio.ErrIsDir {
			log.Debug("is directory %s", path)

			if path[len(path)-1:] != "/" {
				log.Debug("missing trailing slash redirect")
				http.Redirect(w, r, "/ipfs/"+path+"/", 307)
				return
			}

			// storage for directory listing
			var dirListing []directoryItem
			// loop through files
			for _, link := range nd.Links {
				if link.Name == "index.html" {
					log.Debug("found index")
					// return index page
					nd, err := i.ResolvePath(path + "/index.html")
					if err != nil {
						w.WriteHeader(http.StatusInternalServerError)
						w.Write([]byte(err.Error()))
						log.Error("%s", err)
						return
					}
					dr, err := i.NewDagReader(nd)
					if err != nil {
						w.WriteHeader(http.StatusInternalServerError)
						w.Write([]byte(err.Error()))
						log.Error("%s", err)
						return
					}
					// write to request
					io.Copy(w, dr)
					return
				}
				dirListing = append(dirListing, directoryItem{link.Size, link.Name})
			}
			// template and return directory listing
			//for i, j := range dirListing {
			//	log.Debug(i, ":", j.Size, " ", j.Name)
			//}
			err := i.dirList.Execute(w, H{"listing": dirListing, "path": path})
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				log.Error("%s", err)
				return
			}
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		log.Error(err)
		w.Write([]byte(err.Error()))
		return
	}
	// data file
	io.Copy(w, dr)
}

func (i *ipfsHandler) postHandler(w http.ResponseWriter, r *http.Request) {
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
	<li> <a href="./..">Parent</a></li>	
    {{ range $item := .listing }}
	 <li> <a href="./{{ $item.Name }}">{{ $item.Name }}</a> - {{ $item.Size }} bytes</li>	
	{{ end }}
	</ul>
	</body>
</html>
`
