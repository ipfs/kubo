package muxtest

import (
	multiplex "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-stream-muxer/multiplex"
	multistream "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-stream-muxer/multistream"
	muxado "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-stream-muxer/muxado"
	spdy "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-stream-muxer/spdystream"
	yamux "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-stream-muxer/yamux"
)

var _ = multiplex.DefaultTransport
var _ = multistream.NewTransport
var _ = muxado.Transport
var _ = spdy.Transport
var _ = yamux.DefaultTransport
