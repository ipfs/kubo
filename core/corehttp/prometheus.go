package corehttp

import (
	"net"
	"net/http"

	prom "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/prometheus/client_golang/prometheus"

	"github.com/ipfs/go-ipfs/core"
)

func PrometheusOption(path string) ServeOption {
	return func(n *core.IpfsNode, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		mux.Handle(path, prom.Handler())
		return mux, nil
	}
}
