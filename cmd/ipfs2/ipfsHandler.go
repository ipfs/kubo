package main

import (
	"io"
	"net/http"

	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"

	core "github.com/jbenet/go-ipfs/core"
	"github.com/jbenet/go-ipfs/importer"
	dag "github.com/jbenet/go-ipfs/merkledag"
	uio "github.com/jbenet/go-ipfs/unixfs/io"
	u "github.com/jbenet/go-ipfs/util"
)

type ipfs interface {
	ResolvePath(string) (*dag.Node, error)
	NewDagFromReader(io.Reader) (*dag.Node, error)
	AddNodeToDAG(nd *dag.Node) (u.Key, error)
	NewDagReader(nd *dag.Node) (io.Reader, error)
}

type ipfsHandler struct {
	node *core.IpfsNode
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
		w.WriteHeader(http.StatusInternalServerError)
		log.Error(err)
		w.Write([]byte(err.Error()))
		return
	}

	dr, err := i.NewDagReader(nd)
	if err != nil {
		// TODO: return json object containing the tree data if it's a directory (err == ErrIsDir)
		w.WriteHeader(http.StatusInternalServerError)
		log.Error(err)
		w.Write([]byte(err.Error()))
		return
	}

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
