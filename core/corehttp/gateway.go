package corehttp

import (
	"net/http"

	core "github.com/jbenet/go-ipfs/core"
)

func GatewayOption(writable bool) ServeOption {
	return func(n *core.IpfsNode, mux *http.ServeMux) error {
		gateway, err := newGatewayHandler(n, writable)
		if err != nil {
			return err
		}
		mux.Handle("/ipfs/", gateway)
	mux.Handle("/ipns/", gateway)
		return nil
	}
}
