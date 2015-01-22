package corehttp

import (
	"net/http"

	core "github.com/jbenet/go-ipfs/core"
)

const (
	// TODO rename
	webuiPath = "/ipfs/QmTWvqK9dYvqjAMAcCeUun8b45Fwu7wPhEN9B9TsGbkXfJ"
)

func WebUIOption(n *core.IpfsNode, mux *http.ServeMux) error {
	mux.Handle("/webui/", &redirectHandler{webuiPath})
	return nil
}

type redirectHandler struct {
	path string
}

func (i *redirectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, i.path, 302)
}
