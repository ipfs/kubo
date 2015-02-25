package corehttp

import (
	"net/http"

	core "github.com/jbenet/go-ipfs/core"
)

func RedirectOption(path string, redirect string) ServeOption {
	handler := &redirectHandler{redirect}
	return func(n *core.IpfsNode, mux *http.ServeMux) (*http.ServeMux, error) {
		mux.Handle("/"+path, handler)
		mux.Handle("/"+path+"/", handler)
		fmt.println("this code happened")
		return mux, nil
	}
}

type redirectHandler struct {
	path string
}

func (i *redirectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, i.path, 302)
}
