// package multistream implements a peerstream transport using
// go-multistream to select the underlying stream muxer
package multistream

import (
	"net"

	pst "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-peerstream/transport"
	multiplex "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-peerstream/transport/multiplex"
	spdy "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-peerstream/transport/spdystream"
	yamux "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-peerstream/transport/yamux"
	mss "github.com/whyrusleeping/go-multistream"
)

type transport struct {
	mux *mss.MultistreamMuxer

	tpts map[string]pst.Transport
}

func NewTransport() pst.Transport {
	mux := mss.NewMultistreamMuxer()
	mux.AddHandler("/multiplex", nil)
	mux.AddHandler("/spdystream", nil)
	mux.AddHandler("/yamux", nil)

	tpts := map[string]pst.Transport{
		"/multiplex":  multiplex.DefaultTransport,
		"/spdystream": spdy.Transport,
		"/yamux":      yamux.DefaultTransport,
	}

	return &transport{
		mux:  mux,
		tpts: tpts,
	}
}

func (t *transport) NewConn(nc net.Conn, isServer bool) (pst.Conn, error) {
	var proto string
	if isServer {
		selected, _, err := t.mux.Negotiate(nc)
		if err != nil {
			return nil, err
		}
		proto = selected
	} else {
		// prefer yamux
		selected, err := mss.SelectOneOf([]string{"/yamux", "/spdystream", "/multiplex"}, nc)
		if err != nil {
			return nil, err
		}
		proto = selected
	}

	tpt := t.tpts[proto]

	return tpt.NewConn(nc, isServer)
}
