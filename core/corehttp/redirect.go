package corehttp

import (
	"net"
	"net/http"

	core "github.com/ipfs/kubo/core"
)

func RedirectOption(path string, redirect string) ServeOption {
	return func(n *core.IpfsNode, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		cfg, err := n.Repo.Config()
		if err != nil {
			return nil, err
		}

		handler := &redirectHandler{redirect, cfg.API.HTTPHeaders}

		if len(path) > 0 {
			mux.Handle("/"+path+"/", handler)
		} else {
			mux.Handle("/", handler)
		}
		return mux, nil
	}
}

type redirectHandler struct {
	path    string
	headers map[string][]string
}

func (i *redirectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for k, v := range i.headers {
		w.Header()[http.CanonicalHeaderKey(k)] = v
	}

	http.Redirect(w, r, i.path, http.StatusFound)
}
