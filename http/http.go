package http

import (
	"fmt"
	"github.com/gorilla/mux"
	core "github.com/jbenet/go-ipfs/core"
	"net/http"
	"strconv"
)

type ipfsHandler struct {
	node *core.IpfsNode
}

func (i *ipfsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	nd, err := i.node.Resolver.ResolvePath(path)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "%s", nd.Data)
}

func Serve(host string, port uint, node *core.IpfsNode) error {
	r := mux.NewRouter()
	r.PathPrefix("/").Handler(&ipfsHandler{node}).Methods("GET")
	http.Handle("/", r)

	address := host + ":" + strconv.FormatUint(uint64(port), 10)
	fmt.Printf("Serving on %s\n", address)

	return http.ListenAndServe(address, nil)
}
