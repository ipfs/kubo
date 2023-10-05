// Code copied from https://github.com/libp2p/go-libp2p/blob/9bd85029550a084fca63ec6ff9184122cdf06591/p2p/muxer/mplex/transport.go
package mplex

import (
	"net"

	"github.com/libp2p/go-libp2p/core/network"

	mp "github.com/libp2p/go-mplex"
)

// DefaultTransport has default settings for Transport
var DefaultTransport = &Transport{}

const ID = "/mplex/6.7.0"

var _ network.Multiplexer = &Transport{}

// Transport implements mux.Multiplexer that constructs
// mplex-backed muxed connections.
type Transport struct{}

func (t *Transport) NewConn(nc net.Conn, isServer bool, scope network.PeerScope) (network.MuxedConn, error) {
	m, err := mp.NewMultiplex(nc, isServer, scope)
	if err != nil {
		return nil, err
	}
	return NewMuxedConn(m), nil
}
