package http

import (
	"fmt"
	"io"
	"net/http"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gorilla/mux"
	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
	core "github.com/jbenet/go-ipfs/core"
)

type handler struct {
	ipfs
}

// Serve starts the http server
func Serve(address string, node *core.IpfsNode) error {
	r := mux.NewRouter()
	handler := &handler{&ipfsHandler{node}}
	r.HandleFunc("/", handler.postHandler).Methods("POST")
	r.PathPrefix("/").Handler(handler).Methods("GET")
	http.Handle("/", r)

	return http.ListenAndServe(address, nil)
}

func (i *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	nd, err := i.ResolvePath(path)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Println(err)
		return
	}

	dr, err := i.NewDagReader(nd)
	if err != nil {
		// TODO: return json object containing the tree data if it's a directory (err == ErrIsDir)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Println(err)
		return
	}

	io.Copy(w, dr)
}

func (i *handler) postHandler(w http.ResponseWriter, r *http.Request) {
	nd, err := i.NewDagFromReader(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Println(err)
		return
	}

	k, err := i.AddNodeToDAG(nd)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Println(err)
		return
	}

	//TODO: return json representation of list instead
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(mh.Multihash(k).B58String()))
}
