// package multistream implements a peerstream transport using
// go-multistream to select the underlying stream muxer
package multistream

import (
	"net"

	smux "gx/ipfs/QmTYr6RrJs8b63LTVwahmtytnuqzsLfNPBJp6EvmFWHbGh/go-stream-muxer"
	multiplex "gx/ipfs/QmTYr6RrJs8b63LTVwahmtytnuqzsLfNPBJp6EvmFWHbGh/go-stream-muxer/multiplex"
	spdy "gx/ipfs/QmTYr6RrJs8b63LTVwahmtytnuqzsLfNPBJp6EvmFWHbGh/go-stream-muxer/spdystream"
	yamux "gx/ipfs/QmTYr6RrJs8b63LTVwahmtytnuqzsLfNPBJp6EvmFWHbGh/go-stream-muxer/yamux"
	mss "gx/ipfs/QmUeEcYJrzAEKdQXjzTxCgNZgc9sRuwharsvzzm5Gd2oGB/go-multistream"
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
