// package multistream implements a peerstream transport using
// go-multistream to select the underlying stream muxer
package multistream

import (
	"net"

	mss "github.com/whyrusleeping/go-multistream"

	smux "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-stream-muxer"
	multiplex "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-stream-muxer/multiplex"
	spdy "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-stream-muxer/spdystream"
	yamux "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-stream-muxer/yamux"
)

type transport struct {
	mux *mss.MultistreamMuxer

	tpts map[string]smux.Transport
}

func NewTransport() smux.Transport {
	mux := mss.NewMultistreamMuxer()
	mux.AddHandler("/multiplex", nil)
	mux.AddHandler("/spdystream", nil)
	mux.AddHandler("/yamux", nil)

	tpts := map[string]smux.Transport{
		"/multiplex":  multiplex.DefaultTransport,
		"/spdystream": spdy.Transport,
		"/yamux":      yamux.DefaultTransport,
	}

	return &transport{
		mux:  mux,
		tpts: tpts,
	}
}

func (t *transport) NewConn(nc net.Conn, isServer bool) (smux.Conn, error) {
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
