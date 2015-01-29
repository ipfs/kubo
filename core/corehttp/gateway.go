package corehttp

import (
	"net/http"

	core "github.com/jbenet/go-ipfs/core"
)

func GatewayOption(n *core.IpfsNode, mux *http.ServeMux) error {
	gateway, err := newGatewayHandler(n)
	if err != nil {
		return err
	}
	mux.Handle("/ipfs/", gateway)
	mux.Handle("/ipns/", gateway)
	return nil
}
